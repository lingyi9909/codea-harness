package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codea-harness-tools/internal/chain"
	"codea-harness-tools/internal/schema"
)

func runChainEdit(args []string) error {
	fs := flag.NewFlagSet("chain edit", flag.ContinueOnError)
	inputPath := fs.String("input", "", "chain edit request under .code-harness/runs/<runId>/requests/*.json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || strings.TrimSpace(*inputPath) == "" {
		return errors.New("chain edit requires --input")
	}

	pathRunID, cleanInput, err := validateChainRequestPath(*inputPath)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(cleanInput)
	if err != nil {
		return fmt.Errorf("read chain edit request: %w", err)
	}
	contract, err := os.ReadFile(filepath.Join(".code-harness", "contracts", "chain-edit-request.schema.json"))
	if err != nil {
		return fmt.Errorf("read chain edit request contract: %w", err)
	}
	if err := schema.ValidateJSON(contract, data); err != nil {
		return fmt.Errorf("CHAIN_EDIT_REQUEST_INVALID: %w", err)
	}
	var req chain.EditRequest
	if err := decodeStrictChainRequest(data, &req, "chain edit request"); err != nil {
		return fmt.Errorf("CHAIN_EDIT_REQUEST_INVALID: %w", err)
	}
	if req.RunID != pathRunID || !validChainArtifactID(req.RunID) {
		return fmt.Errorf("RUN_ID_PATH_MISMATCH: body runId %q does not match request path runId %q", req.RunID, pathRunID)
	}
	if err := chain.ValidateID(req.ChainID); err != nil {
		return fmt.Errorf("CHAIN_EDIT_REQUEST_INVALID: %w", err)
	}
	if !sameRunChangeAnalysisPath(req.ChangeAnalysisPath, req.RunID) {
		return errors.New("CHAIN_EDIT_REQUEST_INVALID: changeAnalysisPath must be .code-harness/runs/<runId>/analysis/change-analysis.json for the same run")
	}

	result, err := chain.ApplyVerifiedEdit(".", req)
	if err != nil {
		return err
	}
	return writeJSONAndStatus(result, result.Status == "EDIT_READY")
}
