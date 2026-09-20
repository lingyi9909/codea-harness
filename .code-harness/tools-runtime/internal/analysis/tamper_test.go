package analysis

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/changeset"
)

func Test153TamperRejectsChangedAuthoritativeAnalysisBytes(t *testing.T) {
	root, analysisPath, runtime := writeCertifiedFixture153(t)
	p := filepath.Join(root, filepath.FromSlash(analysisPath))
	b, err := os.ReadFile(p)
	if err != nil { t.Fatal(err) }
	if err := os.WriteFile(p, append(b, '\n'), 0o644); err != nil { t.Fatal(err) }
	if _, _, err := loadCertifiedWithRuntime153(root, analysisPath, runtime); err == nil || !strings.Contains(err.Error(), "CHANGED_ANALYSIS_HASH_MISMATCH") {
		t.Fatalf("mutated authoritative analysis must fail closed, got %v", err)
	}
}

func Test153TamperRejectsChangedEntrypointInventoryBytes(t *testing.T) {
	root, analysisPath, runtime := writeCertifiedFixture153(t)
	p := filepath.Join(root, ".code-harness", "runs", "r153", "analysis", "entrypoint-inventory.json")
	b, err := os.ReadFile(p)
	if err != nil { t.Fatal(err) }
	if err := os.WriteFile(p, append(b, '\n'), 0o644); err != nil { t.Fatal(err) }
	if _, _, err := loadCertifiedWithRuntime153(root, analysisPath, runtime); err == nil || !strings.Contains(err.Error(), "ENTRYPOINT_INVENTORY_HASH_MISMATCH") {
		t.Fatalf("mutated inventory must fail closed, got %v", err)
	}
}

func Test153TamperRejectsChangedCertificateBytes(t *testing.T) {
	root, analysisPath, runtime := writeCertifiedFixture153(t)
	p := filepath.Join(root, ".code-harness", "runs", "r153", "analysis", "change-analysis.cert.json")
	b, err := os.ReadFile(p)
	if err != nil { t.Fatal(err) }
	if err := os.WriteFile(p, append(b, '\n'), 0o644); err != nil { t.Fatal(err) }
	if _, _, err := loadCertifiedWithRuntime153(root, analysisPath, runtime); err == nil || !strings.Contains(err.Error(), "CERTIFICATE_BYTES_NOT_CANONICAL") {
		t.Fatalf("mutated certificate bytes must fail closed, got %v", err)
	}
}

func Test153TamperRejectsStaleCurrentChangeSet(t *testing.T) {
	root, analysisPath, runtime := writeCertifiedFixture153(t)
	runtime.snapshot.SHA256 = strings.Repeat("f", 64)
	if _, _, err := loadCertifiedWithRuntime153(root, analysisPath, runtime); err == nil || !strings.Contains(err.Error(), "CERTIFIED_CHANGE_SET_STALE") {
		t.Fatalf("stale certified Change Set must fail closed, got %v", err)
	}
}

func Test153CertifiedLoaderRejectsMissingRuntimeVersion(t *testing.T) {
	root, analysisPath, runtime := writeCertifiedFixture153(t)
	if err := os.Remove(filepath.Join(root, ".code-harness", "VERSION")); err != nil { t.Fatal(err) }
	if _, _, err := loadCertifiedWithRuntime153(root, analysisPath, runtime); err == nil || !strings.Contains(err.Error(), "CERTIFIED_RUNTIME_VERSION_UNAVAILABLE") {
		t.Fatalf("missing Runtime VERSION must fail closed, got %v", err)
	}
}

func Test153CertifiedLoaderRejectsRuntimeVersionMismatch(t *testing.T) {
	root, analysisPath, runtime := writeCertifiedFixture153(t)
	if err := os.WriteFile(filepath.Join(root, ".code-harness", "VERSION"), []byte("9.9.9\n"), 0o644); err != nil { t.Fatal(err) }
	if _, _, err := loadCertifiedWithRuntime153(root, analysisPath, runtime); err == nil || !strings.Contains(err.Error(), "CERTIFIED_RUNTIME_VERSION_MISMATCH") {
		t.Fatalf("Runtime VERSION mismatch must fail closed, got %v", err)
	}
}

func writeCertifiedFixture153(t *testing.T) (string, string, fakeCertificationRuntime153) {
	t.Helper()
	root := t.TempDir()
	copyAnalysisContract153(t, root, "change-analysis.schema.json")
	copyAnalysisContract153(t, root, "entrypoint-inventory.schema.json")
	copyAnalysisContract153(t, root, "change-analysis-cert.schema.json")
	if err := os.WriteFile(filepath.Join(root, ".code-harness", "VERSION"), []byte("1.5.2\n"), 0o644); err != nil { t.Fatal(err) }

	doc := map[string]any{
		"reviewScope": map[string]any{"currentBranch": "feature", "baseRef": "develop", "baseCommit": "base153", "mergeBase": "base153", "headCommit": "head153", "includeWorkingTree": true},
		"changedFiles": []any{}, "affectedControllers": []any{}, "callChains": []any{}, "symbolLocations": []any{}, "resourceRelations": []any{}, "externalDependencies": []any{}, "riskAreas": []any{},
		"reviewCoverage": map[string]any{"status": "COMPLETE", "reviewedFiles": []any{}, "unresolvedSymbols": []any{}},
	}
	analysisBytes, err := json.MarshalIndent(doc, "", "  ")
	if err != nil { t.Fatal(err) }
	analysisBytes = append(analysisBytes, '\n')
	inventory := EntrypointInventory{RunID: "r153", Status: "COMPLETE", ExpectedEntrypoints: []ExpectedEntrypoint{}, ChangeSetSHA256: strings.Repeat("a", 64)}
	inventoryBytes, err := json.MarshalIndent(inventory, "", "  ")
	if err != nil { t.Fatal(err) }
	inventoryBytes = append(inventoryBytes, '\n')
	cert := Certificate{
		RunID: "r153", RuntimeVersion: "1.5.2", AnalysisSHA256: testHash153(analysisBytes), ChangeSetSHA256: strings.Repeat("a", 64),
		EntrypointInventorySHA256: testHash153(inventoryBytes), BaseRef: "develop", Head: "head153",
	}
	certBytes, err := json.MarshalIndent(cert, "", "  ")
	if err != nil { t.Fatal(err) }
	certBytes = append(certBytes, '\n')

	base := filepath.Join(root, ".code-harness", "runs", "r153", "analysis")
	if err := os.MkdirAll(base, 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(base, "change-analysis.json"), analysisBytes, 0o644); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(base, "entrypoint-inventory.json"), inventoryBytes, 0o644); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(base, "change-analysis.cert.json"), certBytes, 0o644); err != nil { t.Fatal(err) }

	runtime := fakeCertificationRuntime153{snapshot: changeset.Snapshot{BaseRef: "develop", Head: "head153", SHA256: strings.Repeat("a", 64)}}
	return root, ".code-harness/runs/r153/analysis/change-analysis.json", runtime
}

func testHash153(b []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(b))
}
