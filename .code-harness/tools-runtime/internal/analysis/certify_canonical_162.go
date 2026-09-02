package analysis

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"codea-harness-tools/internal/changeset"
	"codea-harness-tools/internal/coverage"
	"codea-harness-tools/internal/schema"
)

func isCanonicalCertifyRequest162(req CertifyRequest) bool {
	return strings.TrimSpace(req.ProposalPath) != "" || strings.TrimSpace(req.SnapshotPath) != "" || strings.TrimSpace(req.SnapshotSHA256) != ""
}

func certifyCanonical162(root string, req CertifyRequest, runtime certificationRuntime153) (Certificate, error) {
	if !artifactID153.MatchString(strings.TrimSpace(req.RunID)) || strings.TrimSpace(req.Intent.Mode) == "" {
		return Certificate{}, fmt.Errorf("ANALYSIS_CERTIFY_REQUEST_INVALID: runId and intent.mode are required")
	}
	if strings.TrimSpace(req.DraftPath) != "" || strings.TrimSpace(req.BaseRef) != "" {
		return Certificate{}, fmt.Errorf("ANALYSIS_CERTIFY_REQUEST_INVALID: canonical snapshot mode cannot include legacy draftPath/baseRef")
	}
	if !sameRunSnapshotPath162(req.SnapshotPath, req.RunID) {
		return Certificate{}, fmt.Errorf("ANALYSIS_SNAPSHOT_PATH_INVALID: must be .code-harness/runs/%s/analysis/change-set.json", req.RunID)
	}
	if !sameRunProposalPath162(req.ProposalPath, req.RunID) {
		return Certificate{}, fmt.Errorf("ANALYSIS_PROPOSAL_PATH_INVALID: must be .code-harness/runs/%s/requests/change-analysis-proposal.json", req.RunID)
	}
	if len(strings.TrimSpace(req.SnapshotSHA256)) != 64 {
		return Certificate{}, fmt.Errorf("ANALYSIS_CERTIFY_REQUEST_INVALID: snapshotSha256 is required")
	}

	snapshotBytes, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(req.SnapshotPath)))
	if err != nil {
		return Certificate{}, fmt.Errorf("CHANGE_SET_SNAPSHOT_READ_FAILED: %w", err)
	}
	if schemaBytes, readErr := os.ReadFile(filepath.Join(root, ".code-harness", "contracts", "change-set.schema.json")); readErr == nil {
		if err := schema.ValidateJSON(schemaBytes, snapshotBytes); err != nil {
			return Certificate{}, fmt.Errorf("CHANGE_SET_SNAPSHOT_SCHEMA_INVALID: %w", err)
		}
	} else if !os.IsNotExist(readErr) {
		return Certificate{}, fmt.Errorf("CHANGE_SET_SNAPSHOT_SCHEMA_READ_FAILED: %w", readErr)
	}
	snapshot, err := changeset.DecodeCanonical(snapshotBytes)
	if err != nil {
		return Certificate{}, err
	}
	if req.SnapshotSHA256 != snapshot.SnapshotSHA256 {
		return Certificate{}, fmt.Errorf("CHANGE_SET_SNAPSHOT_IDENTITY_MISMATCH: request=%s artifact=%s", req.SnapshotSHA256, snapshot.SnapshotSHA256)
	}

	live, err := runtime.Compute(root, snapshot.RequestedBaseRef, snapshot.IncludeWorkingTree)
	if err != nil {
		return Certificate{}, err
	}
	if !sameCanonicalSnapshotAuthority162(snapshot, live) {
		return Certificate{}, fmt.Errorf("CHANGE_SET_SNAPSHOT_STALE: snapshot=%s live=%s", snapshot.SnapshotSHA256, live.SnapshotSHA256)
	}

	proposalBytes, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(req.ProposalPath)))
	if err != nil {
		return Certificate{}, fmt.Errorf("ANALYSIS_PROPOSAL_READ_FAILED: %w", err)
	}
	proposalSchema, err := os.ReadFile(filepath.Join(root, ".code-harness", "contracts", "change-analysis-proposal.schema.json"))
	if err != nil {
		return Certificate{}, fmt.Errorf("ANALYSIS_PROPOSAL_SCHEMA_READ_FAILED: %w", err)
	}
	if err := schema.ValidateJSON(proposalSchema, proposalBytes); err != nil {
		return Certificate{}, fmt.Errorf("ANALYSIS_PROPOSAL_SCHEMA_INVALID: %w", err)
	}
	canonicalAnalysis, typed, err := assembleCanonicalAnalysis162(proposalBytes, live)
	if err != nil {
		return Certificate{}, err
	}
	analysisSchema, err := os.ReadFile(filepath.Join(root, ".code-harness", "contracts", "change-analysis.schema.json"))
	if err != nil {
		return Certificate{}, fmt.Errorf("ANALYSIS_SCHEMA_READ_FAILED: %w", err)
	}
	if err := schema.ValidateJSON(analysisSchema, canonicalAnalysis); err != nil {
		return Certificate{}, fmt.Errorf("ANALYSIS_ASSEMBLED_SCHEMA_INVALID: %w", err)
	}

	certifyIntent := Intent{Mode: strings.ToUpper(strings.TrimSpace(req.Intent.Mode)), Target: strings.TrimSpace(req.Intent.Target)}
	inventory, err := runtime.Inventory(root, req.RunID, live, req.Intent)
	if err != nil {
		return Certificate{}, err
	}
	if inventory.RunID != req.RunID || inventory.ChangeSetSHA256 != live.SHA256 || inventory.Status != inventoryComplete153 {
		return Certificate{}, fmt.Errorf("ENTRYPOINT_INVENTORY_IDENTITY_MISMATCH")
	}
	inventory.Intent = &Intent{Mode: certifyIntent.Mode, Target: certifyIntent.Target}
	if err := VerifyEntrypointDispositions(inventory, typed); err != nil {
		return Certificate{}, err
	}
	if err := validateEvidenceAtRoot153(root, typed, inventory); err != nil {
		return Certificate{}, err
	}
	if _, err := coverage.VerifyAnalysisJSON(canonicalAnalysis); err != nil {
		return Certificate{}, fmt.Errorf("ANALYSIS_COVERAGE_INVALID: %w", err)
	}
	return publishCertifiedAnalysis162(root, req.RunID, canonicalAnalysis, inventory, live, certifyIntent)
}

