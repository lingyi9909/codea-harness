package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func installContract(t *testing.T, name string) {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	source := filepath.Join(filepath.Dir(testFile), "..", "..", "..", "contracts", name)
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(".code-harness", "contracts", name), string(data))
}

func targetedChangeAnalysis(reviewedService bool) string {
	// Certified ChangeAnalysis must itself satisfy the complete Change Set coverage
	// contract. TARGETED review then independently verifies its selected scopedFiles.
	reviewed := `[
      {"path":"src/main/java/OrderController.java","role":"Controller","reason":"CHANGED"},
      {"path":"src/main/java/UnrelatedService.java","role":"Service","reason":"CHANGED"}`
	if reviewedService {
		reviewed += `,{"path":"src/main/java/OrderService.java","role":"Service","reason":"CALL_CHAIN"}`
	}
	reviewed += `]`
	return `{
  "reviewScope":{"currentBranch":"develop","baseRef":"origin/develop","baseCommit":"a","mergeBase":"a","headCommit":"b","includeWorkingTree":true},
  "changedFiles":[
    {"path":"src/main/java/OrderController.java","role":"Controller","sources":["COMMITTED"]},
    {"path":"src/main/java/UnrelatedService.java","role":"Service","sources":["COMMITTED"]}
  ],
  "affectedControllers":[{"controller":"OrderController","endpoints":["OrderController.approve"],"impactType":"DIRECT_CHANGE","sourceSymbols":["OrderController.approve"]}],
  "callChains":[{"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderService.approve"]}],
  "symbolLocations":[
    {"symbol":"OrderController.approve","path":"src/main/java/OrderController.java","role":"Controller","source":"FIND_SYMBOL"},
    {"symbol":"OrderService.approve","path":"src/main/java/OrderService.java","role":"Service","source":"FIND_SYMBOL"}
  ],
  "externalDependencies":[],
  "riskAreas":[],
  "reviewCoverage":{"status":"COMPLETE","reviewedFiles":` + reviewed + `,"unresolvedSymbols":[]}
}`
}

func targetedScopeJSON() string {
	return `{
  "mode":"TARGETED",
  "target":{"symbol":"OrderController.approve","kind":"METHOD"},
  "selectedCallChains":[{"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderService.approve"]}],
  "scopedFiles":["src/main/java/OrderController.java","src/main/java/OrderService.java"]
}`
}

func TestValidateReviewScopeUsesScopedCoverageNotFullChangedSet(t *testing.T) {
	withTempProject(t)
	installContract(t, "review-scope.schema.json")
	input := filepath.Join(".code-harness", "runs", "run-001", "requests", "review-scope.json")
	analysis := filepath.Join(".code-harness", "runs", "run-001", "analysis", "change-analysis.json")
	writeFile(t, input, targetedScopeJSON())
	writeFile(t, analysis, targetedChangeAnalysis(true))
	prepareCommittedCertifiedAnalysisFixture153(t, "run-001", analysis)

	if err := run([]string{"validate", "--schema", ".code-harness/contracts/review-scope.schema.json", "--input", input, "--format", "json", "--change-analysis", analysis}); err != nil {
		t.Fatalf("targeted scope should validate when Certified ChangeAnalysis is complete and selected scoped coverage is complete despite unrelated changed files: %v", err)
	}
}

func TestValidateReviewScopeRejectsMissingScopedReviewedFile(t *testing.T) {
	withTempProject(t)
	installContract(t, "review-scope.schema.json")
	input := filepath.Join(".code-harness", "runs", "run-002", "requests", "review-scope.json")
	analysis := filepath.Join(".code-harness", "runs", "run-002", "analysis", "change-analysis.json")
	writeFile(t, input, targetedScopeJSON())
	writeFile(t, analysis, targetedChangeAnalysis(false))
	prepareCommittedCertifiedAnalysisFixture153(t, "run-002", analysis)

	err := run([]string{"validate", "--schema", ".code-harness/contracts/review-scope.schema.json", "--input", input, "--format", "json", "--change-analysis", analysis})
	if err == nil || !strings.Contains(err.Error(), "review scope coverage incomplete") {
		t.Fatalf("expected scoped coverage rejection, err=%v", err)
	}
}
