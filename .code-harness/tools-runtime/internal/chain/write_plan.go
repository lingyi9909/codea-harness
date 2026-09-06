package chain

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	analysisruntime "codea-harness-tools/internal/analysis"
)

type WritePlan struct {
	PlanID               string `json:"planId"`
	RunID                string `json:"runId"`
	ChainID              string `json:"chainId"`
	CandidatePath        string `json:"candidatePath"`
	CandidateHash        string `json:"candidateHash"`
	AnalysisHash         string `json:"analysisHash"`
	AuthorityKind        string `json:"authorityKind,omitempty"`
	SourceHash           string `json:"sourceHash,omitempty"`
	ExpectedExistingHash string `json:"expectedExistingHash,omitempty"`
	PreviewSHA256        string `json:"previewSha256"`
}

var chainWritePlanID153 = regexp.MustCompile(`^chain-write-[0-9a-f]{64}$`)

func SealWritePlan(root string, runID, candidatePath, expectedExistingHash string) (WritePlan, error) {
	root = filepath.Clean(root)
	if !runIDPattern.MatchString(runID) {
		return WritePlan{}, fmt.Errorf("CHAIN_WRITE_PLAN_RUN_ID_INVALID")
	}
	pathRunID, normalizedCandidatePath, _, _, err := parseRuntimeCandidatePath153(candidatePath)
	if err != nil {
		return WritePlan{}, err
	}
	if pathRunID != runID {
		return WritePlan{}, fmt.Errorf("CHAIN_WRITE_PLAN_RUN_ID_MISMATCH")
	}

	// Exact Runtime provenance is always checked first. The certificate then
	// selects the persistence authority: legacy/AFFECTED uses Certified
	// ChangeAnalysis; PROJECT uses the Runtime-owned PROJECT_SOURCE snapshot.
	candidate, candidateCert, err := loadRuntimeCandidateProvenance153(root, normalizedCandidatePath)
	if err != nil {
		return WritePlan{}, err
	}

	authorityHash := candidateCert.AnalysisHash
	authorityKind := candidateCert.AuthorityKind
	sourceHash := ""
	switch authorityKind {
	case "":
		analysisPath := filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json"))
		certified, analysisCert, err := analysisruntime.LoadCertified(root, analysisPath)
		if err != nil {
			return WritePlan{}, fmt.Errorf("CHAIN_WRITE_PLAN_ANALYSIS_NOT_CERTIFIED: %w", err)
		}
		if candidateCert.RunID != analysisCert.RunID || candidateCert.AnalysisHash != analysisCert.AnalysisSHA256 {
			return WritePlan{}, fmt.Errorf("CHAIN_CANDIDATE_ANALYSIS_IDENTITY_MISMATCH")
		}
		evidence, err := analysisEvidence153(certified)
		if err != nil {
			return WritePlan{}, err
		}
		validation := Validate(root, candidate, EvidenceSnapshot(evidence))
		if validation.Status != ValidationValid {
			return WritePlan{}, fmt.Errorf("CHAIN_CANDIDATE_VALIDATION_FAILED: %s", strings.Join(validation.Errors, "; "))
		}
		authorityHash = analysisCert.AnalysisSHA256
	case projectSourceAuthority163:
		projectCandidate, projectCert, err := LoadProjectRuntimeCandidate(root, normalizedCandidatePath)
		if err != nil {
			return WritePlan{}, err
		}
		if projectCert.RunID != runID || projectCert.SourceHash == "" || projectCert.SourceHash != projectCert.AnalysisHash {
			return WritePlan{}, fmt.Errorf("CHAIN_CANDIDATE_PROJECT_SOURCE_IDENTITY_MISMATCH")
		}
		candidate = projectCandidate
		candidateCert = projectCert
		authorityHash = projectCert.SourceHash
		sourceHash = projectCert.SourceHash
	default:
		return WritePlan{}, fmt.Errorf("CHAIN_WRITE_PLAN_AUTHORITY_INVALID: %s", authorityKind)
	}

	existingHash, exists, err := currentProjectChainHash153(root, candidate.ID)
	if err != nil {
		return WritePlan{}, err
	}
	provided := strings.TrimSpace(expectedExistingHash)
	if provided != "" {
		if !exists {
			return WritePlan{}, fmt.Errorf("CHAIN_EXPECTED_HASH_WITHOUT_EXISTING_FILE: %s", candidate.ID)
		}
		if provided != existingHash {
			return WritePlan{}, fmt.Errorf("CHAIN_EXPECTED_HASH_MISMATCH: %s", candidate.ID)
		}
	}
	if !exists {
		existingHash = ""
	}

	accepted := candidate
	accepted.Status = StatusAccepted
	previewBytes, err := MarshalYAML(accepted)
	if err != nil {
		return WritePlan{}, err
	}
	plan := WritePlan{
		RunID:                runID,
		ChainID:              candidate.ID,
		CandidatePath:        normalizedCandidatePath,
		CandidateHash:        candidateCert.CandidateHash,
		AnalysisHash:         authorityHash,
		AuthorityKind:        authorityKind,
		SourceHash:           sourceHash,
		ExpectedExistingHash: existingHash,
		PreviewSHA256:        hashChainBytes153(previewBytes),
	}
	plan.PlanID = writePlanID153(plan)
	planBytes, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return WritePlan{}, err
	}
	planBytes = append(planBytes, '\n')
	if err := validateChainAuthorityJSON153(root, "chain-write-plan.schema.json", planBytes); err != nil {
		return WritePlan{}, err
	}
	planPath := chainWritePlanPath153(root, runID, plan.PlanID)
	if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
		return WritePlan{}, fmt.Errorf("CHAIN_WRITE_PLAN_DIR_FAILED: %w", err)
	}
	if existing, err := os.ReadFile(planPath); err == nil {
		if !bytes.Equal(existing, planBytes) {
			return WritePlan{}, fmt.Errorf("CHAIN_WRITE_PLAN_ID_COLLISION: %s", plan.PlanID)
		}
		return plan, nil
	} else if !os.IsNotExist(err) {
		return WritePlan{}, fmt.Errorf("CHAIN_WRITE_PLAN_READ_FAILED: %w", err)
	}
	if err := atomicReplace(planPath, planBytes); err != nil {
		return WritePlan{}, fmt.Errorf("CHAIN_WRITE_PLAN_WRITE_FAILED: %w", err)
	}
	return plan, nil
}

