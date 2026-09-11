package reviewauthority

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

type Kind string

const (
	ChangeAnalysis Kind = "change-analysis"
	Findings       Kind = "findings"
)

var artifactID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
var sessionID = regexp.MustCompile(`^ses_[A-Za-z0-9_-]+$`)

type Receipt struct {
	Version        int    `json:"version"`
	Host           string `json:"host"`
	Source         string `json:"source"`
	RunID          string `json:"runId"`
	ProposalKind   Kind   `json:"proposalKind"`
	Agent          string `json:"agent"`
	SessionID      string `json:"sessionId"`
	MessageID      string `json:"messageId"`
	ProposalPath   string `json:"proposalPath"`
	ProposalSHA256 string `json:"proposalSha256"`
}

type sessionExport struct {
	Info     map[string]any `json:"info"`
	Messages []struct {
		Info  map[string]any   `json:"info"`
		Parts []map[string]any `json:"parts"`
	} `json:"messages"`
}

func Verify(repoRoot, runID string, kind Kind, proposalPath string) (Receipt, error) {
	if !artifactID.MatchString(runID) {
		return Receipt{}, hardStop("invalid runId")
	}
	expectedProposal, receiptName, err := canonicalPaths(runID, kind)
	if err != nil {
		return Receipt{}, hardStop(err.Error())
	}
	cleanProposal := filepath.ToSlash(filepath.Clean(proposalPath))
	if cleanProposal != expectedProposal {
		return Receipt{}, hardStop(fmt.Sprintf("proposal path %q is not canonical %q", cleanProposal, expectedProposal))
	}
	proposalBytes, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(expectedProposal)))
	if err != nil {
		return Receipt{}, hardStop("Reviewer proposal unavailable: " + err.Error())
	}
	receiptPath := filepath.Join(repoRoot, ".code-harness", "runs", runID, "requests", receiptName)
	receiptBytes, err := os.ReadFile(receiptPath)
	if err != nil {
		return Receipt{}, hardStop("Reviewer authority receipt unavailable: " + err.Error())
	}
	var receipt Receipt
	dec := json.NewDecoder(bytes.NewReader(receiptBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&receipt); err != nil {
		return Receipt{}, hardStop("Reviewer authority receipt malformed: " + err.Error())
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return Receipt{}, hardStop("Reviewer authority receipt contains trailing JSON")
	}
	if receipt.Version != 1 || receipt.Host != "opencode" || receipt.Source != "opencode-tool-context" || receipt.RunID != runID || receipt.ProposalKind != kind || receipt.Agent != "reviewer" {
		return Receipt{}, hardStop("Reviewer authority receipt identity mismatch")
	}
	if !sessionID.MatchString(receipt.SessionID) || strings.TrimSpace(receipt.MessageID) == "" {
		return Receipt{}, hardStop("Reviewer Host session/message identity missing or invalid")
	}
	if filepath.ToSlash(filepath.Clean(receipt.ProposalPath)) != expectedProposal {
		return Receipt{}, hardStop("Reviewer authority receipt proposal path mismatch")
	}
	sum := sha256.Sum256(proposalBytes)
	actual := hex.EncodeToString(sum[:])
	if !strings.EqualFold(actual, receipt.ProposalSHA256) {
		return Receipt{}, hardStop("Reviewer authority receipt proposal hash mismatch")
	}

	exportBytes, err := exportOpenCodeSession(repoRoot, receipt.SessionID)
	if err != nil {
		return Receipt{}, hardStop("Reviewer Host session attestation unavailable: " + err.Error())
	}
	if err := verifySessionAttestation(exportBytes, receipt, proposalBytes); err != nil {
		return Receipt{}, hardStop("Reviewer Host session attestation invalid: " + err.Error())
	}
	return receipt, nil
}

func exportOpenCodeSession(repoRoot, id string) ([]byte, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd.exe", "/d", "/s", "/c", "opencode export "+id)
	} else {
		cmd = exec.Command("opencode", "export", id)
	}
	cmd.Dir = repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf("opencode export %s failed: %s", id, detail)
	}
	return out, nil
}

func verifySessionAttestation(exportBytes []byte, receipt Receipt, proposalBytes []byte) error {
	var exported sessionExport
	dec := json.NewDecoder(bytes.NewReader(exportBytes))
	dec.UseNumber()
	if err := dec.Decode(&exported); err != nil {
		return fmt.Errorf("decode exported session: %w", err)
	}
	if stringValue(exported.Info["id"]) != receipt.SessionID {
		return errors.New("exported session id does not match receipt")
	}
	if strings.TrimSpace(stringValue(exported.Info["parentID"])) == "" {
		return errors.New("Reviewer session is not an independent child session")
	}

	reviewerUsers := map[string]struct{}{}
	for _, message := range exported.Messages {
		if stringValue(message.Info["role"]) == "user" && stringValue(message.Info["agent"]) == "reviewer" {
			id := stringValue(message.Info["id"])
			if id != "" {
				reviewerUsers[id] = struct{}{}
			}
		}
	}
	if len(reviewerUsers) == 0 {
		return errors.New("exported session has no Reviewer-owned user turn")
	}

	for _, message := range exported.Messages {
		if stringValue(message.Info["id"]) != receipt.MessageID {
			continue
		}
		if stringValue(message.Info["role"]) != "assistant" {
			return errors.New("receipt message is not an assistant tool turn")
		}
		parentID := stringValue(message.Info["parentID"])
		if _, ok := reviewerUsers[parentID]; !ok {
			return errors.New("receipt message is not bound to a Reviewer-owned user turn")
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
			if stringValue(input["runId"]) != receipt.RunID || Kind(stringValue(input["kind"])) != receipt.ProposalKind {
				continue
			}
			proposal, ok := input["proposal"].(string)
			if !ok || !sameJSON([]byte(proposal), proposalBytes) {
				continue
			}
			return nil
		}
		return errors.New("receipt message has no matching completed Reviewer submission tool call")
	}
	return errors.New("receipt message id not found in exported Reviewer session")
}

func sameJSON(a, b []byte) bool {
	var av any
	var bv any
	adec := json.NewDecoder(bytes.NewReader(a))
	adec.UseNumber()
	if err := adec.Decode(&av); err != nil {
		return false
	}
	bdec := json.NewDecoder(bytes.NewReader(b))
	bdec.UseNumber()
	if err := bdec.Decode(&bv); err != nil {
		return false
	}
	ac, err := json.Marshal(av)
	if err != nil {
		return false
	}
	bc, err := json.Marshal(bv)
	return err == nil && bytes.Equal(ac, bc)
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

func mapValue(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

func normalizeToolName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func canonicalPaths(runID string, kind Kind) (string, string, error) {
	root := filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "requests"))
	switch kind {
	case ChangeAnalysis:
		return root + "/change-analysis-proposal.json", "change-analysis-reviewer-authority.json", nil
	case Findings:
		return root + "/finding-proposals.json", "finding-reviewer-authority.json", nil
	default:
		return "", "", fmt.Errorf("unsupported Reviewer proposal kind %q", kind)
	}
}

func hardStop(detail string) error {
	return fmt.Errorf("REVIEWER_UNAVAILABLE: MANUAL_ACTION_REQUIRED: HARD STOP: %s", detail)
}
