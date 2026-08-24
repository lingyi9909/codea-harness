package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"codea-harness-tools/internal/coverage"
	"codea-harness-tools/internal/reviewscope"
	"codea-harness-tools/internal/schema"
)

type chainReviewContextRequest struct {
	RunID                  string          `json:"runId"`
	ChangeAnalysisPath     string          `json:"changeAnalysisPath"`
	ReviewScope            json.RawMessage `json:"reviewScope"`
	AllowTemporaryForStale bool            `json:"allowTemporaryForStale,omitempty"`
}

func runChainReviewContext(args []string) error {
	fs := flag.NewFlagSet("chain review-context", flag.ContinueOnError)
	inputPath := fs.String("input", "", "review chain context request under .code-harness/runs/<runId>/requests/*.json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *inputPath == "" {
		return errors.New("chain review-context requires --input")
	}

	pathRunID, cleanInput, err := validateChainRequestPath(*inputPath)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(cleanInput)
	if err != nil {
		return fmt.Errorf("read chain review-context request: %w", err)
	}
	var req chainReviewContextRequest
	if err := decodeStrictChainRequest(data, &req, "chain review-context request"); err != nil {
		return err
	}
	if req.RunID != pathRunID || !validChainArtifactID(req.RunID) {
		return fmt.Errorf("RUN_ID_PATH_MISMATCH: body runId %q does not match request path runId %q", req.RunID, pathRunID)
	}
	if !sameRunChangeAnalysisPath(req.ChangeAnalysisPath, req.RunID) {
		return errors.New("chain review-context changeAnalysisPath must be under the same run")
	}
	if len(req.ReviewScope) == 0 {
		return errors.New("chain review-context requires reviewScope")
	}

	analysisBytes, err := os.ReadFile(filepath.Clean(req.ChangeAnalysisPath))
	if err != nil {
		return fmt.Errorf("read review-context ChangeAnalysis: %w", err)
	}
	analysisSchema, err := os.ReadFile(filepath.Join(".code-harness", "contracts", "change-analysis.schema.json"))
	if err != nil {
		return fmt.Errorf("read ChangeAnalysis schema: %w", err)
	}
	if err := schema.ValidateJSON(analysisSchema, analysisBytes); err != nil {
		return fmt.Errorf("validate review-context ChangeAnalysis: %w", err)
	}
	reviewScopeSchema, err := os.ReadFile(filepath.Join(".code-harness", "contracts", "review-scope.schema.json"))
	if err != nil {
		return fmt.Errorf("read ReviewScope schema: %w", err)
	}
	if err := schema.ValidateJSON(reviewScopeSchema, req.ReviewScope); err != nil {
		return fmt.Errorf("validate review-context ReviewScope: %w", err)
	}
	selection, err := reviewscope.Verify(req.ReviewScope, analysisBytes)
	if err != nil {
		return fmt.Errorf("verify review-context scope: %w", err)
	}
	machine, err := reviewscope.ComputeCoverageFromAnalysis(selection, analysisBytes)
	if err != nil {
		return fmt.Errorf("compute review-context scoped coverage: %w", err)
	}
	if machine.Status != "COMPLETE" {
		return fmt.Errorf("review-context scope coverage incomplete: missingFiles=%v unresolvedSymbols=%v", machine.MissingFiles, machine.UnresolvedSymbols)
	}
	if selection.Mode == "FULL" {
		if _, err := coverage.VerifyAnalysisJSON(analysisBytes); err != nil {
			return fmt.Errorf("verify review-context FULL coverage: %w", err)
		}
	}

	result, err := reviewscope.ResolveChainContexts(".", selection, analysisBytes, reviewscope.ChainResolveOptions{
		RunID:                  req.RunID,
		AllowTemporaryForStale: req.AllowTemporaryForStale,
	})
	if err != nil {
		return err
	}
	return writeJSONAndStatus(result, result.Status != reviewscope.ChainResolutionPartial)
}