func PersistWritePlan(root string, runID, planID string) error {
	root = filepath.Clean(root)
	if !runIDPattern.MatchString(runID) || !chainWritePlanID153.MatchString(planID) {
		return fmt.Errorf("CHAIN_WRITE_PLAN_ID_INVALID")
	}
	planPath := chainWritePlanPath153(root, runID, planID)
	planBytes, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("CHAIN_WRITE_PLAN_READ_FAILED: %w", err)
	}
	if err := validateChainAuthorityJSON153(root, "chain-write-plan.schema.json", planBytes); err != nil {
		return err
	}
	var plan WritePlan
	if err := json.Unmarshal(planBytes, &plan); err != nil {
		return fmt.Errorf("CHAIN_WRITE_PLAN_DECODE_FAILED: %w", err)
	}
	canonical, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	canonical = append(canonical, '\n')
	if !bytes.Equal(planBytes, canonical) {
		return fmt.Errorf("CHAIN_WRITE_PLAN_BYTES_NOT_CANONICAL")
	}
	if plan.RunID != runID || plan.PlanID != planID || writePlanID153(plan) != planID {
		return fmt.Errorf("CHAIN_WRITE_PLAN_ID_MISMATCH")
	}
	pathRunID, normalizedCandidatePath, _, chainID, err := parseRuntimeCandidatePath153(plan.CandidatePath)
	if err != nil {
		return err
	}
	if pathRunID != runID || normalizedCandidatePath != plan.CandidatePath || chainID != plan.ChainID {
		return fmt.Errorf("CHAIN_WRITE_PLAN_IDENTITY_MISMATCH")
	}

	var candidate Chain
	var candidateCert CandidateCertificate
	switch plan.AuthorityKind {
	case "":
		if plan.SourceHash != "" {
			return fmt.Errorf("CHAIN_WRITE_PLAN_AUTHORITY_INVALID")
		}
		analysisPath := filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json"))
		certified, analysisCert, err := analysisruntime.LoadCertified(root, analysisPath)
		if err != nil {
			return fmt.Errorf("CHAIN_WRITE_PLAN_ANALYSIS_NOT_CERTIFIED: %w", err)
		}
		if analysisCert.AnalysisSHA256 != plan.AnalysisHash {
			return fmt.Errorf("CHAIN_WRITE_PLAN_ANALYSIS_HASH_MISMATCH")
		}
		candidate, candidateCert, err = LoadRuntimeCandidate(root, plan.CandidatePath, analysisCert)
		if err != nil {
			return err
		}
		if candidateCert.CandidateHash != plan.CandidateHash {
			return fmt.Errorf("CHAIN_CANDIDATE_HASH_MISMATCH: %s", plan.ChainID)
		}
		evidence, err := analysisEvidence153(certified)
		if err != nil {
			return err
		}
		validation := Validate(root, candidate, EvidenceSnapshot(evidence))
		if validation.Status != ValidationValid {
			return fmt.Errorf("CHAIN_CANDIDATE_VALIDATION_FAILED: %s", strings.Join(validation.Errors, "; "))
		}
	case projectSourceAuthority163:
		if plan.SourceHash == "" || plan.AnalysisHash != plan.SourceHash {
			return fmt.Errorf("CHAIN_WRITE_PLAN_PROJECT_SOURCE_IDENTITY_MISMATCH")
		}
		projectCandidate, projectCert, err := LoadProjectRuntimeCandidate(root, plan.CandidatePath)
		if err != nil {
			return err
		}
		if projectCert.RunID != runID || projectCert.SourceHash != plan.SourceHash || projectCert.AnalysisHash != plan.AnalysisHash {
			return fmt.Errorf("CHAIN_WRITE_PLAN_PROJECT_SOURCE_IDENTITY_MISMATCH")
		}
		if projectCert.CandidateHash != plan.CandidateHash {
			return fmt.Errorf("CHAIN_CANDIDATE_HASH_MISMATCH: %s", plan.ChainID)
		}
		candidate = projectCandidate
		candidateCert = projectCert
	default:
		return fmt.Errorf("CHAIN_WRITE_PLAN_AUTHORITY_INVALID: %s", plan.AuthorityKind)
	}

	accepted := candidate
	accepted.Status = StatusAccepted
	previewBytes, err := MarshalYAML(accepted)
	if err != nil {
		return err
	}
	if hashChainBytes153(previewBytes) != plan.PreviewSHA256 {
		return fmt.Errorf("CHAIN_WRITE_PLAN_PREVIEW_HASH_MISMATCH")
	}
	actualExisting, exists, err := currentProjectChainHash153(root, plan.ChainID)
	if err != nil {
		return err
	}
	if plan.ExpectedExistingHash == "" {
		if exists {
			return fmt.Errorf("CHAIN_EXPECTED_HASH_MISMATCH: %s", plan.ChainID)
		}
	} else if !exists || actualExisting != plan.ExpectedExistingHash {
		return fmt.Errorf("CHAIN_EXPECTED_HASH_MISMATCH: %s", plan.ChainID)
	}

	// Re-check PROJECT source immediately before the Project State write. This
	// keeps the sealed source identity authoritative throughout persistence.
	if plan.AuthorityKind == projectSourceAuthority163 {
		identity, err := loadProjectSourceIdentity163(root, runID)
		if err != nil {
			return err
		}
		if identity.SourceHash != plan.SourceHash {
			return fmt.Errorf("PROJECT_SOURCE_STALE")
		}
	}
	return SaveAccepted(root, accepted, plan.ExpectedExistingHash)
}

