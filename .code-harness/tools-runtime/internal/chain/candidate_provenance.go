package chain

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/schema"
)

type CandidateCertificate struct {
	RunID         string `json:"runId"`
	Kind          string `json:"kind"`
	ChainID       string `json:"chainId"`
	CandidatePath string `json:"candidatePath"`
	CandidateHash string `json:"candidateHash"`
	AnalysisHash  string `json:"analysisHash"`
}

func CertifyCandidate(root string, c Chain, candidatePath, kind string, cert analysisruntime.Certificate) (CandidateCertificate, error) {
	root = filepath.Clean(root)
	runID, normalizedPath, pathKind, chainID, err := parseRuntimeCandidatePath153(candidatePath)
	if err != nil {
		return CandidateCertificate{}, err
	}
	kind = strings.ToUpper(strings.TrimSpace(kind))
	if kind != pathKind {
		return CandidateCertificate{}, fmt.Errorf("CHAIN_CANDIDATE_KIND_MISMATCH: path=%s requested=%s", pathKind, kind)
	}
	if cert.RunID != runID || strings.TrimSpace(cert.AnalysisSHA256) == "" {
		return CandidateCertificate{}, fmt.Errorf("CHAIN_CANDIDATE_ANALYSIS_IDENTITY_MISMATCH")
	}
	if c.ID != chainID {
		return CandidateCertificate{}, fmt.Errorf("CHAIN_CANDIDATE_ID_MISMATCH: path=%s chain=%s", chainID, c.ID)
	}
	candidateAbs := filepath.Join(root, filepath.FromSlash(normalizedPath))
	candidateBytes, err := os.ReadFile(candidateAbs)
	if err != nil {
		return CandidateCertificate{}, fmt.Errorf("CHAIN_CANDIDATE_READ_FAILED: %w", err)
	}
	loaded, err := Load(candidateAbs)
	if err != nil {
		return CandidateCertificate{}, fmt.Errorf("CHAIN_CANDIDATE_INVALID: %w", err)
	}
	if loaded.ID != c.ID {
		return CandidateCertificate{}, fmt.Errorf("CHAIN_CANDIDATE_ID_MISMATCH: bytes=%s chain=%s", loaded.ID, c.ID)
	}
	if err := validateModel(loaded); err != nil {
		return CandidateCertificate{}, fmt.Errorf("CHAIN_CANDIDATE_INVALID: %w", err)
	}

	out := CandidateCertificate{
		RunID: runID,
		Kind: kind,
		ChainID: chainID,
		CandidatePath: normalizedPath,
		CandidateHash: hashChainBytes153(candidateBytes),
		AnalysisHash: cert.AnalysisSHA256,
	}
	certBytes, err := json.MarshalIndent(out, "", "  ")
	if err != nil { return CandidateCertificate{}, err }
	certBytes = append(certBytes, '\n')
	if err := validateChainAuthorityJSON153(root, "chain-candidate-cert.schema.json", certBytes); err != nil {
		return CandidateCertificate{}, err
	}
	if err := atomicReplace(candidateCertPath153(root, normalizedPath), certBytes); err != nil {
		return CandidateCertificate{}, fmt.Errorf("CHAIN_CANDIDATE_CERT_WRITE_FAILED: %w", err)
	}
	return out, nil
}

func LoadRuntimeCandidate(root string, candidatePath string, cert analysisruntime.Certificate) (Chain, CandidateCertificate, error) {
	candidate, candidateCert, err := loadRuntimeCandidateProvenance153(root, candidatePath)
	if err != nil {
		return Chain{}, CandidateCertificate{}, err
	}
	if cert.RunID != candidateCert.RunID || strings.TrimSpace(cert.AnalysisSHA256) == "" || candidateCert.AnalysisHash != cert.AnalysisSHA256 {
		return Chain{}, CandidateCertificate{}, fmt.Errorf("CHAIN_CANDIDATE_ANALYSIS_IDENTITY_MISMATCH")
	}
	return candidate, candidateCert, nil
}

