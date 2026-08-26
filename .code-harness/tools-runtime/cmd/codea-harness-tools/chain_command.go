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

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/chain"
)

var loadCertifiedAnalysis153 = analysisruntime.LoadCertified

type chainDiscoverRequest struct {
	RunID              string `json:"runId"`
	Target             string `json:"target,omitempty"`
	ChangeAnalysisPath string `json:"changeAnalysisPath"`
}

type chainRefreshRequest struct {
	RunID          string `json:"runId"`
	ID             string `json:"id"`
	DiscoveredPath string `json:"discoveredPath"`
}

type chainSealPersistRequest struct {
	RunID               string `json:"runId"`
	CandidatePath        string `json:"candidatePath"`
	ExpectedExistingHash string `json:"expectedExistingHash,omitempty"`
}

type chainPersistRequest struct {
	RunID  string `json:"runId"`
	PlanID string `json:"planId"`
}

func runChain(args []string) error {
	if len(args) == 0 {
		return errors.New("chain requires list, show, discover, review-context, refresh, validate, seal-persist, or persist")
	}
	switch args[0] {
	case "list":
		return runChainList(args[1:])
	case "show":
		return runChainShow(args[1:])
	case "discover":
		return runChainDiscover(args[1:])
	case "review-context":
		return runChainReviewContext(args[1:])
	case "refresh":
		return runChainRefresh(args[1:])
	case "validate":
		return runChainValidate(args[1:])
	case "seal-persist":
		return runChainSealPersist(args[1:])
	case "persist":
		return runChainPersist(args[1:])
	default:
		return fmt.Errorf("unknown chain action %q", args[0])
	}
}

func runChainList(args []string) error {
	if len(args) != 0 {
		return errors.New("chain list takes no arguments")
	}
	items, err := chain.List(".")
	if err != nil {
		return err
	}
	return writeJSONAndStatus(map[string]any{"status": "OK", "chains": items}, true)
}

func runChainShow(args []string) error {
	fs := flag.NewFlagSet("chain show", flag.ContinueOnError)
	target := fs.String("target", "", "exact chain id, Controller, or Controller.method")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || strings.TrimSpace(*target) == "" {
		return errors.New("chain show requires --target")
	}
	c, err := findProjectChain(".", strings.TrimSpace(*target))
	if err != nil {
		return err
	}
	fmt.Print(chain.RenderChinese(c))
	return nil
}

