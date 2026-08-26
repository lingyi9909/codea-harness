package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/reviewscope"
)

func loadCertifiedReviewScopeAnalysis153(selectionPath, changeAnalysisPath string) ([]byte, error) {
	runID, _, err := validateAnalysisRequestPath153(selectionPath)
	if err != nil {
		return nil, fmt.Errorf("REVIEW_SCOPE_REQUEST_PATH_INVALID: %w", err)
	}
	expected := filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json"))
	actual := filepath.ToSlash(filepath.Clean(strings.TrimSpace(changeAnalysisPath)))
	if actual != expected {
		return nil, fmt.Errorf("REVIEW_SCOPE_ANALYSIS_RUN_MISMATCH: got %q want %q", actual, expected)
	}
	certified, _, err := analysisruntime.LoadCertified(".", expected)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(certified)
	if err != nil {
		return nil, fmt.Errorf("encode Certified ChangeAnalysis for ReviewScope: %w", err)
	}
	return b, nil
}

func validateReviewScopeAgainstAnalysis(selectionJSON, changeAnalysisJSON []byte) (reviewscope.Selection, reviewscope.CoverageResult, error) {
	selection, err := reviewscope.Verify(selectionJSON, changeAnalysisJSON)
	if err != nil {
		return reviewscope.Selection{}, reviewscope.CoverageResult{}, err
	}
	machine, err := reviewscope.ComputeCoverageFromAnalysis(selection, changeAnalysisJSON)
	if err != nil {
		return reviewscope.Selection{}, reviewscope.CoverageResult{}, err
	}
	if machine.Status != "COMPLETE" {
		return selection, machine, fmt.Errorf("review scope coverage incomplete: missingFiles=%v unresolvedSymbols=%v", machine.MissingFiles, machine.UnresolvedSymbols)
	}
	return selection, machine, nil
}
