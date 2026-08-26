package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/report"
)

func Test153ReportReviewRejectsUncertifiedAnalysis(t *testing.T) {
	withTempProject(t)
	runID := "run-153-report-uncertified"
	input := writeReportTransport153(t, runID)
	if err := run([]string{"report", "review", "--input", input}); err == nil || !strings.Contains(err.Error(), "CERTIFICATE_READ_FAILED") {
		t.Fatalf("review report must reject uncertified analysis, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(".code-harness", "runs", runID, "review.md")); !os.IsNotExist(err) {
		t.Fatalf("uncertified report must publish no review.md, stat=%v", err)
	}
}

func Test153ReportReviewUsesCertifiedIdentityAndCoverageInsteadOfTransportAuthority(t *testing.T) {
	withTempProject(t)
	runID := "run-153-report-certified"
	analysisPath := filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json")
	writeFile(t, analysisPath, fullReportAnalysis153())
	prepareCommittedCertifiedAnalysisFixture153(t, runID, analysisPath)
	input := writeReportTransport153(t, runID)

	if err := run([]string{"report", "review", "--input", input}); err != nil {
		t.Fatalf("certified review report must pass: %v", err)
	}
	out, err := os.ReadFile(filepath.Join(".code-harness", "runs", runID, "review.md"))
	if err != nil { t.Fatal(err) }
	text := string(out)
	for _, bad := range []string{"agent-version", "agent-base", "agent-head", "src/main/java/Evil.java"} {
		if strings.Contains(text, bad) {
			t.Fatalf("Agent self-reported authority leaked into report: %q\n%s", bad, text)
		}
	}
	if !strings.Contains(text, "src/main/java/OrderController.java") || !strings.Contains(text, "1.5.2") {
		t.Fatalf("report missing Certified Runtime authority: %s", text)
	}
}

func writeReportTransport153(t *testing.T, runID string) string {
	t.Helper()
	req := report.ReviewRequest{
		RunID: runID,
		HarnessVersion: "agent-version",
		BaseRef: "agent-base",
		Head: "agent-head",
		Result: report.ResultPassed,
		Mode: "FULL",
		Scope: report.ReviewScope{ChangedFiles: []string{"src/main/java/Evil.java"}},
		Coverage: report.ReviewCoverage{
			ReviewedFiles: []string{"src/main/java/Evil.java"},
			CallChains: []report.CallChain{}, ExternalDependencies: []string{}, Unresolved: []string{},
			MissingReviewedFiles: []string{}, RuntimeErrors: []string{}, Status: "COMPLETE",
		},
		Findings: []report.Finding{},
	}
	b, err := json.Marshal(req)
	if err != nil { t.Fatal(err) }
	input := filepath.Join(".code-harness", "runs", runID, "requests", "review-report.json")
	writeFile(t, input, string(b))
	return input
}

func fullReportAnalysis153() string {
	return `{
  "reviewScope":{"currentBranch":"develop","baseRef":"HEAD~1","baseCommit":"base","mergeBase":"base","headCommit":"head","includeWorkingTree":true},
  "changedFiles":[{"path":"src/main/java/OrderController.java","role":"Controller","sources":["COMMITTED"]}],
  "affectedControllers":[{"controller":"OrderController","endpoints":["OrderController.approve"],"impactType":"DIRECT_CHANGE","sourceSymbols":["OrderController.approve"]}],
  "callChains":[{"entryPoint":"OrderController.approve","chain":["OrderController.approve"]}],
  "symbolLocations":[{"symbol":"OrderController.approve","path":"src/main/java/OrderController.java","role":"Controller","source":"FIND_SYMBOL"}],
  "resourceRelations":[],"externalDependencies":[],"riskAreas":[],
  "reviewCoverage":{"status":"COMPLETE","reviewedFiles":[{"path":"src/main/java/OrderController.java","role":"Controller","reason":"CHANGED"}],"unresolvedSymbols":[]}
}`
}