func runChainValidate(args []string) error {
	fs := flag.NewFlagSet("chain validate", flag.ContinueOnError)
	id := fs.String("id", "", "project chain id")
	analysisPath := fs.String("change-analysis", "", "certified ChangeAnalysis under .code-harness/runs/<runId>/analysis/change-analysis.json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || strings.TrimSpace(*id) == "" || strings.TrimSpace(*analysisPath) == "" {
		return errors.New("chain validate requires --id and --change-analysis")
	}
	path, err := chain.ChainPath(".", strings.TrimSpace(*id))
	if err != nil {
		return err
	}
	c, err := chain.Load(path)
	if err != nil {
		return err
	}
	analysis, _, err := loadVerifiedChainAnalysis(*analysisPath)
	if err != nil {
		return err
	}
	result := chain.Validate(".", c, chain.EvidenceSnapshot(analysis))
	return writeJSONAndStatus(result, result.Status == chain.ValidationValid)
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

	pathRunID, cleanInput, err := validateChainRequestPath(*inputPath)
	if err != nil {
		return err
	}
	requestBytes, err := os.ReadFile(cleanInput)
	if err != nil {
		return fmt.Errorf("read chain discovery request: %w", err)
	}
	var req chainDiscoverRequest
	if err := decodeStrictChainRequest(requestBytes, &req, "chain discovery request"); err != nil {
		return err
	}
	if strings.TrimSpace(req.RunID) == "" || strings.TrimSpace(req.ChangeAnalysisPath) == "" {
		return errors.New("chain discovery request requires runId and changeAnalysisPath")
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

	evidence, cert, err := loadCertifiedChainAnalysis(req.ChangeAnalysisPath)
	if err != nil {
		return err
	}
	result, err := chain.Discover(".", chain.DiscoverInput{RunID: req.RunID, Target: req.Target, ChangeAnalysis: evidence})
	if err != nil {
		return err
	}
	for _, candidate := range result.Chains {
		candidatePath := filepath.ToSlash(filepath.Join(".code-harness", "runs", req.RunID, "analysis", "discovered-chains", candidate.ID+".yaml"))
		if _, err := chain.CertifyCandidate(".", candidate, candidatePath, "DISCOVERED", cert); err != nil {
			return fmt.Errorf("certify discovered chain candidate: %w", err)
		}
	}
	return writeJSONAndStatus(result, result.Status == chain.DiscoveryComplete)
}

func runChainRefresh(args []string) error {
	fs := flag.NewFlagSet("chain refresh", flag.ContinueOnError)
	inputPath := fs.String("input", "", "chain refresh request under .code-harness/runs/<runId>/requests/*.json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *inputPath == "" {
		return errors.New("chain refresh requires --input")
	}
	pathRunID, cleanInput, err := validateChainRequestPath(*inputPath)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(cleanInput)
	if err != nil {
		return err
	}
	var req chainRefreshRequest
	if err := decodeStrictChainRequest(data, &req, "chain refresh request"); err != nil {
		return err
	}
	if req.RunID != pathRunID || !validChainArtifactID(req.RunID) {
		return fmt.Errorf("RUN_ID_PATH_MISMATCH: body runId %q does not match request path runId %q", req.RunID, pathRunID)
	}
	if err := chain.ValidateID(req.ID); err != nil {
		return err
	}
	if !sameRunChainAnalysisArtifact(req.DiscoveredPath, req.RunID, "discovered-chains") {
		return errors.New("chain refresh discoveredPath must be under the same run analysis/discovered-chains directory")
	}
	analysisPath := filepath.ToSlash(filepath.Join(".code-harness", "runs", req.RunID, "analysis", "change-analysis.json"))
	_, cert, err := loadCertifiedChainAnalysis(analysisPath)
	if err != nil {
		return fmt.Errorf("chain refresh requires certified ChangeAnalysis: %w", err)
	}
	existingPath, err := chain.ChainPath(".", req.ID)
	if err != nil {
		return err
	}
	existing, err := chain.Load(existingPath)
	if err != nil {
		return fmt.Errorf("load existing chain: %w", err)
	}
	discovered, _, err := chain.LoadRuntimeCandidate(".", filepath.ToSlash(filepath.Clean(req.DiscoveredPath)), cert)
	if err != nil {
		return fmt.Errorf("load Runtime-certified discovered chain: %w", err)
	}
	result := chain.Refresh(".", existing, discovered)
	if len(result.Errors) != 0 {
		return writeJSONAndStatus(result, false)
	}
	candidatePath := filepath.Join(".code-harness", "runs", req.RunID, "analysis", "refresh-candidates", req.ID+".yaml")
	if err := os.MkdirAll(filepath.Dir(candidatePath), 0o755); err != nil {
		return err
	}
	candidateBytes, err := chain.MarshalYAML(result.Candidate)
	if err != nil {
		return err
	}
	if err := os.WriteFile(candidatePath, candidateBytes, 0o644); err != nil {
		return fmt.Errorf("write refresh candidate: %w", err)
	}
	if _, err := chain.CertifyCandidate(".", result.Candidate, filepath.ToSlash(candidatePath), "REFRESH", cert); err != nil {
		return fmt.Errorf("certify refresh chain candidate: %w", err)
	}
	return writeJSONAndStatus(map[string]any{"status": "DIFF_READY", "candidatePath": filepath.ToSlash(candidatePath), "refresh": result}, true)
}

func runChainSealPersist(args []string) error {
	fs := flag.NewFlagSet("chain seal-persist", flag.ContinueOnError)
	inputPath := fs.String("input", "", "chain persistence sealing request under .code-harness/runs/<runId>/requests/*.json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *inputPath == "" {
		return errors.New("chain seal-persist requires --input")
	}
	pathRunID, cleanInput, err := validateChainRequestPath(*inputPath)
	if err != nil { return err }
	data, err := os.ReadFile(cleanInput)
	if err != nil { return err }
	var req chainSealPersistRequest
	if err := decodeStrictChainRequest(data, &req, "chain seal-persist request"); err != nil {
		return err
	}
	if req.RunID != pathRunID || !validChainArtifactID(req.RunID) {
		return fmt.Errorf("RUN_ID_PATH_MISMATCH: body runId %q does not match request path runId %q", req.RunID, pathRunID)
	}
	if strings.TrimSpace(req.CandidatePath) == "" {
		return errors.New("chain seal-persist request requires candidatePath")
	}
	plan, err := chain.SealWritePlan(".", req.RunID, req.CandidatePath, req.ExpectedExistingHash)
	if err != nil { return err }
	planPath := filepath.ToSlash(filepath.Join(".code-harness", "runs", req.RunID, "analysis", "chain-write-plans", plan.PlanID+".json"))
	return writeJSONAndStatus(map[string]any{"status": "SEALED", "planId": plan.PlanID, "planPath": planPath, "plan": plan}, true)
}

func runChainPersist(args []string) error {
	fs := flag.NewFlagSet("chain persist", flag.ContinueOnError)
	inputPath := fs.String("input", "", "confirmed persistence request under .code-harness/runs/<runId>/requests/*.json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *inputPath == "" {
		return errors.New("chain persist requires --input")
	}
	pathRunID, cleanInput, err := validateChainRequestPath(*inputPath)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(cleanInput)
	if err != nil {
		return err
	}
	var req chainPersistRequest
	if err := decodeStrictChainRequest(data, &req, "chain persist request"); err != nil {
		return fmt.Errorf("chain persist request accepts only runId and planId: %w", err)
	}
	if req.RunID != pathRunID || !validChainArtifactID(req.RunID) {
		return fmt.Errorf("RUN_ID_PATH_MISMATCH: body runId %q does not match request path runId %q", req.RunID, pathRunID)
	}
	if strings.TrimSpace(req.PlanID) == "" {
		return errors.New("chain persist request requires planId")
	}
	if err := chain.PersistWritePlan(".", req.RunID, req.PlanID); err != nil {
		return err
	}
	return writeJSONAndStatus(map[string]any{"status": "PERSISTED", "planId": req.PlanID}, true)
}

func findProjectChain(root, target string) (chain.Chain, error) {
	if chain.ValidateID(target) == nil {
		path, _ := chain.ChainPath(root, target)
		if c, err := chain.Load(path); err == nil {
			return c, nil
		}
	}
	dir := filepath.Join(filepath.Clean(root), ".code-harness", "chains")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return chain.Chain{}, err
	}
	var matches []chain.Chain
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".yaml") {
			continue
		}
		c, err := chain.Load(filepath.Join(dir, entry.Name()))
		if err != nil {
			return chain.Chain{}, err
		}
		matched := c.Name == target
		for _, ep := range c.EntryPoints {
			if ep.Symbol == target || exactSymbolOwner(ep.Symbol) == target {
				matched = true
			}
		}
		if matched {
			matches = append(matches, c)
		}
	}
	if len(matches) == 0 {
		return chain.Chain{}, fmt.Errorf("CHAIN_NOT_FOUND: %s", target)
	}
	if len(matches) > 1 {
		return chain.Chain{}, fmt.Errorf("AMBIGUOUS_CHAIN_TARGET: %s", target)
	}
	return matches[0], nil
}

