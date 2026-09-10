package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/changeset"
)

func writeReviewerAuthorityTestReceipt(t *testing.T, root, runID, kind, proposalRel string) {
	t.Helper()
	proposalBytes, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(proposalRel)))
	if err != nil {
		t.Fatalf("read Reviewer proposal fixture: %v", err)
	}
	sum := sha256.Sum256(proposalBytes)
	receiptName := "change-analysis-reviewer-authority.json"
	if kind == "findings" {
		receiptName = "finding-reviewer-authority.json"
	}
	sessionID := "ses_test_reviewer_child_" + runID
	messageID := "msg_test_reviewer_message_" + runID
	receipt := map[string]any{
		"version":        1,
		"host":           "opencode",
		"source":         "opencode-tool-context",
		"runId":          runID,
		"proposalKind":   kind,
		"agent":          "reviewer",
		"sessionId":      sessionID,
		"messageId":      messageID,
		"proposalPath":   proposalRel,
		"proposalSha256": hex.EncodeToString(sum[:]),
	}
	b, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	mustWrite153Cmd(t, filepath.Join(root, ".code-harness", "runs", runID, "requests", receiptName), string(append(b, '\n')))
	installReviewerAuthorityExportShim(t, runID, kind, sessionID, messageID, proposalBytes)
}

func installReviewerAuthorityExportShim(t *testing.T, runID, kind, sessionID, messageID string, proposalBytes []byte) {
	t.Helper()
	userMessageID := "msg_test_reviewer_user_" + runID
	exported := map[string]any{
		"info": map[string]any{
			"id":       sessionID,
			"parentID": "ses_test_parent_" + runID,
		},
		"messages": []any{
			map[string]any{
				"info":  map[string]any{"id": userMessageID, "role": "user", "agent": "reviewer"},
				"parts": []any{},
			},
			map[string]any{
				"info": map[string]any{"id": messageID, "role": "assistant", "parentID": userMessageID},
				"parts": []any{map[string]any{
					"type": "tool",
					"tool": "codea-reviewer-submit",
					"state": map[string]any{
						"status": "completed",
						"input":  map[string]any{"kind": kind, "runId": runID, "proposal": string(proposalBytes)},
					},
				}},
			},
		},
	}
	exportBytes, err := json.Marshal(exported)
	if err != nil {
		t.Fatal(err)
	}
	shimDir := t.TempDir()
	exportPath := filepath.Join(shimDir, "session-export.json")
	if err := os.WriteFile(exportPath, exportBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	unixShim := filepath.Join(shimDir, "opencode")
	if err := os.WriteFile(unixShim, []byte("#!/bin/sh\ncat \"$CODEA_TEST_OPENCODE_EXPORT\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	windowsShim := filepath.Join(shimDir, "opencode.cmd")
	if err := os.WriteFile(windowsShim, []byte("@echo off\r\ntype \"%CODEA_TEST_OPENCODE_EXPORT%\"\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(unixShim, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("CODEA_TEST_OPENCODE_EXPORT", exportPath)
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// canonicalAnalysisCertifyRequestFromExistingTest converts a previously
// Runtime-certified ChangeAnalysis fixture back to the Reviewer-owned semantic
// proposal shape, recomputes the current Runtime ChangeSet snapshot, and binds
// the proposal to a Host-derived Reviewer authority receipt. Tests using this
// helper therefore exercise the 1.6.4 authority chain instead of legacy
// draftPath certification.
func canonicalAnalysisCertifyRequestFromExistingTest(t *testing.T, root, runID, analysisPath, baseRef string, includeWorkingTree bool, intent analysisruntime.Intent) analysisruntime.CertifyRequest {
	t.Helper()
	analysisBytes, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(analysisPath)))
	if err != nil {
		t.Fatal(err)
	}
	var proposal map[string]any
	if err := json.Unmarshal(analysisBytes, &proposal); err != nil {
		t.Fatal(err)
	}
	changed, ok := proposal["changedFiles"].([]any)
	if !ok {
		t.Fatal("certified analysis fixture missing changedFiles")
	}
	roles := make([]map[string]any, 0, len(changed))
	for _, raw := range changed {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Fatal("certified analysis fixture contains invalid changedFiles entry")
		}
		pathValue, pathOK := item["path"].(string)
		roleValue, roleOK := item["role"].(string)
		if !pathOK || !roleOK {
			t.Fatal("certified analysis fixture changedFiles entry missing path/role")
		}
		roles = append(roles, map[string]any{"path": pathValue, "role": roleValue})
	}
	delete(proposal, "reviewScope")
	delete(proposal, "changedFiles")
	proposal["changedFileRoles"] = roles
	proposalBytes, err := json.MarshalIndent(proposal, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	proposalBytes = append(proposalBytes, '\n')
	proposalRel := filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "requests", "change-analysis-proposal.json"))
	mustWrite153Cmd(t, filepath.Join(root, filepath.FromSlash(proposalRel)), string(proposalBytes))
	writeReviewerAuthorityTestReceipt(t, root, runID, "change-analysis", proposalRel)

	snapshot, err := changeset.Compute(root, baseRef, includeWorkingTree)
	if err != nil {
		t.Fatalf("compute canonical ChangeSet fixture: %v", err)
	}
	snapshotBytes, err := changeset.CanonicalBytes(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	snapshotRel := filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "analysis", "change-set.json"))
	mustWrite153Cmd(t, filepath.Join(root, filepath.FromSlash(snapshotRel)), string(snapshotBytes))
	return analysisruntime.CertifyRequest{
		RunID:          runID,
		ProposalPath:   proposalRel,
		SnapshotPath:   snapshotRel,
		SnapshotSHA256: snapshot.SnapshotSHA256,
		Intent:         intent,
	}
}
