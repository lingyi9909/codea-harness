package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	req := report.ReviewRequest{
		RunID:          "review-001",
		HarnessVersion: "1.3.0",
		BaseRef:        "origin/develop",
		Head:           "abc123",
		Result:         report.ResultPassed,
		Scope:          report.ReviewScope{},
		Coverage:       report.ReviewCoverage{Status: "COMPLETE"},
		Findings:       []report.Finding{},
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(".code-harness", "runs", req.RunID, "requests", "review-report.json")
	writeFile(t, input, string(data))
	if err := run([]string{"report", "review", "--input", input}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(".code-harness", "runs", req.RunID, "review.md")); err != nil {
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