func exactSymbolOwner(symbol string) string {
	symbol = strings.TrimSpace(symbol)
	idx := strings.LastIndex(symbol, ".")
	if idx <= 0 {
		return ""
	}
	return symbol[:idx]
}

func loadCertifiedChainAnalysis(value string) (chain.ChangeAnalysisEvidence, analysisruntime.Certificate, error) {
	certified, cert, err := loadCertifiedAnalysis153(".", value)
	if err != nil {
		return chain.ChangeAnalysisEvidence{}, analysisruntime.Certificate{}, fmt.Errorf("load certified ChangeAnalysis: %w", err)
	}
	analysisBytes, err := json.Marshal(certified)
	if err != nil {
		return chain.ChangeAnalysisEvidence{}, analysisruntime.Certificate{}, fmt.Errorf("encode certified ChangeAnalysis for chain: %w", err)
	}
	var evidence chain.ChangeAnalysisEvidence
	if err := json.Unmarshal(analysisBytes, &evidence); err != nil {
		return chain.ChangeAnalysisEvidence{}, analysisruntime.Certificate{}, fmt.Errorf("decode certified chain ChangeAnalysis: %w", err)
	}
	return evidence, cert, nil
}

func loadVerifiedChainAnalysis(value string) (chain.ChangeAnalysisEvidence, string, error) {
	evidence, cert, err := loadCertifiedChainAnalysis(value)
	if err != nil {
		return chain.ChangeAnalysisEvidence{}, "", err
	}
	return evidence, cert.RunID, nil
}

func decodeStrictChainRequest(data []byte, out any, label string) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", label, err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("decode %s: multiple JSON values are not allowed", label)
		}
		return fmt.Errorf("decode %s: %w", label, err)
	}
	return nil
}

func decodeChainDiscoveryRequest(data []byte) (chainDiscoverRequest, error) {
	var req chainDiscoverRequest
	if err := decodeStrictChainRequest(data, &req, "chain discovery request"); err != nil {
		return chainDiscoverRequest{}, err
	}
	if strings.TrimSpace(req.RunID) == "" || strings.TrimSpace(req.ChangeAnalysisPath) == "" {
		return chainDiscoverRequest{}, errors.New("chain discovery request requires runId and changeAnalysisPath")
	}
	return req, nil
}

func validateChainDiscoveryRequestPath(inputPath string) (string, string, error) {
	return validateChainRequestPath(inputPath)
}

func validateChainRequestPath(inputPath string) (string, string, error) {
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
	return errors.New("chain command input must be under .code-harness/runs/<runId>/requests/*.json")
}

func sameRunChangeAnalysisPath(value, runID string) bool {
	actualRun, ok := chainAnalysisRunID(value)
	return ok && actualRun == runID
}

func chainAnalysisRunID(value string) (string, bool) {
	if filepath.IsAbs(value) {
		return "", false
	}
	parts := strings.Split(filepath.ToSlash(filepath.Clean(value)), "/")
	if len(parts) != 5 || parts[0] != ".code-harness" || parts[1] != "runs" || parts[3] != "analysis" || parts[4] != "change-analysis.json" || !validChainArtifactID(parts[2]) {
		return "", false
	}
	return parts[2], true
}

func sameRunChainAnalysisArtifact(value, runID, kind string) bool {
	if filepath.IsAbs(value) || !validChainArtifactID(runID) {
		return false
	}
	parts := strings.Split(filepath.ToSlash(filepath.Clean(value)), "/")
	return len(parts) == 6 && parts[0] == ".code-harness" && parts[1] == "runs" && parts[2] == runID && parts[3] == "analysis" && parts[4] == kind && strings.EqualFold(filepath.Ext(parts[5]), ".yaml")
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
