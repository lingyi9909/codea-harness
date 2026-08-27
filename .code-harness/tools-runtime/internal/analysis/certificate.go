package analysis

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"codea-harness-tools/internal/coverage"
	"codea-harness-tools/internal/schema"
)

type Certificate struct {
	RunID                     string `json:"runId"`
	RuntimeVersion            string `json:"runtimeVersion"`
	AnalysisSHA256            string `json:"analysisSha256"`
	ChangeSetSHA256           string `json:"changeSetSha256"`
	EntrypointInventorySHA256 string `json:"entrypointInventorySha256"`
	BaseRef                   string `json:"baseRef"`
	Head                      string  `json:"head"`
	Intent                    *Intent `json:"intent,omitempty"`
}

func LoadCertified(root, analysisPath string) (ChangeAnalysis, Certificate, error) {
	return loadCertifiedWithRuntime153(root, analysisPath, defaultCertificationRuntime153{})
}

func loadCertifiedWithRuntime153(root, analysisPath string, runtime certificationRuntime153) (ChangeAnalysis, Certificate, error) {
	root = filepath.Clean(root)
	runID, cleanAnalysis, ok := certifiedAnalysisPath153(analysisPath)
	if !ok {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("CERTIFIED_ANALYSIS_PATH_INVALID: %q", analysisPath)
	}
	analysisAbs := filepath.Join(root, filepath.FromSlash(cleanAnalysis))
	analysisDir := filepath.Dir(analysisAbs)
	inventoryAbs := filepath.Join(analysisDir, "entrypoint-inventory.json")
	certAbs := filepath.Join(analysisDir, "change-analysis.cert.json")

	certBytes, err := os.ReadFile(certAbs)
	if err != nil {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("CERTIFICATE_READ_FAILED: %w", err)
	}
	certSchema, err := os.ReadFile(filepath.Join(root, ".code-harness", "contracts", "change-analysis-cert.schema.json"))
	if err != nil {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("ANALYSIS_CERT_SCHEMA_READ_FAILED: %w", err)
	}
	if err := schema.ValidateJSON(certSchema, certBytes); err != nil {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("CERTIFICATE_SCHEMA_INVALID: %w", err)
	}
	var cert Certificate
	if err := json.Unmarshal(certBytes, &cert); err != nil {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("CERTIFICATE_DECODE_FAILED: %w", err)
	}
	canonicalCert, err := json.MarshalIndent(cert, "", "  ")
	if err != nil {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("CERTIFICATE_CANONICALIZE_FAILED: %w", err)
	}
	canonicalCert = append(canonicalCert, '\n')
	if !bytes.Equal(certBytes, canonicalCert) {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("CERTIFICATE_BYTES_NOT_CANONICAL")
	}
	if cert.RunID != runID {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("CERTIFICATE_IDENTITY_MISMATCH: cert runId=%q path runId=%q", cert.RunID, runID)
	}

	analysisBytes, err := os.ReadFile(analysisAbs)
	if err != nil {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("CERTIFIED_ANALYSIS_READ_FAILED: %w", err)
	}
	if hashBytes153(analysisBytes) != cert.AnalysisSHA256 {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("CHANGED_ANALYSIS_HASH_MISMATCH")
	}
	inventoryBytes, err := os.ReadFile(inventoryAbs)
	if err != nil {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("ENTRYPOINT_INVENTORY_READ_FAILED: %w", err)
	}
	if hashBytes153(inventoryBytes) != cert.EntrypointInventorySHA256 {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("ENTRYPOINT_INVENTORY_HASH_MISMATCH")
	}

	analysisSchema, err := os.ReadFile(filepath.Join(root, ".code-harness", "contracts", "change-analysis.schema.json"))
	if err != nil {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("ANALYSIS_SCHEMA_READ_FAILED: %w", err)
	}
	if err := schema.ValidateJSON(analysisSchema, analysisBytes); err != nil {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("CERTIFIED_ANALYSIS_SCHEMA_INVALID: %w", err)
	}
	inventorySchema, err := os.ReadFile(filepath.Join(root, ".code-harness", "contracts", "entrypoint-inventory.schema.json"))
	if err != nil {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("ENTRYPOINT_INVENTORY_SCHEMA_READ_FAILED: %w", err)
	}
	if err := schema.ValidateJSON(inventorySchema, inventoryBytes); err != nil {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("ENTRYPOINT_INVENTORY_SCHEMA_INVALID: %w", err)
	}

	var meta certifyDraftMeta153
	if err := json.Unmarshal(analysisBytes, &meta); err != nil {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("CERTIFIED_ANALYSIS_DECODE_FAILED: %w", err)
	}
	var typed ChangeAnalysis
	if err := json.Unmarshal(analysisBytes, &typed); err != nil {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("CERTIFIED_ANALYSIS_DECODE_FAILED: %w", err)
	}
	var inventory EntrypointInventory
	if err := json.Unmarshal(inventoryBytes, &inventory); err != nil {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("ENTRYPOINT_INVENTORY_DECODE_FAILED: %w", err)
	}

	if cert.BaseRef != meta.ReviewScope.BaseRef || cert.Head != meta.ReviewScope.HeadCommit {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("CERTIFICATE_IDENTITY_MISMATCH: reviewScope identity differs from certificate")
	}
	if inventory.RunID != runID || inventory.ChangeSetSHA256 != cert.ChangeSetSHA256 {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("CERTIFICATE_IDENTITY_MISMATCH: inventory identity differs from certificate")
	}
	if !certificationIntentsEqual153(cert.Intent, inventory.Intent) {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("CERTIFICATE_INTENT_AUTHORITY_MISMATCH")
	}
	if err := verifyChainMaintenanceAuthority153(root, cert); err != nil {
		return ChangeAnalysis{}, Certificate{}, err
	}

	snapshot, err := runtime.Compute(root, cert.BaseRef, meta.ReviewScope.IncludeWorkingTree)
	if err != nil {
		return ChangeAnalysis{}, Certificate{}, err
	}
	if snapshot.SHA256 != cert.ChangeSetSHA256 || snapshot.Head != cert.Head || snapshot.BaseRef != cert.BaseRef {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("CERTIFIED_CHANGE_SET_STALE")
	}
	if err := compareDraftChangeSet153(meta.ChangedFiles, snapshot.Files); err != nil {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("CERTIFIED_CHANGE_SET_STALE: %w", err)
	}
	if err := VerifyEntrypointDispositions(inventory, typed); err != nil {
		return ChangeAnalysis{}, Certificate{}, err
	}
	if err := validateEvidenceAtRoot153(root, typed, inventory); err != nil {
		return ChangeAnalysis{}, Certificate{}, err
	}
	if _, err := coverage.VerifyAnalysisJSON(analysisBytes); err != nil {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("CERTIFIED_ANALYSIS_COVERAGE_INVALID: %w", err)
	}

	versionBytes, err := os.ReadFile(filepath.Join(root, ".code-harness", "VERSION"))
	if err != nil {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("CERTIFIED_RUNTIME_VERSION_UNAVAILABLE: %w", err)
	}
	currentVersion := strings.TrimSpace(string(versionBytes))
	if currentVersion == "" {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("CERTIFIED_RUNTIME_VERSION_UNAVAILABLE: VERSION is empty")
	}
	if currentVersion != cert.RuntimeVersion {
		return ChangeAnalysis{}, Certificate{}, fmt.Errorf("CERTIFIED_RUNTIME_VERSION_MISMATCH: cert=%s current=%s", cert.RuntimeVersion, currentVersion)
	}
	return typed, cert, nil
}

func certificationIntentsEqual153(certIntent, inventoryIntent *Intent) bool {
	if certIntent == nil || inventoryIntent == nil {
		return certIntent == nil && inventoryIntent == nil
	}
	return certIntent.Mode == inventoryIntent.Mode && certIntent.Target == inventoryIntent.Target
}

func certifiedAnalysisPath153(value string) (string, string, bool) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" || path.IsAbs(value) {
		return "", "", false
	}
	clean := path.Clean(value)
	parts := strings.Split(clean, "/")
	if len(parts) != 5 || parts[0] != ".code-harness" || parts[1] != "runs" || !artifactID153.MatchString(parts[2]) || parts[3] != "analysis" || parts[4] != "change-analysis.json" {
		return "", "", false
	}
	return parts[2], clean, true
}