func analysisEvidence153(certified analysisruntime.ChangeAnalysis) (ChangeAnalysisEvidence, error) {
	b, err := json.Marshal(certified)
	if err != nil {
		return ChangeAnalysisEvidence{}, err
	}
	var evidence ChangeAnalysisEvidence
	if err := json.Unmarshal(b, &evidence); err != nil {
		return ChangeAnalysisEvidence{}, fmt.Errorf("CHAIN_CERTIFIED_ANALYSIS_DECODE_FAILED: %w", err)
	}
	return evidence, nil
}

func currentProjectChainHash153(root, chainID string) (string, bool, error) {
	path, err := ChainPath(root, chainID)
	if err != nil {
		return "", false, err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", false, nil
	} else if err != nil {
		return "", false, fmt.Errorf("CHAIN_PROJECT_STATE_READ_FAILED: %w", err)
	}
	hash, err := FileHash(path)
	if err != nil {
		return "", false, err
	}
	return hash, true, nil
}

func writePlanID153(plan WritePlan) string {
	identity := struct {
		RunID                string `json:"runId"`
		ChainID              string `json:"chainId"`
		CandidatePath        string `json:"candidatePath"`
		CandidateHash        string `json:"candidateHash"`
		AnalysisHash         string `json:"analysisHash"`
		AuthorityKind        string `json:"authorityKind,omitempty"`
		SourceHash           string `json:"sourceHash,omitempty"`
		ExpectedExistingHash string `json:"expectedExistingHash,omitempty"`
		PreviewSHA256        string `json:"previewSha256"`
	}{
		plan.RunID,
		plan.ChainID,
		plan.CandidatePath,
		plan.CandidateHash,
		plan.AnalysisHash,
		plan.AuthorityKind,
		plan.SourceHash,
		plan.ExpectedExistingHash,
		plan.PreviewSHA256,
	}
	b, _ := json.Marshal(identity)
	sum := sha256.Sum256(b)
	return fmt.Sprintf("chain-write-%x", sum[:])
}

func chainWritePlanPath153(root, runID, planID string) string {
	return filepath.Join(filepath.Clean(root), ".code-harness", "runs", runID, "analysis", "chain-write-plans", planID+".json")
}
