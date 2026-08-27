package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"codea-harness-tools/internal/report"
)

func TestReportDispatchRecognizesReport(t *testing.T) {
	err := run([]string{"report"})
	if err == nil || strings.Contains(err.Error(), "unknown subcommand") {
		t.Fatalf("report dispatch not wired: %v", err)
	}
}

func TestReportReviewWritesArtifactAndDeletesTransport(t *testing.T) {
	withTempProject(t)
	runID := "review-001"
	analysisPath := filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json")
	writeFile(t, analysisPath, fullReportAnalysis153())
	prepareCommittedCertifiedAnalysisFixture153(t, runID, analysisPath)
	input := writeReportTransport153(t, runID)
	if err := run([]string{"report", "review", "--input", input}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(".code-harness", "runs", runID, "review.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(input); !os.IsNotExist(err) {
		t.Fatalf("transport should be deleted, stat err=%v", err)
	}
}

func TestReportReviewRejectsInputOutsideRunRequests(t *testing.T) {
	withTempProject(t)
	req := report.ReviewRequest{
		RunID:          "review-001",
		HarnessVersion: "1.3.0",
		BaseRef:        "origin/develop",
		Head:           "abc123",
		Result:         report.ResultPassed,
		Coverage:       report.ReviewCoverage{Status: "COMPLETE"},
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, "review-report.json", string(data))
	err = run([]string{"report", "review", "--input", "review-report.json"})
	if err == nil || !strings.Contains(err.Error(), "must be under .code-harness/runs/<runId>/requests") {
		t.Fatalf("err=%v", err)
	}
}

func installApiDocSchema(t *testing.T) {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(0)
	if !ok { t.Fatal("locate test source") }
	source := filepath.Join(filepath.Dir(testFile), "..", "..", "..", "contracts", "api-doc.schema.json")
	data, err := os.ReadFile(source)
	if err != nil { t.Fatal(err) }
	writeFile(t, filepath.Join(".code-harness", "contracts", "api-doc.schema.json"), string(data))
}

func TestReportApiDocWritesArtifactAndDeletesTransport(t *testing.T) {
	withTempProject(t)
	installApiDocSchema(t)
	body := `{
	  "runId":"api-doc-001",
	  "harnessVersion":"1.2.0",
	  "apiDoc":{"controllers":[{"name":"OrderController","apis":[{
	    "title":"Get order","httpMethod":"GET","path":"/orders/{id}","description":"Get order",
	    "request":{"fields":[],"example":{}},
	    "response":{"fields":[],"example":{}},
	    "permissions":[],"preconditions":[],"businessFlow":[],"stateTransitions":[],
	    "dataEffects":[],"externalEffects":[],"transactions":[],"idempotency":[],
	    "errorCodes":[],"testCoverage":[],"businessNotes":[]
	  }]}]}
	}`
	input := filepath.Join(".code-harness", "runs", "api-doc-001", "requests", "api-doc.json")
	writeFile(t, input, body)
	if err := run([]string{"report", "api-doc", "--input", input}); err != nil { t.Fatal(err) }
	artifact := filepath.Join(".code-harness", "runs", "api-doc-001", "api-doc.md")
	if _, err := os.Stat(artifact); err != nil { t.Fatal(err) }
	if _, err := os.Stat(input); !os.IsNotExist(err) { t.Fatalf("transport should be deleted, stat err=%v", err) }
}

func TestReportApiDocRejectsUnknownAction(t *testing.T) {
	err := run([]string{"report", "something-else"})
	if err == nil || !strings.Contains(err.Error(), "unknown report action") { t.Fatalf("err=%v", err) }
}
