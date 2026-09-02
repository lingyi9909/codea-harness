package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/schema"
)

func Test162HotfixTask2ZeroArgUsageUsesRuntimeName(t *testing.T) {
	err := run(nil)
	if err == nil {
		t.Fatal("expected zero-arg usage error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "codea-dcep-tools") {
		t.Fatalf("usage must name codea-dcep-tools, got %q", msg)
	}
	if strings.Contains(msg, "codea-harness-tools") {
		t.Fatalf("usage must not name legacy runtime, got %q", msg)
	}
}

func Test162HotfixTask2InvocationRequestSchemas(t *testing.T) {
	contracts := filepath.Clean(filepath.Join("..", "..", "..", "contracts"))
	tests := []struct {
		name        string
		schemaName  string
		validJSON   string
		invalidJSON string
	}{
		{
			name:        "snapshot",
			schemaName:  "change-set-request.schema.json",
			validJSON:   `{"runId":"r1","baseRef":"HEAD","includeWorkingTree":true}`,
			invalidJSON: `{"runId":"r1","requestedBaseRef":"HEAD","includeWorkingTree":true}`,
		},
		{
			name:        "inventory",
			schemaName:  "analysis-inventory-request.schema.json",
			validJSON:   `{"runId":"r1","baseRef":"HEAD","includeWorkingTree":true,"intent":{"mode":"FULL"}}`,
			invalidJSON: `{"runId":"r1","baseRef":"HEAD","includeWorkingTree":true,"intent":{"mode":"FULL"},"unexpected":true}`,
		},
		{
			name:        "canonical certify",
			schemaName:  "analysis-certify-request.schema.json",
			validJSON:   `{"runId":"r1","snapshotPath":".code-harness/runs/r1/analysis/change-set.json","snapshotSha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","proposalPath":".code-harness/runs/r1/requests/change-analysis-proposal.json","intent":{"mode":"FULL"}}`,
			invalidJSON: `{"runId":"r1","snapshotPath":".code-harness/runs/r1/analysis/change-set.json","snapshotSha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","proposalPath":".code-harness/runs/r1/requests/change-analysis-proposal.json","intent":{"mode":"FULL"},"unexpected":true}`,
		},
		{
			name:        "review options",
			schemaName:  "review-options-request.schema.json",
			validJSON:   `{"runId":"r1","changeAnalysisPath":".code-harness/runs/r1/analysis/change-analysis.json"}`,
			invalidJSON: `{"runId":"r1","changeAnalysisPath":".code-harness/runs/r1/analysis/change-analysis.json","baseRef":"HEAD"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			schemaBytes, err := os.ReadFile(filepath.Join(contracts, tc.schemaName))
			if err != nil {
				t.Fatalf("read %s: %v", tc.schemaName, err)
			}
			if err := schema.ValidateJSON(schemaBytes, []byte(tc.validJSON)); err != nil {
				t.Fatalf("valid request rejected by %s: %v", tc.schemaName, err)
			}
			if err := schema.ValidateJSON(schemaBytes, []byte(tc.invalidJSON)); err == nil {
				t.Fatalf("invalid request unexpectedly accepted by %s", tc.schemaName)
			}
		})
	}
}

func Test162HotfixTask2CommandEntryRejectsSchemaInvalidFields(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	requestDir := filepath.Join(".code-harness", "runs", "r1", "requests")
	if err := os.MkdirAll(requestDir, 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		file       string
		body       string
		invoke     func(string) error
		wantPrefix string
	}{
		{
			name: "snapshot requestedBaseRef",
			file: "snapshot.json",
			body: `{"runId":"r1","requestedBaseRef":"HEAD","includeWorkingTree":true}`,
			invoke: func(path string) error { return runAnalysisSnapshot162([]string{"--input", path}) },
			wantPrefix: "CHANGE_SET_REQUEST_SCHEMA_INVALID",
		},
		{
			name: "inventory unknown field",
			file: "inventory.json",
			body: `{"runId":"r1","baseRef":"HEAD","includeWorkingTree":true,"intent":{"mode":"FULL"},"unexpected":true}`,
			invoke: func(path string) error { return runAnalysisInventory([]string{"--input", path}) },
			wantPrefix: "ANALYSIS_INVENTORY_REQUEST_SCHEMA_INVALID",
		},
		{
			name: "certify unknown field",
			file: "certify.json",
			body: `{"runId":"r1","snapshotPath":".code-harness/runs/r1/analysis/change-set.json","snapshotSha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","proposalPath":".code-harness/runs/r1/requests/change-analysis-proposal.json","intent":{"mode":"FULL"},"unexpected":true}`,
			invoke: func(path string) error { return runAnalysisCertify([]string{"--input", path}) },
			wantPrefix: "ANALYSIS_CERTIFY_REQUEST_SCHEMA_INVALID",
		},
		{
			name: "review options baseRef",
			file: "review-options.json",
			body: `{"runId":"r1","changeAnalysisPath":".code-harness/runs/r1/analysis/change-analysis.json","baseRef":"HEAD"}`,
			invoke: func(path string) error { return runReviewOptions([]string{"--input", path}) },
			wantPrefix: "REVIEW_OPTIONS_REQUEST_SCHEMA_INVALID",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(requestDir, tc.file)
			if err := os.WriteFile(path, []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			err := tc.invoke(filepath.ToSlash(path))
			if err == nil {
				t.Fatalf("schema-invalid request unexpectedly reached command logic: %s", tc.body)
			}
			if !strings.Contains(err.Error(), tc.wantPrefix) {
				t.Fatalf("expected %s, got %v", tc.wantPrefix, err)
			}
		})
	}
}
