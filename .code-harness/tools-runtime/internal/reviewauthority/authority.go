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
	"path/filepath"
	"regexp"
	"strings"
)

type Kind string

const (
	ChangeAnalysis Kind = "change-analysis"
	Findings       Kind = "findings"
)

var artifactID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

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
	if strings.TrimSpace(receipt.SessionID) == "" || strings.TrimSpace(receipt.MessageID) == "" {
		return Receipt{}, hardStop("Reviewer Host session/message identity missing")
	}
	if filepath.ToSlash(filepath.Clean(receipt.ProposalPath)) != expectedProposal {
		return Receipt{}, hardStop("Reviewer authority receipt proposal path mismatch")
	}
	sum := sha256.Sum256(proposalBytes)
	actual := hex.EncodeToString(sum[:])
	if !strings.EqualFold(actual, receipt.ProposalSHA256) {
		return Receipt{}, hardStop("Reviewer authority receipt proposal hash mismatch")
	}
	return receipt, nil
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
