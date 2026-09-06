package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/schema"
)

func Test162ReviewReliabilityTask1RequestSchemas(t *testing.T) {
	contracts := filepath.Clean(filepath.Join("..", "..", "..", "contracts"))
	fullReport := `{
		"runId":"r1",
		"harnessVersion":"1.6.2",
		"baseRef":"origin/master",
		"head":"0123456789012345678901234567890123456789",
		"result":"PASSED",
		"mode":"FULL",
		"reviewScope":{"changedFiles":[]},
		"reviewCoverage":{"reviewedFiles":[],"callChains":[],"externalDependencies":[],"unresolved":[],"missingReviewedFiles":[],"runtimeErrors":[],"status":"COMPLETE"},
		"findings":[]
	}`
	targetedReport := `{
		"runId":"r1",
		"harnessVersion":"1.6.2",
		"baseRef":"origin/master",
		"head":"0123456789012345678901234567890123456789",
		"result":"PASSED",
		"mode":"TARGETED",
		"target":{"symbol":"OrderController.create","kind":"METHOD"},
		"reviewScope":{"changedFiles":["src/main/java/acme/OrderController.java"],"scopedFiles":["src/main/java/acme/OrderController.java"]},
		"reviewCoverage":{"reviewedFiles":["src/main/java/acme/OrderController.java"],"callChains":[{"entryPoint":"OrderController.create","chain":["OrderController.create"]}],"externalDependencies":[],"unresolved":[],"missingReviewedFiles":[],"runtimeErrors":[],"status":"COMPLETE"},
		"findings":[]
	}`
	tests := []struct {
		name        string
		schemaName  string
		validJSON   string
		invalidJSON string
	}{
		{
			name:        "finding certify",
			schemaName:  "finding-certify-request.schema.json",
			validJSON:   `{"runId":"r1","proposalsPath":".code-harness/runs/r1/requests/finding-proposals.json"}`,
			invalidJSON: `{"runId":"r1","proposalsPath":".code-harness/runs/r1/requests/finding-proposals.json","unexpected":true}`,
		},
		{
			name:        "report review full",
			schemaName:  "report-review-request.schema.json",
			validJSON:   fullReport,
			invalidJSON: strings.Replace(fullReport, `"findings":[]`, `"findings":[{"id":"raw-agent-finding"}]`, 1),
		},
		{
			name:        "report review targeted",
			schemaName:  "report-review-request.schema.json",
			validJSON:   targetedReport,
			invalidJSON: strings.Replace(targetedReport, `"findings":[]`, `"unexpected":true,"findings":[]`, 1),
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

func Test162ReviewReliabilityTask1CommandsValidateSchemaBeforeStrictDecode(t *testing.T) {
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
			name:       "finding certify unknown field",
			file:       "finding-certify-request.json",
			body:       `{"runId":"r1","proposalsPath":".code-harness/runs/r1/requests/finding-proposals.json","unexpected":true}`,
			invoke:     func(path string) error { return runReviewCertifyFindings160([]string{"--input", path}) },
			wantPrefix: "FINDING_CERTIFY_REQUEST_SCHEMA_INVALID",
		},
		{
			name: "report review unknown field",
			file: "report-review.json",
			body: `{"runId":"r1","harnessVersion":"1.6.2","baseRef":"HEAD","head":"0123456789012345678901234567890123456789","result":"PASSED","mode":"FULL","reviewScope":{"changedFiles":[]},"reviewCoverage":{"reviewedFiles":[],"callChains":[],"externalDependencies":[],"unresolved":[],"missingReviewedFiles":[],"runtimeErrors":[],"status":"COMPLETE"},"findings":[],"unexpected":true}`,
			invoke:     func(path string) error { return runReviewReport([]string{"--input", path}) },
			wantPrefix: "REPORT_REVIEW_REQUEST_SCHEMA_INVALID",
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
				t.Fatalf("schema-invalid request unexpectedly reached downstream logic: %s", tc.body)
			}
			if !strings.Contains(err.Error(), tc.wantPrefix) {
				t.Fatalf("expected %s before strict/business decode, got %v", tc.wantPrefix, err)
			}
		})
	}
}
