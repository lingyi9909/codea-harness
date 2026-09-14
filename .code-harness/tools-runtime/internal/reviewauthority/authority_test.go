package reviewauthority

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeReviewerAuthorityFixture(t *testing.T, session, message string) (string, string) {
	t.Helper()
	root := t.TempDir()
	runID := "run-reviewer-real-session"
	proposalRel := filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "requests", "change-analysis-proposal.json"))
	proposal := []byte("{}\n")
	proposalPath := filepath.Join(root, filepath.FromSlash(proposalRel))
	if err := os.MkdirAll(filepath.Dir(proposalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(proposalPath, proposal, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(proposal)
	receipt := Receipt{
		Version:        1,
		Host:           "opencode",
		Source:         "opencode-tool-context",
		RunID:          runID,
		ProposalKind:   ChangeAnalysis,
		Agent:          "reviewer",
		SessionID:      session,
		MessageID:      message,
		ProposalPath:   proposalRel,
		ProposalSHA256: hex.EncodeToString(sum[:]),
	}
	data, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	receiptPath := filepath.Join(root, ".code-harness", "runs", runID, "requests", "change-analysis-reviewer-authority.json")
	if err := os.WriteFile(receiptPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return root, runID
}

func TestReviewerAuthorityAcceptsHostOpaqueSessionIdentity(t *testing.T) {
	root, runID := writeReviewerAuthorityFixture(t, "reviewer-session-change-analysis.", "message-reviewer-1")
	_, err := Verify(root, runID, ChangeAnalysis, filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "requests", "change-analysis-proposal.json")))
	if err == nil {
		t.Fatal("fixture has no exported OpenCode session and must not fully verify")
	}
	if strings.Contains(err.Error(), "session/message identity missing or invalid") {
		t.Fatalf("opaque non-empty Host session identity must pass the format-independent identity gate: %v", err)
	}
	if !strings.Contains(err.Error(), "Reviewer Host session attestation unavailable") {
		t.Fatalf("expected verification to advance to Host session attestation, got %v", err)
	}
}

func TestReviewerAuthorityRejectsEmptyHostIdentity(t *testing.T) {
	root, runID := writeReviewerAuthorityFixture(t, "", "")
	_, err := Verify(root, runID, ChangeAnalysis, filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "requests", "change-analysis-proposal.json")))
	if err == nil || !strings.Contains(err.Error(), "session/message identity missing or invalid") {
		t.Fatalf("empty Host identity must fail closed before session attestation, got %v", err)
	}
}
