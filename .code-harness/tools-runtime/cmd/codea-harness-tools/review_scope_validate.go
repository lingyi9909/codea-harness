package main

import (
	"fmt"

	"codea-harness-tools/internal/reviewscope"
)

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
