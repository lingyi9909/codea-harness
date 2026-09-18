package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test180ReviewStartCommandWritesIncompleteReport(t *testing.T) {
	withTempProject(t)
	if err := run([]string{"review", "start"}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(".code-harness", "runs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		t.Fatalf("expected exactly one review run directory, got %+v", entries)
	}
	runID := entries[0].Name()
	reportPath := filepath.Join(".code-harness", "runs", runID, "review.md")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("评审未完成")) {
		t.Fatalf("missing incomplete report marker: %s", data)
	}
	if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		t.Fatal("BOM")
	}
	if err := run([]string{"review", "status", "--run-id", runID}); err != nil {
		t.Fatal(err)
	}
}

func Test180ReviewPrepareAndSelectRoutes(t *testing.T) {
	withTempProject(t)
	if err := run([]string{"review", "start"}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(".code-harness", "runs"))
	if err != nil {
		t.Fatal(err)
	}
	runID := entries[0].Name()
	t.Setenv("CODEA_AST_GREP_TEST_PATH", "")
	t.Setenv("PATH", t.TempDir())
	if err := run([]string{"review", "prepare", "--run-id", runID, "--mode", "CURRENT_IMPLEMENTATION", "--target", "OrderController"}); err != nil {
		t.Fatalf("prepare route failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(".code-harness", "runs", runID, "options.json")); err != nil {
		t.Fatalf("prepare route did not persist options: %v", err)
	}
	if err := run([]string{"review", "select"}); err == nil || !strings.Contains(err.Error(), "--options-hash") {
		t.Fatalf("select did not route to 1.8 command: %v", err)
	}
}

func Test180ReviewCancelCommandKeepsSameReport(t *testing.T) {
	withTempProject(t)
	if err := run([]string{"review", "start"}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(".code-harness", "runs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one run, got %d", len(entries))
	}
	runID := entries[0].Name()
	if err := run([]string{"review", "cancel", "--run-id", runID, "--reason", "user cancelled"}); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(".code-harness", "runs", runID, "review.md")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("评审已取消")) {
		t.Fatalf("cancel did not update same report: %s", data)
	}
}

func Test180ReviewFinishCommandRejectsRunIDMismatch(t *testing.T) {
	withTempProject(t)
	if err := run([]string{"review", "start"}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(".code-harness", "runs"))
	if err != nil {
		t.Fatal(err)
	}
	runID := entries[0].Name()
	requestsDir := filepath.Join(".code-harness", "runs", runID, "requests")
	if err := os.MkdirAll(requestsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{"runId": "review-00000000000000000000000000000000", "reads": []any{}, "findings": []any{}, "pendingRisks": []any{}, "gaps": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(requestsDir, "finish.json")
	if err := os.WriteFile(input, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"review", "finish", "--input", input}); err == nil {
		t.Fatal("expected body/path runId mismatch rejection")
	}
}

func Test180ReviewFinishIngressIsBOMCompatibleAndStrict(t *testing.T) {
	withTempProject(t)
	if err := run([]string{"review", "start"}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(".code-harness", "runs"))
	if err != nil {
		t.Fatal(err)
	}
	runID := entries[0].Name()
	requestsDir := filepath.Join(".code-harness", "runs", runID, "requests")
	if err := os.MkdirAll(requestsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(requestsDir, "finish.json")
	full := `{"runId":"` + runID + `","reads":[],"findings":[],"pendingRisks":[],"gaps":[]}`

	t.Run("single UTF-8 BOM is accepted by the wire decoder", func(t *testing.T) {
		payload := append([]byte{0xef, 0xbb, 0xbf}, []byte(full)...)
		if err := os.WriteFile(input, payload, 0o600); err != nil {
			t.Fatal(err)
		}
		err := run([]string{"review", "finish", "--input", input})
		if err == nil || !strings.Contains(err.Error(), "REVIEW_FINISH_SCOPE_NOT_READY") {
			t.Fatalf("BOM request did not reach semantic finish validation: %v", err)
		}
	})

	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "missing reads", body: `{"runId":"` + runID + `","findings":[],"pendingRisks":[],"gaps":[]}`},
		{name: "missing findings", body: `{"runId":"` + runID + `","reads":[],"pendingRisks":[],"gaps":[]}`},
		{name: "missing pending risks", body: `{"runId":"` + runID + `","reads":[],"findings":[],"gaps":[]}`},
		{name: "missing gaps", body: `{"runId":"` + runID + `","reads":[],"findings":[],"pendingRisks":[]}`},
		{name: "legacy chainRefs", body: `{"runId":"` + runID + `","reads":[],"findings":[],"pendingRisks":[],"gaps":[],"chainRefs":[]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(input, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			err := run([]string{"review", "finish", "--input", input})
			if err == nil || !strings.Contains(err.Error(), "REVIEW_FINISH_REQUEST_INVALID") {
				t.Fatalf("invalid finish request was not rejected at ingress: %v", err)
			}
		})
	}
}
