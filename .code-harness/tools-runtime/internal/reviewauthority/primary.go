package reviewauthority

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// PrimaryFlow is a versioned contract: old certified runs retain the legacy
// Host protocol. A current run must also match the installed Runtime version.
func PrimaryFlow(root string) bool {
	b, err := os.ReadFile(filepath.Join(root, ".code-harness", "VERSION"))
	if err != nil {
		return false
	}
	parts := strings.Split(strings.TrimSpace(string(b)), ".")
	if len(parts) != 3 {
		return false
	}
	var v [3]int
	for i, p := range parts {
		n, e := strconv.Atoi(p)
		if e != nil {
			return false
		}
		v[i] = n
	}
	return v[0] > 1 || v[0] == 1 && (v[1] > 6 || v[1] == 6 && v[2] >= 6)
}

func verifyPrimarySession(data []byte, receipt Receipt, proposal []byte, selectionMenu ...string) error {
	var exported sessionExport
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&exported); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("trailing session JSON")
	}
	if stringValue(exported.Info["id"]) != receipt.SessionID || strings.TrimSpace(stringValue(exported.Info["parentID"])) != "" {
		return errors.New("submission requires the primary Host session")
	}
	for index, message := range exported.Messages {
		if stringValue(message.Info["id"]) != receipt.MessageID {
			continue
		}
		if stringValue(message.Info["role"]) != "assistant" || stringValue(message.Info["agent"]) != receipt.Agent {
			return errors.New("primary submission message identity mismatch")
		}
		parent := stringValue(message.Info["parentID"])
		userIndex := -1
		for i := 0; i < index; i++ {
			m := exported.Messages[i]
			if stringValue(m.Info["role"]) == "user" {
				userIndex = i
			}
		}
		if userIndex < 0 || stringValue(exported.Messages[userIndex].Info["id"]) != parent {
			return errors.New("submission is not bound to current primary user turn")
		}
		if receipt.ProposalKind == Selection {
			if err := verifyHumanSelection(exported, userIndex, receipt, proposal, selectionMenu...); err != nil {
				return err
			}
		}
		for _, part := range message.Parts {
			if stringValue(part["type"]) != "tool" || normalizeToolName(stringValue(part["tool"])) != "codeareviewersubmit" {
				continue
			}
			state, ok := mapValue(part["state"])
			if !ok || stringValue(state["status"]) != "completed" {
				continue
			}
			input, ok := mapValue(state["input"])
			if !ok {
				continue
			}
			if stringValue(input["runId"]) == receipt.RunID && Kind(stringValue(input["kind"])) == receipt.ProposalKind && sameJSON([]byte(stringValue(input["proposal"])), proposal) {
				return nil
			}
		}
		return errors.New("no matching completed primary submission")
	}
	return errors.New("primary submission message unavailable")
}

func verifyHumanSelection(exported sessionExport, userIndex int, receipt Receipt, proposal []byte, selectionMenu ...string) error {
	var req struct {
		RunID       string   `json:"runId"`
		Mode        string   `json:"mode"`
		OptionsHash string   `json:"optionsHash"`
		IDs         []string `json:"selectionIds"`
	}
	if err := json.Unmarshal(proposal, &req); err != nil {
		return err
	}
	if req.RunID != receipt.RunID || req.OptionsHash == "" {
		return errors.New("selection identity missing")
	}
	// The immediately preceding user interval must contain the displayed current
	// menu. An old menu elsewhere in a long session cannot authorize this choice.
	menu := false
	for i := userIndex - 1; i >= 0; i-- {
		m := exported.Messages[i]
		if stringValue(m.Info["role"]) == "user" {
			break
		}
		if stringValue(m.Info["role"]) != "assistant" {
			continue
		}
		for _, p := range m.Parts {
			text := stringValue(p["text"])
			if stringValue(p["type"]) == "text" && strings.Contains(text, req.RunID) && strings.Contains(text, req.OptionsHash) {
				if len(selectionMenu) == 0 || strings.Contains(text, selectionMenu[0]) {
					menu = true
				}
			}
		}
	}
	if !menu {
		return errors.New("HUMAN_SELECTION_REQUIRED: current menu must precede the next user turn")
	}
	var texts []string
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
	expected := ""
	switch req.Mode {
	case "FULL":
		expected = "全部"
	case "LIST":
		expected = "仅列出"
	case "TARGETED":
		expected = "选择 " + strings.Join(req.IDs, ",")
	}
	if expected == "" || actual != expected {
		return fmt.Errorf("HUMAN_SELECTION_REQUIRED: user reply must explicitly match current selection (%s)", expected)
	}
	return nil
}

func verifyPrimaryDirectory(root string, data []byte) error {
	var exported sessionExport
	if err := json.Unmarshal(data, &exported); err != nil {
		return err
	}
	directory := stringValue(exported.Info["directory"])
	if !filepath.IsAbs(directory) {
		return errors.New("primary Host project directory missing")
	}
	expected, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	expected, err = filepath.EvalSymlinks(expected)
	if err != nil {
		return err
	}
	actual, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return err
	}
	equal := filepath.Clean(expected) == filepath.Clean(actual)
	if runtime.GOOS == "windows" {
		equal = strings.EqualFold(filepath.Clean(expected), filepath.Clean(actual))
	}
	if !equal {
		return errors.New("primary Host session belongs to a different project")
	}
	return nil
}
