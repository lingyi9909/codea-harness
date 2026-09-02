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
	"regexp"
	"strings"

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/changeset"
	"codea-harness-tools/internal/requestcontract"
	"codea-harness-tools/internal/schema"
)

var analysisArtifactID153 = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type analysisSnapshotRequest162 struct {
	RunID              string `json:"runId"`
	BaseRef            string `json:"baseRef"`
	IncludeWorkingTree bool   `json:"includeWorkingTree"`
}

type analysisInventoryRequest153 struct {
	RunID              string                 `json:"runId"`
	BaseRef            string                 `json:"baseRef"`
	IncludeWorkingTree bool                   `json:"includeWorkingTree"`
	Intent             analysisruntime.Intent `json:"intent"`
}

func runAnalysis(args []string) error {
	if len(args) == 0 { return errors.New("analysis requires snapshot, inventory or certify") }
	switch args[0] {
	case "snapshot":
		return runAnalysisSnapshot162(args[1:])
	case "inventory":
		return runAnalysisInventory(args[1:])
	case "certify":
		return runAnalysisCertify(args[1:])
	default:
		return fmt.Errorf("unknown analysis action %q", args[0])
	}
}

func runAnalysisSnapshot162(args []string) error {
	fs := flag.NewFlagSet("analysis snapshot", flag.ContinueOnError)
	inputPath := fs.String("input", "", "canonical ChangeSet request under .code-harness/runs/<runId>/requests/*.json")
	if err := fs.Parse(args); err != nil { return err }
	if fs.NArg() != 0 || strings.TrimSpace(*inputPath) == "" { return errors.New("analysis snapshot requires --input") }

	pathRunID, cleanInput, err := validateAnalysisRequestPath153(*inputPath)
	if err != nil { return err }
	data, err := os.ReadFile(cleanInput)
	if err != nil { return fmt.Errorf("read ChangeSet snapshot request: %w", err) }
	if err := requestcontract.Validate("change-set-request.schema.json", data); err != nil {
		return fmt.Errorf("CHANGE_SET_REQUEST_SCHEMA_INVALID: %w", err)
	}
	var req analysisSnapshotRequest162
	if err := decodeStrictAnalysisRequest153(data, &req, "ChangeSet snapshot request"); err != nil { return err }
	if req.RunID != pathRunID {
		return fmt.Errorf("RUN_ID_PATH_MISMATCH: body runId %q does not match request path runId %q", req.RunID, pathRunID)
	}
	if !analysisArtifactID153.MatchString(req.RunID) { return errors.New("ChangeSet snapshot request contains invalid runId") }
	if strings.TrimSpace(req.BaseRef) == "" { return errors.New("ChangeSet snapshot request requires baseRef") }

	snapshot, err := changeset.Compute(".", req.BaseRef, req.IncludeWorkingTree)
	if err != nil { return err }
	artifactBytes, err := changeset.CanonicalBytes(snapshot)
	if err != nil { return err }
	schemaBytes, err := os.ReadFile(filepath.Join(".code-harness", "contracts", "change-set.schema.json"))
	if err != nil { return fmt.Errorf("read ChangeSet snapshot schema: %w", err) }
	if err := schema.ValidateJSON(schemaBytes, artifactBytes); err != nil { return fmt.Errorf("validate ChangeSet snapshot: %w", err) }

	artifactPath := filepath.Join(".code-harness", "runs", req.RunID, "analysis", "change-set.json")
	if err := atomicWriteAnalysis153(artifactPath, artifactBytes); err != nil { return err }
	return writeJSONAndStatus(map[string]any{
		"status": "SNAPSHOT_READY",
		"runId": req.RunID,
		"artifactPath": filepath.ToSlash(artifactPath),
		"snapshotSha256": snapshot.SnapshotSHA256,
		"resolvedBaseCommit": snapshot.ResolvedBaseCommit,
		"mergeBase": snapshot.MergeBase,
		"headCommit": snapshot.HeadCommit,
	}, true)
}

