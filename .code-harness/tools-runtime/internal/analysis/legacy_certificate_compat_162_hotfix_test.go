package analysis

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"codea-harness-tools/internal/changeset"
)

// Test162HotfixLoadCertifiedAcceptsE737LegacyCertificateBytes protects the
// on-disk certificate shape emitted before the Canonical ChangeSet hotfix at
// e737a3b6e77af07df14de4f887ad0b8c9dddcf03. The fixture below deliberately
// does not marshal the current Certificate type: its field set/order is the
// legacy 1.6.2 format and intentionally has no canonical snapshot identity or
// includeWorkingTree certificate field.
func Test162HotfixLoadCertifiedAcceptsE737LegacyCertificateBytes(t *testing.T) {
	root := t.TempDir()
	runLegacyGit162(t, root, "init")
	runLegacyGit162(t, root, "config", "user.email", "legacy-cert@example.test")
	runLegacyGit162(t, root, "config", "user.name", "Legacy Cert Fixture")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("legacy fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runLegacyGit162(t, root, "add", "README.md")
	runLegacyGit162(t, root, "commit", "-m", "legacy base")

	for _, name := range []string{"change-analysis.schema.json", "entrypoint-inventory.schema.json", "change-analysis-cert.schema.json"} {
		copyAnalysisContract153(t, root, name)
	}
	writeLegacyArtifact162(t, root, ".code-harness/VERSION", []byte("1.6.2\n"))

	live, err := changeset.Compute(root, "HEAD", true)
	if err != nil {
		t.Fatalf("compute unchanged Git state: %v", err)
	}
	legacyChangeSetSHA := e737LegacyChangeSetSHA162(t, "HEAD", live.HeadCommit, live.Files)

	analysisDoc := map[string]any{
		"reviewScope": map[string]any{
			"currentBranch": live.CurrentBranch,
			"baseRef": "HEAD",
			"baseCommit": live.ResolvedBaseCommit,
			"mergeBase": live.MergeBase,
			"headCommit": live.HeadCommit,
			"includeWorkingTree": true,
		},
		"changedFiles": []any{},
		"affectedControllers": []any{},
		"callChains": []any{},
		"externalDependencies": []any{},
		"riskAreas": []any{},
		"reviewCoverage": map[string]any{
			"status": "COMPLETE",
			"reviewedFiles": []any{},
			"unresolvedSymbols": []any{},
		},
	}
	analysisBytes := marshalCanonicalLegacyArtifact162(t, analysisDoc)
	analysisPath := ".code-harness/runs/legacy-e737/analysis/change-analysis.json"
	writeLegacyArtifact162(t, root, analysisPath, analysisBytes)

	inventory := EntrypointInventory{
		RunID: "legacy-e737",
		Status: "COMPLETE",
		ExpectedEntrypoints: []ExpectedEntrypoint{},
		ChangeSetSHA256: legacyChangeSetSHA,
		Intent: &Intent{Mode: "FULL"},
	}
	inventoryBytes := marshalCanonicalLegacyArtifact162(t, inventory)
	writeLegacyArtifact162(t, root, ".code-harness/runs/legacy-e737/analysis/entrypoint-inventory.json", inventoryBytes)

	// Exact legacy Certificate JSON field set/order from e737a3b6: no
	// resolvedBaseCommit/mergeBase/currentBranch/includeWorkingTree/snapshotSha256.
	legacyCertBytes := []byte(fmt.Sprintf("{\n  \"runId\": \"legacy-e737\",\n  \"runtimeVersion\": \"1.6.2\",\n  \"analysisSha256\": \"%s\",\n  \"changeSetSha256\": \"%s\",\n  \"entrypointInventorySha256\": \"%s\",\n  \"baseRef\": \"HEAD\",\n  \"head\": \"%s\",\n  \"intent\": {\n    \"mode\": \"FULL\"\n  }\n}\n",
		hashBytes153(analysisBytes), legacyChangeSetSHA, hashBytes153(inventoryBytes), live.HeadCommit))
	writeLegacyArtifact162(t, root, ".code-harness/runs/legacy-e737/analysis/change-analysis.cert.json", legacyCertBytes)

	if _, _, err := LoadCertified(root, analysisPath); err != nil {
		t.Fatalf("new Runtime must load unchanged pre-hotfix e737 legacy certificate bytes: %v", err)
	}
}

func e737LegacyChangeSetSHA162(t *testing.T, baseRef, head string, files []changeset.File) string {
	t.Helper()
	// This is the exact pre-hotfix e737a3b6 ChangeSet certificate identity.
	canonical, err := json.Marshal(struct {
		BaseRef string           `json:"baseRef"`
		Head    string           `json:"head"`
		Files   []changeset.File `json:"files"`
	}{BaseRef: baseRef, Head: head, Files: files})
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(canonical))
}

func marshalCanonicalLegacyArtifact162(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(data, '\n')
}

func writeLegacyArtifact162(t *testing.T, root, rel string, data []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func runLegacyGit162(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v: %s", args, err, string(out))
	}
}
