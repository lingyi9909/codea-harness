package analysis

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"codea-harness-tools/internal/changeset"
	"codea-harness-tools/internal/coverage"
	"codea-harness-tools/internal/schema"
)

var artifactID153 = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type CertifyRequest struct {
	RunID              string `json:"runId"`
	DraftPath          string `json:"draftPath,omitempty"`
	BaseRef            string `json:"baseRef,omitempty"`
	IncludeWorkingTree bool   `json:"includeWorkingTree,omitempty"`
	ProposalPath       string `json:"proposalPath,omitempty"`
	SnapshotPath       string `json:"snapshotPath,omitempty"`
	SnapshotSHA256     string `json:"snapshotSha256,omitempty"`
	Intent             Intent `json:"intent"`
}

type certificationRuntime153 interface {
	Compute(root, baseRef string, includeWorkingTree bool) (changeset.Snapshot, error)
	Inventory(root, runID string, snapshot changeset.Snapshot, intent Intent) (EntrypointInventory, error)
}

type defaultCertificationRuntime153 struct{}

func (defaultCertificationRuntime153) Compute(root, baseRef string, includeWorkingTree bool) (changeset.Snapshot, error) {
	return changeset.Compute(root, baseRef, includeWorkingTree)
}

func (defaultCertificationRuntime153) Inventory(root, runID string, snapshot changeset.Snapshot, intent Intent) (EntrypointInventory, error) {
	return BuildEntrypointInventory(root, runID, snapshot, intent)
}

type certifyDraftMeta153 struct {
	ReviewScope struct {
		CurrentBranch      string `json:"currentBranch"`
		BaseRef            string `json:"baseRef"`
		BaseCommit         string `json:"baseCommit"`
		MergeBase          string `json:"mergeBase"`
		HeadCommit         string `json:"headCommit"`
		IncludeWorkingTree bool   `json:"includeWorkingTree"`
	} `json:"reviewScope"`
	ChangedFiles []struct {
		Path    string             `json:"path"`
		Role    string             `json:"role"`
		Sources []changeset.Source `json:"sources"`
	} `json:"changedFiles"`
}

func Certify(root string, req CertifyRequest) (Certificate, error) {
	return certifyWithRuntime153(root, req, defaultCertificationRuntime153{})
}