func runAnalysisInventory(args []string) error {
	fs := flag.NewFlagSet("analysis inventory", flag.ContinueOnError)
	inputPath := fs.String("input", "", "entrypoint inventory request under .code-harness/runs/<runId>/requests/*.json")
	if err := fs.Parse(args); err != nil { return err }
	if fs.NArg() != 0 || strings.TrimSpace(*inputPath) == "" { return errors.New("analysis inventory requires --input") }

	pathRunID, cleanInput, err := validateAnalysisRequestPath153(*inputPath)
	if err != nil { return err }
	data, err := os.ReadFile(cleanInput)
	if err != nil { return fmt.Errorf("read entrypoint inventory request: %w", err) }
	if err := requestcontract.Validate("analysis-inventory-request.schema.json", data); err != nil {
		return fmt.Errorf("ANALYSIS_INVENTORY_REQUEST_SCHEMA_INVALID: %w", err)
	}
	var req analysisInventoryRequest153
	if err := decodeStrictAnalysisRequest153(data, &req, "entrypoint inventory request"); err != nil { return err }
	if req.RunID != pathRunID {
		return fmt.Errorf("RUN_ID_PATH_MISMATCH: body runId %q does not match request path runId %q", req.RunID, pathRunID)
	}
	if !analysisArtifactID153.MatchString(req.RunID) { return errors.New("analysis inventory request contains invalid runId") }
	if strings.TrimSpace(req.BaseRef) == "" { return errors.New("analysis inventory request requires baseRef") }
	if strings.TrimSpace(req.Intent.Mode) == "" { return errors.New("analysis inventory request requires intent.mode") }

	snapshot, err := changeset.Compute(".", req.BaseRef, req.IncludeWorkingTree)
	if err != nil { return err }
	inventory, err := analysisruntime.BuildEntrypointInventory(".", req.RunID, snapshot, req.Intent)
	if err != nil { return err }
	artifactBytes, err := json.MarshalIndent(inventory, "", "  ")
	if err != nil { return fmt.Errorf("encode entrypoint inventory: %w", err) }
	artifactBytes = append(artifactBytes, '\n')

	schemaBytes, err := os.ReadFile(filepath.Join(".code-harness", "contracts", "entrypoint-inventory.schema.json"))
	if err != nil { return fmt.Errorf("read entrypoint inventory schema: %w", err) }
	if err := schema.ValidateJSON(schemaBytes, artifactBytes); err != nil { return fmt.Errorf("validate entrypoint inventory: %w", err) }

	artifactPath := filepath.Join(".code-harness", "runs", req.RunID, "analysis", "entrypoint-inventory.json")
	if err := atomicWriteAnalysis153(artifactPath, artifactBytes); err != nil { return err }
	return writeJSONAndStatus(map[string]any{
		"status": "COMPLETE",
		"runId": req.RunID,
		"artifactPath": filepath.ToSlash(artifactPath),
		"expectedEntryPoints": len(inventory.ExpectedEntrypoints),
		"changeSetSha256": inventory.ChangeSetSHA256,
	}, true)
}

func runAnalysisCertify(args []string) error {
	fs := flag.NewFlagSet("analysis certify", flag.ContinueOnError)
	inputPath := fs.String("input", "", "certification request under .code-harness/runs/<runId>/requests/*.json")
	if err := fs.Parse(args); err != nil { return err }
	if fs.NArg() != 0 || strings.TrimSpace(*inputPath) == "" { return errors.New("analysis certify requires --input") }

	pathRunID, cleanInput, err := validateAnalysisRequestPath153(*inputPath)
	if err != nil { return err }
	data, err := os.ReadFile(cleanInput)
	if err != nil { return fmt.Errorf("read analysis certify request: %w", err) }
	if err := requestcontract.Validate("analysis-certify-request.schema.json", data); err != nil {
		return fmt.Errorf("ANALYSIS_CERTIFY_REQUEST_SCHEMA_INVALID: %w", err)
	}
	var req analysisruntime.CertifyRequest
	if err := decodeStrictAnalysisRequest153(data, &req, "analysis certify request"); err != nil { return err }
	if req.RunID != pathRunID {
		return fmt.Errorf("RUN_ID_PATH_MISMATCH: body runId %q does not match request path runId %q", req.RunID, pathRunID)
	}
	if !analysisArtifactID153.MatchString(req.RunID) {
		return errors.New("analysis certify request contains invalid runId")
	}
	cert, err := analysisruntime.Certify(".", req)
	if err != nil { return err }
	analysisPath := filepath.Join(".code-harness", "runs", req.RunID, "analysis", "change-analysis.json")
	certPath := filepath.Join(".code-harness", "runs", req.RunID, "analysis", "change-analysis.cert.json")
	return writeJSONAndStatus(map[string]any{
		"status": "CERTIFIED",
		"runId": req.RunID,
		"artifactPath": filepath.ToSlash(analysisPath),
		"certificatePath": filepath.ToSlash(certPath),
		"analysisSha256": cert.AnalysisSHA256,
		"changeSetSha256": cert.ChangeSetSHA256,
		"entrypointInventorySha256": cert.EntrypointInventorySHA256,
	}, true)
}

func validateAnalysisRequestPath153(input string) (string, string, error) {
	if filepath.IsAbs(input) { return "", "", errors.New("analysis request path must be repository-relative") }
	clean := filepath.Clean(input)
	parts := strings.Split(filepath.ToSlash(clean), "/")
	if len(parts) != 5 || parts[0] != ".code-harness" || parts[1] != "runs" || parts[3] != "requests" || parts[4] == "" {
		return "", "", errors.New("analysis request must be .code-harness/runs/<runId>/requests/*.json")
	}
	if !analysisArtifactID153.MatchString(parts[2]) || !strings.EqualFold(filepath.Ext(parts[4]), ".json") {
		return "", "", errors.New("analysis request path contains invalid runId or file name")
	}
	return parts[2], clean, nil
}

func decodeStrictAnalysisRequest153(data []byte, out any, label string) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil { return fmt.Errorf("decode %s: %w", label, err) }
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil { return fmt.Errorf("decode %s: multiple JSON values are not allowed", label) }
		return fmt.Errorf("decode %s: %w", label, err)
	}
	return nil
}

func atomicWriteAnalysis153(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { return fmt.Errorf("create analysis directory: %w", err) }
	tmp, err := os.CreateTemp(filepath.Dir(path), ".analysis-artifact-*.tmp")
	if err != nil { return fmt.Errorf("create analysis artifact temp file: %w", err) }
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil { _ = tmp.Close(); return fmt.Errorf("write analysis artifact temp file: %w", err) }
	if err := tmp.Chmod(0o644); err != nil { _ = tmp.Close(); return err }
	if err := tmp.Close(); err != nil { return err }
	if err := os.Rename(tmpName, path); err != nil { return fmt.Errorf("publish analysis artifact: %w", err) }
	return nil
}