func loadRuntimeCandidateProvenance153(root string, candidatePath string) (Chain, CandidateCertificate, error) {
	root = filepath.Clean(root)
	runID, normalizedPath, pathKind, chainID, err := parseRuntimeCandidatePath153(candidatePath)
	if err != nil {
		return Chain{}, CandidateCertificate{}, err
	}
	certBytes, err := os.ReadFile(candidateCertPath153(root, normalizedPath))
	if err != nil {
		if os.IsNotExist(err) {
			return Chain{}, CandidateCertificate{}, fmt.Errorf("CHAIN_ARTIFACT_NOT_RUNTIME_OWNED: %s", normalizedPath)
		}
		return Chain{}, CandidateCertificate{}, fmt.Errorf("CHAIN_CANDIDATE_CERT_READ_FAILED: %w", err)
	}
	if err := validateChainAuthorityJSON153(root, "chain-candidate-cert.schema.json", certBytes); err != nil {
		return Chain{}, CandidateCertificate{}, err
	}
	var candidateCert CandidateCertificate
	if err := json.Unmarshal(certBytes, &candidateCert); err != nil {
		return Chain{}, CandidateCertificate{}, fmt.Errorf("CHAIN_CANDIDATE_CERT_DECODE_FAILED: %w", err)
	}
	canonical, err := json.MarshalIndent(candidateCert, "", "  ")
	if err != nil { return Chain{}, CandidateCertificate{}, err }
	canonical = append(canonical, '\n')
	if !bytes.Equal(certBytes, canonical) {
		return Chain{}, CandidateCertificate{}, fmt.Errorf("CHAIN_CANDIDATE_CERT_BYTES_NOT_CANONICAL")
	}
	if candidateCert.RunID != runID || candidateCert.Kind != pathKind || candidateCert.ChainID != chainID || candidateCert.CandidatePath != normalizedPath || strings.TrimSpace(candidateCert.AnalysisHash) == "" {
		return Chain{}, CandidateCertificate{}, fmt.Errorf("CHAIN_CANDIDATE_CERT_IDENTITY_MISMATCH")
	}
	candidateAbs := filepath.Join(root, filepath.FromSlash(normalizedPath))
	candidateBytes, err := os.ReadFile(candidateAbs)
	if err != nil {
		return Chain{}, CandidateCertificate{}, fmt.Errorf("CHAIN_CANDIDATE_READ_FAILED: %w", err)
	}
	if hashChainBytes153(candidateBytes) != candidateCert.CandidateHash {
		return Chain{}, CandidateCertificate{}, fmt.Errorf("CHAIN_CANDIDATE_HASH_MISMATCH: %s", chainID)
	}
	candidate, err := Load(candidateAbs)
	if err != nil {
		return Chain{}, CandidateCertificate{}, fmt.Errorf("CHAIN_CANDIDATE_INVALID: %w", err)
	}
	if candidate.ID != chainID {
		return Chain{}, CandidateCertificate{}, fmt.Errorf("CHAIN_CANDIDATE_ID_MISMATCH: path=%s bytes=%s", chainID, candidate.ID)
	}
	return candidate, candidateCert, nil
}

func parseRuntimeCandidatePath153(value string) (runID, normalizedPath, kind, chainID string, err error) {
	if filepath.IsAbs(value) {
		return "", "", "", "", fmt.Errorf("CHAIN_CANDIDATE_PATH_INVALID: %q", value)
	}
	normalizedPath = filepath.ToSlash(filepath.Clean(value))
	parts := strings.Split(normalizedPath, "/")
	if len(parts) != 6 || parts[0] != ".code-harness" || parts[1] != "runs" || parts[3] != "analysis" || !strings.EqualFold(filepath.Ext(parts[5]), ".yaml") {
		return "", "", "", "", fmt.Errorf("CHAIN_CANDIDATE_PATH_INVALID: %q", value)
	}
	runID = parts[2]
	if !runIDPattern.MatchString(runID) {
		return "", "", "", "", fmt.Errorf("CHAIN_CANDIDATE_PATH_INVALID: invalid runId")
	}
	switch parts[4] {
	case "discovered-chains":
		kind = "DISCOVERED"
	case "refresh-candidates":
		kind = "REFRESH"
	case "edit-candidates":
		kind = "EDIT"
	default:
		return "", "", "", "", fmt.Errorf("CHAIN_CANDIDATE_PATH_INVALID: unsupported Runtime candidate directory")
	}
	chainID = strings.TrimSuffix(parts[5], filepath.Ext(parts[5]))
	if err := ValidateID(chainID); err != nil {
		return "", "", "", "", fmt.Errorf("CHAIN_CANDIDATE_PATH_INVALID: %w", err)
	}
	return runID, normalizedPath, kind, chainID, nil
}

func candidateCertPath153(root, candidatePath string) string {
	return filepath.Join(filepath.Clean(root), filepath.FromSlash(strings.TrimSuffix(candidatePath, filepath.Ext(candidatePath))+".cert.json"))
}

func hashChainBytes153(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func validateChainAuthorityJSON153(root, schemaName string, data []byte) error {
	schemaBytes, err := os.ReadFile(filepath.Join(filepath.Clean(root), ".code-harness", "contracts", schemaName))
	if err != nil {
		return fmt.Errorf("CHAIN_AUTHORITY_SCHEMA_READ_FAILED: %s: %w", schemaName, err)
	}
	if err := schema.ValidateJSON(schemaBytes, data); err != nil {
		return fmt.Errorf("CHAIN_AUTHORITY_SCHEMA_INVALID: %s: %w", schemaName, err)
	}
	return nil
}