func sameCanonicalSnapshotAuthority162(expected, live changeset.Snapshot) bool {
	return expected.ResolvedBaseCommit == live.ResolvedBaseCommit &&
		expected.MergeBase == live.MergeBase &&
		expected.HeadCommit == live.HeadCommit &&
		expected.CurrentBranch == live.CurrentBranch &&
		expected.IncludeWorkingTree == live.IncludeWorkingTree &&
		expected.GitStateSHA256 == live.GitStateSHA256 &&
		expected.SnapshotSHA256 == live.SnapshotSHA256
}

func assembleCanonicalAnalysis162(proposalBytes []byte, snapshot changeset.Snapshot) ([]byte, ChangeAnalysis, error) {
	var doc map[string]any
	if err := json.Unmarshal(proposalBytes, &doc); err != nil {
		return nil, ChangeAnalysis{}, fmt.Errorf("ANALYSIS_PROPOSAL_DECODE_FAILED: %w", err)
	}
	rolesRaw, ok := doc["changedFileRoles"].([]any)
	if !ok {
		return nil, ChangeAnalysis{}, fmt.Errorf("ANALYSIS_PROPOSAL_CHANGESET_ROLE_MISMATCH: changedFileRoles missing")
	}
	roles := map[string]string{}
	for _, raw := range rolesRaw {
		item, ok := raw.(map[string]any)
		if !ok {
			return nil, ChangeAnalysis{}, fmt.Errorf("ANALYSIS_PROPOSAL_CHANGESET_ROLE_MISMATCH: invalid role entry")
		}
		p, _ := item["path"].(string)
		role, _ := item["role"].(string)
		clean, valid := safeEvidencePath153(p)
		if !valid || clean != p || role == "" {
			return nil, ChangeAnalysis{}, fmt.Errorf("ANALYSIS_PROPOSAL_CHANGESET_ROLE_MISMATCH: invalid role path %q", p)
		}
		if _, duplicate := roles[clean]; duplicate {
			return nil, ChangeAnalysis{}, fmt.Errorf("ANALYSIS_PROPOSAL_CHANGESET_ROLE_MISMATCH: duplicate role path %q", clean)
		}
		roles[clean] = role
	}
	if len(roles) != len(snapshot.Files) {
		return nil, ChangeAnalysis{}, fmt.Errorf("ANALYSIS_PROPOSAL_CHANGESET_ROLE_MISMATCH: roles=%d snapshotFiles=%d", len(roles), len(snapshot.Files))
	}
	changed := make([]any, 0, len(snapshot.Files))
	for _, file := range snapshot.Files {
		role, exists := roles[file.Path]
		if !exists {
			return nil, ChangeAnalysis{}, fmt.Errorf("ANALYSIS_PROPOSAL_CHANGESET_ROLE_MISMATCH: missing role for %s", file.Path)
		}
		changed = append(changed, map[string]any{"path": file.Path, "role": role, "sources": file.Sources})
	}
	delete(doc, "changedFileRoles")
	doc["reviewScope"] = map[string]any{
		"currentBranch": snapshot.CurrentBranch,
		"baseRef": snapshot.RequestedBaseRef,
		"baseCommit": snapshot.ResolvedBaseCommit,
		"mergeBase": snapshot.MergeBase,
		"headCommit": snapshot.HeadCommit,
		"includeWorkingTree": snapshot.IncludeWorkingTree,
	}
	doc["changedFiles"] = changed
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, ChangeAnalysis{}, fmt.Errorf("ANALYSIS_ASSEMBLE_FAILED: %w", err)
	}
	data = append(data, '\n')
	var typed ChangeAnalysis
	if err := json.Unmarshal(data, &typed); err != nil {
		return nil, ChangeAnalysis{}, fmt.Errorf("ANALYSIS_ASSEMBLE_DECODE_FAILED: %w", err)
	}
	return data, typed, nil
}

