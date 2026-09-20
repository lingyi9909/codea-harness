package reviewselection

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/reviewauthority"
	"codea-harness-tools/internal/reviewscope"
)

func RecordSelection(root string, req SelectionRequest) error {
	if !reviewauthority.PrimaryFlow(root) {
		return nil
	}
	// LIST replaces any preceding choice. It never leaves an older FULL scope
	// usable; consumers verify this record before reading the scope artifact.
	data, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Join(root, ".code-harness", "runs", req.RunID, "analysis")
	f, err := os.CreateTemp(dir, ".selection-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(append(data, '\n')); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(dir, "review-selection-verified.json"))
}

// Rebuild the selected scope, including human Host attestation, for every
// downstream authority consumer. Neither a missing scope nor a fabricated
// review-scope.json can silently broaden the user's confirmed selection.
func VerifyExecutionScope(root, runID string) error {
	if !reviewauthority.PrimaryFlow(root) {
		return nil
	}
	dir := filepath.Join(root, ".code-harness", "runs", runID, "analysis")
	data, err := os.ReadFile(filepath.Join(dir, "review-selection-verified.json"))
	if os.IsNotExist(err) {
		// Preserve the existing automatic FULL consumer contract only when no menu
		// or explicit scope exists and certified analysis contains at most one
		// actual chain. Multi-chain runs never enter this compatibility path.
		_, menuErr := os.Stat(filepath.Join(dir, "review-options.json"))
		_, scopeErr := os.Stat(filepath.Join(dir, "review-scope.json"))
		if os.IsNotExist(menuErr) && os.IsNotExist(scopeErr) {
			analysis, cert, loadErr := analysisruntime.LoadCertified(root, filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json")))
			if loadErr != nil {
				return loadErr
			}
			chains := map[string]bool{}
			for _, c := range analysis.CallChains {
				b, _ := json.Marshal(c)
				chains[string(b)] = true
			}
			if len(chains) <= 1 && (cert.Intent == nil || cert.Intent.Mode == "FULL") {
				return nil
			}
		}
	}
	if err != nil {
		return fmt.Errorf("HUMAN_SELECTION_REQUIRED: Runtime selection unavailable: %w", err)
	}
	var req SelectionRequest
	if err := decodeStrictReviewArtifact153(data, &req); err != nil {
		return err
	}
	if req.RunID != runID || req.Mode == "LIST" {
		return fmt.Errorf("REVIEW_SELECTION_NOT_AUTHORIZED: LIST or mismatched run cannot execute review")
	}
	expected, err := VerifyAndBuildScope(root, req)
	if err != nil {
		return err
	}
	scopeBytes, err := os.ReadFile(filepath.Join(dir, "review-scope.json"))
	if err != nil {
		return fmt.Errorf("REVIEW_SELECTION_NOT_AUTHORIZED: missing scope: %w", err)
	}
	var scope reviewscope.Selection
	if err := decodeStrictReviewArtifact153(scopeBytes, &scope); err != nil {
		return err
	}
	actualJSON, err := json.Marshal(scope)
	if err != nil {
		return err
	}
	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		return err
	}
	if !bytes.Equal(actualJSON, expectedJSON) {
		return fmt.Errorf("REVIEW_SELECTION_NOT_AUTHORIZED: scope differs from confirmed Runtime selection")
	}
	return nil
}

// ReportScopeJSON derives the report boundary from the confirmed Runtime
// artifact. Agent transport cannot substitute different branches with the same
// target/files, and does not need to re-encode internal exact symbol refs.
func ReportScopeJSON(root, runID string, analysisJSON, proposedJSON []byte) ([]byte, error) {
	if !reviewauthority.PrimaryFlow(root) {
		return proposedJSON, nil
	}
	if err := VerifyExecutionScope(root, runID); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(root, ".code-harness", "runs", runID, "analysis", "review-scope.json"))
	if os.IsNotExist(err) {
		scope, err := reviewscope.BuildFullSelection(analysisJSON)
		if err != nil {
			return nil, err
		}
		return json.Marshal(scope)
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}
