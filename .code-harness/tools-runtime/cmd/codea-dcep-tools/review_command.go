package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codea-harness-tools/internal/reviewselection"
	"codea-harness-tools/internal/schema"
)

type reviewOptionsRequest struct {
	RunID              string `json:"runId"`
	ChangeAnalysisPath string `json:"changeAnalysisPath"`
	Target             string `json:"target,omitempty"`
}

func runReview(args []string) error {
	if len(args) == 0 {
		return errors.New("review requires options or select")
	}
	switch args[0] {
	case "options":
		return runReviewOptions(args[1:])
	case "select":
		return runReviewSelect(args[1:])
	default:
		return fmt.Errorf("unknown review action %q", args[0])
	}
}

func runReviewOptions(args []string) error {
	fs := flag.NewFlagSet("review options", flag.ContinueOnError)
	inputPath := fs.String("input", "", "review options request under .code-harness/runs/<runId>/requests/*.json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || strings.TrimSpace(*inputPath) == "" {
		return errors.New("review options requires --input")
	}
	pathRunID, cleanInput, err := validateChainRequestPath(*inputPath)
	if err != nil {
		return err
	}
	requestBytes, err := os.ReadFile(cleanInput)
	if err != nil {
		return fmt.Errorf("read review options request: %w", err)
	}
	var req reviewOptionsRequest
	if err := decodeStrictChainRequest(requestBytes, &req, "review options request"); err != nil {
		return err
	}
	if req.RunID != pathRunID || !validChainArtifactID(req.RunID) {
		return fmt.Errorf("RUN_ID_PATH_MISMATCH: body runId %q does not match request path runId %q", req.RunID, pathRunID)
	}
	if !sameRunChangeAnalysisPath(req.ChangeAnalysisPath, req.RunID) {
		return errors.New("review options changeAnalysisPath must be .code-harness/runs/<runId>/analysis/change-analysis.json for the same run")
	}
	options, err := reviewselection.BuildOptions(".", req.ChangeAnalysisPath, req.Target)
	if err != nil {
		return err
	}
	optionsBytes, err := canonicalReviewJSON153(options)
	if err != nil {
		return err
	}
	if err := validateReviewContract153("review-options.schema.json", optionsBytes); err != nil {
		return err
	}
	origin, err := reviewselection.BuildOptionsOrigin(".", req.ChangeAnalysisPath, options)
	if err != nil {
		return err
	}
	originBytes, err := canonicalReviewJSON153(origin)
	if err != nil {
		return err
	}
	analysisDir := filepath.Join(".code-harness", "runs", req.RunID, "analysis")
	originPath := filepath.Join(analysisDir, "review-options-origin.json")
	artifactPath := filepath.Join(analysisDir, "review-options.json")
	// Persist the immutable Runtime-owned origin before the consumable options.
	// A select operation requires both artifacts and re-derives options from the
	// origin plus current certified facts, so a partial write cannot authorize a scope.
	if err := atomicReviewWrite153(originPath, originBytes); err != nil {
		return err
	}
	if err := atomicReviewWrite153(artifactPath, optionsBytes); err != nil {
		return err
	}
	return writeJSONAndStatus(map[string]any{"status": "READY", "artifactPath": filepath.ToSlash(artifactPath), "options": options}, true)
}

func runReviewSelect(args []string) error {
	fs := flag.NewFlagSet("review select", flag.ContinueOnError)
	inputPath := fs.String("input", "", "review selection request under .code-harness/runs/<runId>/requests/*.json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || strings.TrimSpace(*inputPath) == "" {
		return errors.New("review select requires --input")
	}
	pathRunID, cleanInput, err := validateChainRequestPath(*inputPath)
	if err != nil {
		return err
	}
	requestBytes, err := os.ReadFile(cleanInput)
	if err != nil {
		return fmt.Errorf("read review selection request: %w", err)
	}
	if err := validateReviewContract153("review-selection-request.schema.json", requestBytes); err != nil {
		return err
	}
	var req reviewselection.SelectionRequest
	if err := decodeStrictChainRequest(requestBytes, &req, "review selection request"); err != nil {
		return err
	}
	if req.RunID != pathRunID || !validChainArtifactID(req.RunID) {
		return fmt.Errorf("RUN_ID_PATH_MISMATCH: body runId %q does not match request path runId %q", req.RunID, pathRunID)
	}
	scope, err := reviewselection.VerifyAndBuildScope(".", req)
	if err != nil {
		return err
	}
	if strings.EqualFold(req.Mode, "LIST") {
		return writeJSONAndStatus(map[string]any{"status": "LIST_ONLY", "runId": req.RunID, "findingReviewAuthorized": false}, true)
	}
	scopeBytes, err := canonicalReviewJSON153(scope)
	if err != nil {
		return err
	}
	if err := validateReviewContract153("review-scope.schema.json", scopeBytes); err != nil {
		return fmt.Errorf("REVIEW_SELECTION_SCOPE_INVALID: %w", err)
	}
	artifactPath := filepath.Join(".code-harness", "runs", req.RunID, "analysis", "review-scope.json")
	if err := atomicReviewWrite153(artifactPath, scopeBytes); err != nil {
		return err
	}
	return writeJSONAndStatus(map[string]any{"status": "SELECTED", "artifactPath": filepath.ToSlash(artifactPath), "reviewScope": scope}, true)
}

func canonicalReviewJSON153(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("REVIEW_ARTIFACT_ENCODE_FAILED: %w", err)
	}
	return append(data, '\n'), nil
}

func validateReviewContract153(name string, data []byte) error {
	contractBytes, err := os.ReadFile(filepath.Join(".code-harness", "contracts", name))
	if err != nil {
		return fmt.Errorf("REVIEW_CONTRACT_READ_FAILED: %s: %w", name, err)
	}
	if err := schema.ValidateJSON(contractBytes, data); err != nil {
		return fmt.Errorf("REVIEW_CONTRACT_INVALID: %s: %w", name, err)
	}
	return nil
}

func atomicReviewWrite153(path string, data []byte) error {
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("REVIEW_ARTIFACT_DIR_FAILED: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".review-artifact-*.tmp")
	if err != nil {
		return fmt.Errorf("REVIEW_ARTIFACT_TEMP_FAILED: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("REVIEW_ARTIFACT_WRITE_FAILED: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("REVIEW_ARTIFACT_WRITE_FAILED: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("REVIEW_ARTIFACT_REPLACE_FAILED: %w", err)
	}
	return nil
}
