package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"codea-harness-tools/internal/chain"
	"codea-harness-tools/internal/coverage"
	"codea-harness-tools/internal/schema"
)

type chainDiscoverRequest struct {
	RunID              string `json:"runId"`
	Target             string `json:"target,omitempty"`
	ChangeAnalysisPath string `json:"changeAnalysisPath"`
}

func runChain(args []string) error {
	if len(args) == 0 {
		return errors.New("chain requires discover")
	}
	switch args[0] {
	case "discover":
		return runChainDiscover(args[1:])
	default:
		return fmt.Errorf("unknown chain action %q", args[0])
	}
}

func runChainDiscover(args []string) error {
	fs := flag.NewFlagSet("chain discover", flag.ContinueOnError)
	inputPath := fs.String("input", "", "chain discovery request under .code-harness/runs/<runId>/requests/*.json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *inputPath == "" {
		return errors.New("chain discover requires --input")
	}

	pathRunID, cleanInput, err := validateChainDiscoveryRequestPath(*inputPath)
	if err != nil {
		return err
	}
	requestBytes, err := os.ReadFile(cleanInput)
	if err != nil {
		return fmt.Errorf("read chain discovery request: %w", err)
	}
	req, err := decodeChainDiscoveryRequest(requestBytes)
	if err != nil {
		return err
	}
	if req.RunID != pathRunID {
		return fmt.Errorf("RUN_ID_PATH_MISMATCH: body runId %q does not match request path runId %q", req.RunID, pathRunID)
	}
	if !validChainArtifactID(req.RunID) {
		return errors.New("chain discovery request contains invalid runId")
	}
	if !sameRunChangeAnalysisPath(req.ChangeAnalysisPath, req.RunID) {
		return errors.New("chain discovery changeAnalysisPath must be .code-harness/runs/<runId>/analysis/change-analysis.json for the same run")
	}

	analysisBytes, err := os.ReadFile(filepath.Clean(req.ChangeAnalysisPath))
	if err != nil {
		return fmt.Errorf("read chain discovery ChangeAnalysis: %w", err)
	}
	analysisSchema, err := os.ReadFile(filepath.Join(".code-harness", "contracts", "change-analysis.schema.json"))
	if err != nil {
		return fmt.Errorf("read ChangeAnalysis schema: %w", err)
	}
	if err := schema.ValidateJSON(analysisSchema, analysisBytes); err != nil {
		return fmt.Errorf("validate chain discovery ChangeAnalysis: %w", err)
	}
	if _, err := coverage.VerifyAnalysisJSON(analysisBytes); err != nil {
		return fmt.Errorf("verify chain discovery ChangeAnalysis: %w", err)
	}
	var evidence chain.ChangeAnalysisEvidence
	if err := json.Unmarshal(analysisBytes, &evidence); err != nil {
		return fmt.Errorf("decode verified chain discovery ChangeAnalysis: %w", err)
	}

	result, err := chain.Discover(".", chain.DiscoverInput{
		RunID:          req.RunID,
		Target:         req.Target,
		ChangeAnalysis: evidence,
	})
	if err != nil {
		return err
	}
	return writeJSONAndStatus(result, result.Status == chain.DiscoveryComplete)
}

func decodeChainDiscoveryRequest(data []byte) (chainDiscoverRequest, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var req chainDiscoverRequest
	if err := dec.Decode(&req); err != nil {
		return chainDiscoverRequest{}, fmt.Errorf("decode chain discovery request: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return chainDiscoverRequest{}, errors.New("decode chain discovery request: multiple JSON values are not allowed")
		}
		return chainDiscoverRequest{}, fmt.Errorf("decode chain discovery request: %w", err)
	}
	if strings.TrimSpace(req.RunID) == "" || strings.TrimSpace(req.ChangeAnalysisPath) == "" {
		return chainDiscoverRequest{}, errors.New("chain discovery request requires runId and changeAnalysisPath")
	}
	return req, nil
}

func validateChainDiscoveryRequestPath(inputPath string) (string, string, error) {
	if filepath.IsAbs(inputPath) {
		return "", "", invalidChainDiscoveryRequestPath()
	}
	clean := filepath.Clean(inputPath)
	parts := strings.Split(filepath.ToSlash(clean), "/")
	if len(parts) != 5 || !strings.EqualFold(parts[0], ".code-harness") || !strings.EqualFold(parts[1], "runs") || !strings.EqualFold(parts[3], "requests") || !strings.EqualFold(filepath.Ext(parts[4]), ".json") || parts[4] == ".json" || !validChainArtifactID(parts[2]) {
		return "", "", invalidChainDiscoveryRequestPath()
	}
	return parts[2], clean, nil
}

func invalidChainDiscoveryRequestPath() error {
	return errors.New("chain discover input must be under .code-harness/runs/<runId>/requests/*.json")
}

func sameRunChangeAnalysisPath(path, runID string) bool {
	if filepath.IsAbs(path) || !validChainArtifactID(runID) {
		return false
	}
	clean := filepath.Clean(path)
	want := filepath.Clean(filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json"))
	return clean == want
}

func validChainArtifactID(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	for i := 0; i < len(value); i++ {
		b := value[i]
		if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '.' || b == '_' || b == '-' {
			continue
		}
		return false
	}
	return true
}
