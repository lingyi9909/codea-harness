package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test153ReviewScopeRejectsUncertifiedAnalysis(t *testing.T) {
	withTempProject(t)
	installContract(t, "review-scope.schema.json")
	installContract(t, "change-analysis.schema.json")
	runID := "run-153-review-uncertified"
	input, analysisPath := writeReviewScopeFixture153(t, runID)

	err := run([]string{"validate", "--schema", ".code-harness/contracts/review-scope.schema.json", "--input", input, "--format", "json", "--change-analysis", analysisPath})
	if err == nil || !strings.Contains(err.Error(), "CERTIFICATE_READ_FAILED") {
		t.Fatalf("uncertified review scope must fail closed, got %v", err)
	}
}

func Test153ReviewScopeRejectsTamperedCertifiedAnalysis(t *testing.T) {
	withTempProject(t)
	installContract(t, "review-scope.schema.json")
	runID := "run-153-review-tampered"
	input, analysisPath := writeReviewScopeFixture153(t, runID)
	prepareCommittedCertifiedAnalysisFixture153(t, runID, analysisPath)
	data, err := os.ReadFile(analysisPath)
	if err != nil { t.Fatal(err) }
	if err := os.WriteFile(analysisPath, append(data, ' '), 0o644); err != nil { t.Fatal(err) }

	err = run([]string{"validate", "--schema", ".code-harness/contracts/review-scope.schema.json", "--input", input, "--format", "json", "--change-analysis", analysisPath})
	if err == nil || !strings.Contains(err.Error(), "CHANGED_ANALYSIS_HASH_MISMATCH") {
		t.Fatalf("tampered Certified ChangeAnalysis must be rejected by review scope, got %v", err)
	}
}

func Test153ReviewScopeRejectsStaleCertifiedChangeSet(t *testing.T) {
	withTempProject(t)
	installContract(t, "review-scope.schema.json")
	runID := "run-153-review-stale"
	input, analysisPath := writeReviewScopeFixture153(t, runID)
	prepareCommittedCertifiedAnalysisFixture153(t, runID, analysisPath)
	writeFile(t, "src/main/java/OrderController.java", "package fixture; class OrderController { int stale = 99; }\n")

	err := run([]string{"validate", "--schema", ".code-harness/contracts/review-scope.schema.json", "--input", input, "--format", "json", "--change-analysis", analysisPath})
	if err == nil || !strings.Contains(err.Error(), "CERTIFIED_CHANGE_SET_STALE") {
		t.Fatalf("stale Certified ChangeAnalysis must be rejected by review scope, got %v", err)
	}
}

func Test153ReviewScopeAcceptsCertifiedAnalysis(t *testing.T) {
	withTempProject(t)
	installContract(t, "review-scope.schema.json")
	runID := "run-153-review-certified"
	input, analysisPath := writeReviewScopeFixture153(t, runID)
	prepareCommittedCertifiedAnalysisFixture153(t, runID, analysisPath)

	if err := run([]string{"validate", "--schema", ".code-harness/contracts/review-scope.schema.json", "--input", input, "--format", "json", "--change-analysis", analysisPath}); err != nil {
		t.Fatalf("certified review scope must pass: %v", err)
	}
}

func writeReviewScopeFixture153(t *testing.T, runID string) (string, string) {
	t.Helper()
	input := filepath.Join(".code-harness", "runs", runID, "requests", "review-scope.json")
	analysisPath := filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json")
	writeFile(t, input, targetedScopeJSON())
	writeFile(t, analysisPath, targetedChangeAnalysis(true))
	return input, analysisPath
}
