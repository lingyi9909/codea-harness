package reviewauthority

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"
)

type SelectionTurnRequest180 struct {
	SessionID    string
	MessageID    string
	RunID        string
	OptionsHash  string
	SelectionIDs []string
	AllIDs       []string
	MenuMarker   string
}

func VerifySelectionTurn180(ctx context.Context, root string, req SelectionTurnRequest180) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
	}
	launcher, err := exec.LookPath("opencode")
	if err != nil {
		return fmt.Errorf("HUMAN_SELECTION_REQUIRED: opencode unavailable: %w", err)
	}
	if runtime.GOOS == "windows" {
		launcher, err = nativeOpenCodeExecutable(launcher)
		if err != nil {
			return fmt.Errorf("HUMAN_SELECTION_REQUIRED: %w", err)
		}
	}
	cmd := exec.CommandContext(ctx, launcher, "export", "--sessionID="+req.SessionID)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("HUMAN_SELECTION_REQUIRED: opencode export failed: %s", detail)
	}
	return verifySelectionTurnBytesForRoot180(data, root, req)
}

func VerifySelectionTurnBytes180(data []byte, req SelectionTurnRequest180) error {
	return verifySelectionTurnBytesForRoot180(data, "", req)
}

func verifySelectionTurnBytesForRoot180(data []byte, root string, req SelectionTurnRequest180) error {
	if strings.TrimSpace(req.SessionID) == "" || strings.TrimSpace(req.MessageID) == "" || strings.TrimSpace(req.RunID) == "" || strings.TrimSpace(req.OptionsHash) == "" || strings.TrimSpace(req.MenuMarker) == "" {
		return errors.New("HUMAN_SELECTION_REQUIRED: missing Host/menu identity")
	}
	var exported sessionExport
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&exported); err != nil {
		return fmt.Errorf("HUMAN_SELECTION_REQUIRED: malformed session export: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("HUMAN_SELECTION_REQUIRED: trailing session JSON")
	}
	if stringValue(exported.Info["id"]) != req.SessionID {
		return errors.New("HUMAN_SELECTION_REQUIRED: session mismatch")
	}
	if root != "" {
		if err := verifyPrimaryDirectory(root, data); err != nil {
			return fmt.Errorf("HUMAN_SELECTION_REQUIRED: %w", err)
		}
	}

	userIndex, err := selectionUserIndex180(exported, req.MessageID)
	if err != nil {
		return err
	}

	previousUser := -1
	for i := userIndex - 1; i >= 0; i-- {
		if stringValue(exported.Messages[i].Info["role"]) == "user" {
			previousUser = i
			break
		}
	}

	latestMenuIndex := -1
	latestMenuMatches := false
	for i := userIndex - 1; i > previousUser; i-- {
		m := exported.Messages[i]
		if stringValue(m.Info["role"]) != "assistant" {
			continue
		}
		// Export arrays preserve message/part order. Bind only to the final
		// menu occurrence, even when menus share a message or text part.
		texts := []string{}
		for _, p := range m.Parts {
			if stringValue(p["type"]) == "text" {
				texts = append(texts, stringValue(p["text"]))
			}
		}
		text := strings.Join(texts, "\n")
		if selectionMenu180(text) {
			latestMenuIndex = i
			latestMenuMatches = currentSelectionMenu180(text, req)
			break
		}
	}
	if latestMenuIndex < 0 || !latestMenuMatches {
		return errors.New("HUMAN_SELECTION_REQUIRED: current menu must immediately precede the next real user interval")
	}

	for i := userIndex + 1; i < len(exported.Messages); i++ {
		m := exported.Messages[i]
		if stringValue(m.Info["role"]) != "assistant" {
			continue
		}
		for _, p := range m.Parts {
			if stringValue(p["type"]) == "text" && selectionMenu180(stringValue(p["text"])) {
				return errors.New("HUMAN_SELECTION_REQUIRED: selection reply belongs to a stale menu")
			}
		}
	}

	texts := []string{}
	for _, p := range exported.Messages[userIndex].Parts {
		if stringValue(p["type"]) != "text" {
			continue
		}
		if synthetic, _ := p["synthetic"].(bool); synthetic {
			return errors.New("HUMAN_SELECTION_REQUIRED: synthetic user input")
		}
		if ignored, _ := p["ignored"].(bool); ignored {
			continue
		}
		texts = append(texts, stringValue(p["text"]))
	}
	actual := strings.TrimSpace(strings.Join(texts, "\n"))
	ids := append([]string(nil), req.SelectionIDs...)
	all := append([]string(nil), req.AllIDs...)
	sort.Strings(ids)
	sort.Strings(all)
	expected := "选择 " + strings.Join(ids, ",")
	if len(ids) > 0 && len(ids) == len(all) {
		same := true
		for i := range ids {
			if ids[i] != all[i] {
				same = false
				break
			}
		}
		if same && actual == "全部" {
			return nil
		}
	}
	if actual != expected {
		return fmt.Errorf("HUMAN_SELECTION_REQUIRED: user reply must be %q", expected)
	}
	return nil
}