func certifyWithRuntime153(root string, req CertifyRequest, runtime certificationRuntime153) (Certificate, error) {
	root = filepath.Clean(root)
	if isCanonicalCertifyRequest162(req) {
		return certifyCanonical162(root, req, runtime)
	}
	if !artifactID153.MatchString(strings.TrimSpace(req.RunID)) {
		return Certificate{}, fmt.Errorf("ANALYSIS_CERTIFY_RUN_ID_INVALID: %q", req.RunID)
	}
	if strings.TrimSpace(req.BaseRef) == "" || strings.TrimSpace(req.Intent.Mode) == "" {
		return Certificate{}, fmt.Errorf("ANALYSIS_CERTIFY_REQUEST_INVALID: baseRef and intent.mode are required")
	}
	if !sameRunDraftPath153(req.DraftPath, req.RunID) {
		return Certificate{}, fmt.Errorf("ANALYSIS_DRAFT_PATH_INVALID: must be .code-harness/runs/%s/requests/change-analysis-draft.json", req.RunID)
	}

	draftPath := filepath.Join(root, filepath.FromSlash(req.DraftPath))
	draftBytes, err := os.ReadFile(draftPath)
	if err != nil { return Certificate{}, fmt.Errorf("ANALYSIS_DRAFT_READ_FAILED: %w", err) }
	analysisSchema, err := os.ReadFile(filepath.Join(root, ".code-harness", "contracts", "change-analysis.schema.json"))
	if err != nil { return Certificate{}, fmt.Errorf("ANALYSIS_SCHEMA_READ_FAILED: %w", err) }
	if err := schema.ValidateJSON(analysisSchema, draftBytes); err != nil { return Certificate{}, fmt.Errorf("ANALYSIS_DRAFT_SCHEMA_INVALID: %w", err) }

	var meta certifyDraftMeta153
	if err := json.Unmarshal(draftBytes, &meta); err != nil { return Certificate{}, fmt.Errorf("ANALYSIS_DRAFT_DECODE_FAILED: %w", err) }
	var typed ChangeAnalysis
	if err := json.Unmarshal(draftBytes, &typed); err != nil { return Certificate{}, fmt.Errorf("ANALYSIS_DRAFT_DECODE_FAILED: %w", err) }
	if meta.ReviewScope.IncludeWorkingTree != req.IncludeWorkingTree {
		return Certificate{}, fmt.Errorf("CHANGE_SET_MISMATCH: draft includeWorkingTree does not match certify request")
	}

	snapshot, err := runtime.Compute(root, req.BaseRef, req.IncludeWorkingTree)
	if err != nil { return Certificate{}, err }
	if snapshot.ResolvedBaseCommit != "" {
		if meta.ReviewScope.BaseCommit != snapshot.ResolvedBaseCommit || meta.ReviewScope.MergeBase != snapshot.MergeBase || meta.ReviewScope.HeadCommit != snapshot.HeadCommit || meta.ReviewScope.CurrentBranch != snapshot.CurrentBranch {
			return Certificate{}, fmt.Errorf("CHANGE_SET_MISMATCH: draft deterministic Git identity differs from Runtime snapshot")
		}
	} else {
		if meta.ReviewScope.BaseRef != req.BaseRef || strings.TrimSpace(meta.ReviewScope.HeadCommit) == "" || meta.ReviewScope.HeadCommit != snapshot.Head {
			return Certificate{}, fmt.Errorf("CHANGE_SET_MISMATCH: draft reviewScope does not match certify request")
		}
	}
	if err := compareDraftChangeSet153(meta.ChangedFiles, snapshot.Files); err != nil { return Certificate{}, err }

	certifyIntent := Intent{Mode: strings.ToUpper(strings.TrimSpace(req.Intent.Mode)), Target: strings.TrimSpace(req.Intent.Target)}
	inventory, err := runtime.Inventory(root, req.RunID, snapshot, req.Intent)
	if err != nil { return Certificate{}, err }
	if inventory.RunID != req.RunID || inventory.ChangeSetSHA256 != snapshot.SHA256 || inventory.Status != inventoryComplete153 {
		return Certificate{}, fmt.Errorf("ENTRYPOINT_INVENTORY_IDENTITY_MISMATCH")
	}
	inventory.Intent = &Intent{Mode: certifyIntent.Mode, Target: certifyIntent.Target}
	if err := VerifyEntrypointDispositions(inventory, typed); err != nil { return Certificate{}, err }
	if err := validateEvidenceAtRoot153(root, typed, inventory); err != nil { return Certificate{}, err }

	canonicalAnalysis, err := canonicalJSON153(draftBytes)
	if err != nil { return Certificate{}, fmt.Errorf("ANALYSIS_CANONICALIZE_FAILED: %w", err) }
	if _, err := coverage.VerifyAnalysisJSON(canonicalAnalysis); err != nil { return Certificate{}, fmt.Errorf("ANALYSIS_COVERAGE_INVALID: %w", err) }

	if snapshot.ResolvedBaseCommit != "" && snapshot.GitStateSHA256 != "" {
		return publishCertifiedAnalysis162(root, req.RunID, canonicalAnalysis, inventory, snapshot, certifyIntent)
	}

	inventoryBytes, err := json.MarshalIndent(inventory, "", "  ")
	if err != nil { return Certificate{}, fmt.Errorf("ENTRYPOINT_INVENTORY_ENCODE_FAILED: %w", err) }
	inventoryBytes = append(inventoryBytes, '\n')
	inventorySchema, err := os.ReadFile(filepath.Join(root, ".code-harness", "contracts", "entrypoint-inventory.schema.json"))
	if err != nil { return Certificate{}, fmt.Errorf("ENTRYPOINT_INVENTORY_SCHEMA_READ_FAILED: %w", err) }
	if err := schema.ValidateJSON(inventorySchema, inventoryBytes); err != nil { return Certificate{}, fmt.Errorf("ENTRYPOINT_INVENTORY_SCHEMA_INVALID: %w", err) }

	versionBytes, err := os.ReadFile(filepath.Join(root, ".code-harness", "VERSION"))
	if err != nil { return Certificate{}, fmt.Errorf("RUNTIME_VERSION_UNAVAILABLE: %w", err) }
	runtimeVersion := strings.TrimSpace(string(versionBytes))
	if runtimeVersion == "" { return Certificate{}, fmt.Errorf("RUNTIME_VERSION_UNAVAILABLE: empty VERSION") }
	cert := Certificate{
		RunID: req.RunID, RuntimeVersion: runtimeVersion, AnalysisSHA256: hashBytes153(canonicalAnalysis),
		ChangeSetSHA256: snapshot.SHA256, EntrypointInventorySHA256: hashBytes153(inventoryBytes), BaseRef: snapshot.BaseRef, Head: snapshot.Head,
		Intent: &Intent{Mode: certifyIntent.Mode, Target: certifyIntent.Target},
	}
	certBytes, err := json.MarshalIndent(cert, "", "  ")
	if err != nil { return Certificate{}, fmt.Errorf("ANALYSIS_CERT_ENCODE_FAILED: %w", err) }
	certBytes = append(certBytes, '\n')
	certSchema, err := os.ReadFile(filepath.Join(root, ".code-harness", "contracts", "change-analysis-cert.schema.json"))
	if err != nil { return Certificate{}, fmt.Errorf("ANALYSIS_CERT_SCHEMA_READ_FAILED: %w", err) }
	if err := schema.ValidateJSON(certSchema, certBytes); err != nil { return Certificate{}, fmt.Errorf("ANALYSIS_CERT_SCHEMA_INVALID: %w", err) }
	if err := sealChainMaintenanceAuthority153(root, cert); err != nil { return Certificate{}, err }

	analysisDir := filepath.Join(root, ".code-harness", "runs", req.RunID, "analysis")
	if err := os.MkdirAll(analysisDir, 0o755); err != nil { return Certificate{}, fmt.Errorf("ANALYSIS_ARTIFACT_DIR_FAILED: %w", err) }
	if err := atomicWriteCertified153(filepath.Join(analysisDir, "change-analysis.json"), canonicalAnalysis); err != nil { return Certificate{}, err }
	if err := atomicWriteCertified153(filepath.Join(analysisDir, "entrypoint-inventory.json"), inventoryBytes); err != nil { return Certificate{}, err }
	if err := atomicWriteCertified153(filepath.Join(analysisDir, "change-analysis.cert.json"), certBytes); err != nil { return Certificate{}, err }
	return cert, nil
}

