package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/coverage"
	"codea-harness-tools/internal/schema"
)

// useLegacyChainAnalysisLoader153 keeps pre-1.5.3 command tests focused on
// their original Chain/Review behavior. It is test-only: production defaults
// to analysis.LoadCertified and the dedicated 1.5.3 tests explicitly restore
// that strict loader before asserting fail-closed behavior.
func useLegacyChainAnalysisLoader153(t *testing.T) {
	t.Helper()
	previous := loadCertifiedAnalysis153
	loadCertifiedAnalysis153 = func(root, analysisPath string) (analysisruntime.ChangeAnalysis, analysisruntime.Certificate, error) {
		runID, ok := chainAnalysisRunID(analysisPath)
		if !ok {
			return analysisruntime.ChangeAnalysis{}, analysisruntime.Certificate{}, fmt.Errorf("legacy test loader: invalid ChangeAnalysis path %q", analysisPath)
		}
		data, err := os.ReadFile(filepath.Join(filepath.Clean(root), filepath.Clean(analysisPath)))
		if err != nil {
			return analysisruntime.ChangeAnalysis{}, analysisruntime.Certificate{}, err
		}
		schemaBytes, err := os.ReadFile(filepath.Join(filepath.Clean(root), ".code-harness", "contracts", "change-analysis.schema.json"))
		if err != nil {
			return analysisruntime.ChangeAnalysis{}, analysisruntime.Certificate{}, err
		}
		if err := schema.ValidateJSON(schemaBytes, data); err != nil {
			return analysisruntime.ChangeAnalysis{}, analysisruntime.Certificate{}, err
		}
		if _, err := coverage.VerifyAnalysisJSON(data); err != nil {
			return analysisruntime.ChangeAnalysis{}, analysisruntime.Certificate{}, err
		}
		var analysis analysisruntime.ChangeAnalysis
		if err := json.Unmarshal(data, &analysis); err != nil {
			return analysisruntime.ChangeAnalysis{}, analysisruntime.Certificate{}, err
		}
		return analysis, analysisruntime.Certificate{RunID: runID}, nil
	}
	t.Cleanup(func() { loadCertifiedAnalysis153 = previous })
}

func requireCertifiedChainAnalysis153(t *testing.T) {
	t.Helper()
	previous := loadCertifiedAnalysis153
	loadCertifiedAnalysis153 = analysisruntime.LoadCertified
	t.Cleanup(func() { loadCertifiedAnalysis153 = previous })
}