func selectionUserIndex180(exported sessionExport, messageID string) (int, error) {
	anchorIndex := -1
	for i, m := range exported.Messages {
		if stringValue(m.Info["id"]) != messageID {
			continue
		}
		if anchorIndex >= 0 {
			return -1, errors.New("HUMAN_SELECTION_REQUIRED: Host message id is ambiguous")
		}
		anchorIndex = i
	}
	if anchorIndex < 0 {
		return -1, errors.New("HUMAN_SELECTION_REQUIRED: Host turn unavailable")
	}
	anchor := exported.Messages[anchorIndex]
	switch stringValue(anchor.Info["role"]) {
	case "user":
		return anchorIndex, nil
	case "assistant":
		parentID := strings.TrimSpace(stringValue(anchor.Info["parentID"]))
		if parentID == "" {
			return -1, errors.New("HUMAN_SELECTION_REQUIRED: assistant Host context has no real user parent")
		}
		userIndex := -1
		for i := 0; i < anchorIndex; i++ {
			m := exported.Messages[i]
			if stringValue(m.Info["id"]) != parentID {
				continue
			}
			if userIndex >= 0 || stringValue(m.Info["role"]) != "user" {
				return -1, errors.New("HUMAN_SELECTION_REQUIRED: assistant Host context user parent is ambiguous")
			}
			userIndex = i
		}
		if userIndex < 0 {
			return -1, errors.New("HUMAN_SELECTION_REQUIRED: assistant Host context user parent unavailable")
		}
		for i := userIndex + 1; i < anchorIndex; i++ {
			if stringValue(exported.Messages[i].Info["role"]) == "user" {
				return -1, errors.New("HUMAN_SELECTION_REQUIRED: assistant Host context is not bound to the latest real user")
			}
		}
		return userIndex, nil
	default:
		return -1, errors.New("HUMAN_SELECTION_REQUIRED: Host context is neither a real user turn nor its assistant tool response")
	}
}

func selectionMenu180(text string) bool {
	// A malformed later header still invalidates an earlier menu.
	return strings.Contains(text, "options=")
}

func currentSelectionMenu180(text string, req SelectionTurnRequest180) bool {
	i := strings.LastIndex(text, "options=")
	if i < 0 {
		return false
	}
	start := strings.LastIndex(text[:i], "\n") + 1
	menu := text[start:]
	header, body, ok := strings.Cut(menu, "\n")
	// Match the exact run/hash pair in this occurrence. Never borrow a
	// hash or marker from an older menu in the same message.
	return ok && strings.TrimSpace(header) == req.RunID+" options="+req.OptionsHash &&
		strings.Contains("\n"+body+"\n", "\n"+req.MenuMarker+"\n")
}