func publishCertifiedAnalysis162(root, runID string, canonicalAnalysis []byte, inventory EntrypointInventory, snapshot changeset.Snapshot, certifyIntent Intent) (Certificate, error) {
	inventoryBytes, err := json.MarshalIndent(inventory, "", "  ")
	if err != nil {
		return Certificate{}, fmt.Errorf("ENTRYPOINT_INVENTORY_ENCODE_FAILED: %w", err)
	}
	inventoryBytes = append(inventoryBytes, '\n')
	inventorySchema, err := os.ReadFile(filepath.Join(root, ".code-harness", "contracts", "entrypoint-inventory.schema.json"))
	if err != nil {
		return Certificate{}, fmt.Errorf("ENTRYPOINT_INVENTORY_SCHEMA_READ_FAILED: %w", err)
	}
	if err := schema.ValidateJSON(inventorySchema, inventoryBytes); err != nil {
		return Certificate{}, fmt.Errorf("ENTRYPOINT_INVENTORY_SCHEMA_INVALID: %w", err)
	}
	versionBytes, err := os.ReadFile(filepath.Join(root, ".code-harness", "VERSION"))
	if err != nil {
		return Certificate{}, fmt.Errorf("RUNTIME_VERSION_UNAVAILABLE: %w", err)
	}
	runtimeVersion := strings.TrimSpace(string(versionBytes))
	if runtimeVersion == "" {
		return Certificate{}, fmt.Errorf("RUNTIME_VERSION_UNAVAILABLE: empty VERSION")
	}
	cert := Certificate{
		RunID: runID,
		RuntimeVersion: runtimeVersion,
		AnalysisSHA256: hashBytes153(canonicalAnalysis),
		ChangeSetSHA256: snapshot.SHA256,
		EntrypointInventorySHA256: hashBytes153(inventoryBytes),
		BaseRef: snapshot.RequestedBaseRef,
		Head: snapshot.HeadCommit,
		ResolvedBaseCommit: snapshot.ResolvedBaseCommit,
		MergeBase: snapshot.MergeBase,
		CurrentBranch: snapshot.CurrentBranch,
		IncludeWorkingTree: snapshot.IncludeWorkingTree,
		SnapshotSHA256: snapshot.SnapshotSHA256,
		Intent: &Intent{Mode: certifyIntent.Mode, Target: certifyIntent.Target},
	}
	certBytes, err := json.MarshalIndent(cert, "", "  ")
	if err != nil {
		return Certificate{}, fmt.Errorf("ANALYSIS_CERT_ENCODE_FAILED: %w", err)
	}
	certBytes = append(certBytes, '\n')
	certSchema, err := os.ReadFile(filepath.Join(root, ".code-harness", "contracts", "change-analysis-cert.schema.json"))
	if err != nil {
		return Certificate{}, fmt.Errorf("ANALYSIS_CERT_SCHEMA_READ_FAILED: %w", err)
	}
	if err := schema.ValidateJSON(certSchema, certBytes); err != nil {
		return Certificate{}, fmt.Errorf("ANALYSIS_CERT_SCHEMA_INVALID: %w", err)
	}
	if err := sealChainMaintenanceAuthority153(root, cert); err != nil {
		return Certificate{}, err
	}

	analysisDir := filepath.Join(root, ".code-harness", "runs", runID, "analysis")
	if err := os.MkdirAll(analysisDir, 0o755); err != nil {
		return Certificate{}, fmt.Errorf("ANALYSIS_ARTIFACT_DIR_FAILED: %w", err)
	}
	if _, err := os.Stat(filepath.Join(analysisDir, "change-set.json")); os.IsNotExist(err) {
		snapshotBytes, encodeErr := changeset.CanonicalBytes(snapshot)
		if encodeErr != nil { return Certificate{}, encodeErr }
		if writeErr := atomicWriteCertified153(filepath.Join(analysisDir, "change-set.json"), snapshotBytes); writeErr != nil { return Certificate{}, writeErr }
	} else if err != nil {
		return Certificate{}, fmt.Errorf("CHANGE_SET_SNAPSHOT_READ_FAILED: %w", err)
	}
	if err := atomicWriteCertified153(filepath.Join(analysisDir, "change-analysis.json"), canonicalAnalysis); err != nil { return Certificate{}, err }
	if err := atomicWriteCertified153(filepath.Join(analysisDir, "entrypoint-inventory.json"), inventoryBytes); err != nil { return Certificate{}, err }
	if err := atomicWriteCertified153(filepath.Join(analysisDir, "change-analysis.cert.json"), certBytes); err != nil { return Certificate{}, err }
	return cert, nil
}

func sameRunSnapshotPath162(value, runID string) bool {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" || path.IsAbs(value) { return false }
	return path.Clean(value) == ".code-harness/runs/"+runID+"/analysis/change-set.json"
}

func sameRunProposalPath162(value, runID string) bool {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" || path.IsAbs(value) { return false }
	return path.Clean(value) == ".code-harness/runs/"+runID+"/requests/change-analysis-proposal.json"
}
