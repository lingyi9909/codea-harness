package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/reviewprogress"
)

func Test170KnowledgeCommandReturnsVerifiedDocumentsWithoutPersistingBodies(t *testing.T) {
	options := task153BuildReviewOptions(t)
	if len(options.AutoSelectionIDs) != 1 {
		t.Fatalf("fixture must produce AUTO_SINGLE: %+v", options)
	}
	selection := writeQueryRequest(t, "run-task4-review", "review-knowledge-select.json", `{"runId":"run-task4-review","mode":"TARGETED","selectionIds":["`+options.AutoSelectionIDs[0]+`"],"optionsHash":"`+options.OptionsHash+`"}`)
	if err := run([]string{"review", "select", "--input", selection}); err != nil {
		t.Fatalf("review select: %v", err)
	}
	copyTask153CommandContract(t, ".", "review-unit.schema.json")
	copyTask153CommandContract(t, ".", "review-knowledge.schema.json")
	if err := run([]string{"review", "units", "--run-id", "run-task4-review"}); err != nil {
		t.Fatalf("review units: %v", err)
	}

	if _, err := reviewprogress.Begin170(".", "run-task4-review"); err != nil {
		t.Fatalf("begin 1.7 progress: %v", err)
	}
	for _, stage := range []string{reviewprogress.StageSnapshot, reviewprogress.StageChangeAnalysis, reviewprogress.StageCertification} {
		if _, err := reviewprogress.Advance(".", "run-task4-review", stage); err != nil {
			t.Fatalf("advance %s: %v", stage, err)
		}
	}

	body := "Verified RULES body sentinel."
	rule := `---
ruleId: ORDER-KNOWLEDGE-001
projectId: order-service
status: ACTIVE
version: "1"
owner: order-team
source: requirements/order.md
approvalRef: approvals/ORDER-KNOWLEDGE
appliesTo:
  paths: ['./']
  entryPoints: []
supersedes: []
exceptions: []
---
` + body + "\n"
	writeFile(t, filepath.Join("docs", "business", "order-rule.md"), rule)
	writeFile(t, filepath.Join("docs", "business", "unbound.md"), "UNVERIFIED SOURCE SENTINEL\n")
	writeFile(t, filepath.Join(".code-harness", "context.yaml"), "version: 1\nprojectId: order-service\nsources:\n  - id: order-rule\n    root: PROJECT\n    path: docs/business/order-rule.md\n    kind: RULES\n    required: true\n")
	request := writeQueryRequest(t, "run-task4-review", "review-knowledge-blocker.json", `{"runId":"run-task4-review"}`)

	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	commandErr := run([]string{"review", "knowledge", "--input", request})
	_ = writer.Close()
	os.Stdout = oldStdout
	out, readErr := io.ReadAll(reader)
	_ = reader.Close()
	if commandErr != nil {
		t.Fatalf("review knowledge: %v", commandErr)
	}
	if readErr != nil {
		t.Fatal(readErr)
	}

	artifactPath := filepath.Join(".code-harness", "runs", "run-task4-review", "analysis", "review-knowledge.json")
	artifact, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(artifact), body) || strings.Contains(string(artifact), `"documents"`) || strings.Contains(string(artifact), `"content"`) {
		t.Fatalf("persisted review-knowledge artifact leaked run-local document bodies: %s", artifact)
	}

	var result struct {
		Manifest struct {
			Sources []struct {
				SourceID string `json:"sourceId"`
				SHA256   string `json:"sha256"`
			} `json:"sources"`
		} `json:"manifest"`
		Documents []struct {
			SourceID string `json:"sourceId"`
			Content  string `json:"content"`
		} `json:"documents"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("decode command result %q: %v", out, err)
	}
	if len(result.Documents) != 1 || result.Documents[0].SourceID != "order-rule" || result.Documents[0].Content != rule {
		t.Fatalf("verified document was not returned by command: %s", out)
	}
	if strings.Contains(string(out), "UNVERIFIED SOURCE SENTINEL") {
		t.Fatalf("unbound source bypassed Runtime knowledge authority: %s", out)
	}
	if len(result.Manifest.Sources) != 1 || result.Manifest.Sources[0].SourceID != "order-rule" {
		t.Fatalf("unexpected manifest sources: %s", out)
	}
	digest := sha256.Sum256([]byte(rule))
	if result.Manifest.Sources[0].SHA256 != hex.EncodeToString(digest[:]) {
		t.Fatalf("document content does not match manifest source digest: result=%s want=%x", result.Manifest.Sources[0].SHA256, digest)
	}
}