func compareDraftChangeSet153(draft []struct {
	Path    string             `json:"path"`
	Role    string             `json:"role"`
	Sources []changeset.Source `json:"sources"`
}, actual []changeset.File) error {
	type entry struct { path string; sources string }
	canonSources := func(in []changeset.Source) string {
		copyIn := append([]changeset.Source(nil), in...)
		sort.Slice(copyIn, func(i, j int) bool { return copyIn[i] < copyIn[j] })
		out := make([]string, 0, len(copyIn)); last := changeset.Source("")
		for _, source := range copyIn { if source == last { continue }; last = source; out = append(out, string(source)) }
		return strings.Join(out, ",")
	}
	draftSet := map[string]entry{}
	for _, f := range draft {
		p, ok := safeEvidencePath153(f.Path); if !ok { return fmt.Errorf("CHANGE_SET_MISMATCH: invalid draft path %q", f.Path) }
		if _, exists := draftSet[p]; exists { return fmt.Errorf("CHANGE_SET_MISMATCH: duplicate draft path %q", p) }
		draftSet[p] = entry{path: p, sources: canonSources(f.Sources)}
	}
	actualSet := map[string]entry{}
	for _, f := range actual {
		p, ok := safeEvidencePath153(f.Path); if !ok { return fmt.Errorf("CHANGE_SET_MISMATCH: invalid runtime path %q", f.Path) }
		actualSet[p] = entry{path: p, sources: canonSources(f.Sources)}
	}
	if len(draftSet) != len(actualSet) { return fmt.Errorf("CHANGE_SET_MISMATCH: draft files=%d runtime files=%d", len(draftSet), len(actualSet)) }
	for p, expected := range actualSet {
		got, ok := draftSet[p]
		if !ok || got.sources != expected.sources { return fmt.Errorf("CHANGE_SET_MISMATCH: %s draft sources=%q runtime sources=%q", p, got.sources, expected.sources) }
	}
	return nil
}

func sameRunDraftPath153(value, runID string) bool {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" || path.IsAbs(value) { return false }
	return path.Clean(value) == ".code-harness/runs/"+runID+"/requests/change-analysis-draft.json"
}

func canonicalJSON153(raw []byte) ([]byte, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil { return nil, err }
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil { return nil, err }
	return append(b, '\n'), nil
}

func hashBytes153(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

func atomicWriteCertified153(target string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(target), ".certified-analysis-*.tmp")
	if err != nil { return fmt.Errorf("ANALYSIS_ARTIFACT_TEMP_FAILED: %w", err) }
	tmpName := tmp.Name(); defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil { _ = tmp.Close(); return fmt.Errorf("ANALYSIS_ARTIFACT_WRITE_FAILED: %w", err) }
	if err := tmp.Chmod(0o644); err != nil { _ = tmp.Close(); return err }
	if err := tmp.Close(); err != nil { return err }
	if err := os.Rename(tmpName, target); err != nil { return fmt.Errorf("ANALYSIS_ARTIFACT_PUBLISH_FAILED: %w", err) }
	return nil
}
