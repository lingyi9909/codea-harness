package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/changeset"
)

func prepareCommittedCertifiedAnalysisFixture153(t *testing.T, runID, analysisPath string) {
	t.Helper()
	data, err := os.ReadFile(analysisPath)
	if err != nil { t.Fatal(err) }
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil { t.Fatal(err) }
	changed, ok := doc["changedFiles"].([]any)
	if !ok { t.Fatal("legacy analysis changedFiles must be an array") }

	final := map[string][]byte{}
	for _, raw := range changed {
		item, ok := raw.(map[string]any)
		if !ok { t.Fatal("legacy analysis changedFiles entry must be an object") }
		p, _ := item["path"].(string)
		if strings.TrimSpace(p) == "" { t.Fatal("legacy analysis changedFiles entry missing path") }
		if b, err := os.ReadFile(filepath.FromSlash(p)); err == nil {
			final[p] = b
		} else if os.IsNotExist(err) {
			final[p] = []byte(finalFixtureContent153(p))
		} else {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(filepath.FromSlash(p)), 0o755); err != nil { t.Fatal(err) }
		if err := os.WriteFile(filepath.FromSlash(p), []byte(baseFixtureContent153(p)), 0o644); err != nil { t.Fatal(err) }
		item["sources"] = []any{"COMMITTED"}
	}

	git153Cmd(t, ".", "init", "-b", "develop")
	git153Cmd(t, ".", "config", "user.email", "task153-fixture@example.test")
	git153Cmd(t, ".", "config", "user.name", "Task 153 Fixture")
	git153Cmd(t, ".", "add", "src")
	git153Cmd(t, ".", "commit", "-m", "fixture base")
	base := git153Cmd(t, ".", "rev-parse", "HEAD")

	for p, b := range final {
		if err := os.WriteFile(filepath.FromSlash(p), b, 0o644); err != nil { t.Fatal(err) }
	}
	git153Cmd(t, ".", "add", "src")
	git153Cmd(t, ".", "commit", "-m", "fixture head")
	head := git153Cmd(t, ".", "rev-parse", "HEAD")

	doc["reviewScope"] = map[string]any{
		"currentBranch": "develop", "baseRef": base, "baseCommit": base,
		"mergeBase": base, "headCommit": head, "includeWorkingTree": true,
	}
	canonical, err := json.MarshalIndent(doc, "", "  ")
	if err != nil { t.Fatal(err) }
	canonical = append(canonical, '\n')
	if err := os.WriteFile(analysisPath, canonical, 0o644); err != nil { t.Fatal(err) }
	sealExistingAnalysisFixture153(t, runID, analysisPath)
}

func sealExistingAnalysisFixture153(t *testing.T, runID, analysisPath string) {
	t.Helper()
	installCertifiedSchemas153(t)
	analysisBytes, err := os.ReadFile(analysisPath)
	if err != nil { t.Fatal(err) }
	var meta struct {
		ReviewScope struct {
			BaseRef string `json:"baseRef"`
			HeadCommit string `json:"headCommit"`
			IncludeWorkingTree bool `json:"includeWorkingTree"`
		} `json:"reviewScope"`
	}
	if err := json.Unmarshal(analysisBytes, &meta); err != nil { t.Fatal(err) }
	snapshot, err := changeset.Compute(".", meta.ReviewScope.BaseRef, meta.ReviewScope.IncludeWorkingTree)
	if err != nil { t.Fatalf("compute fixture Change Set: %v", err) }
	if meta.ReviewScope.HeadCommit != snapshot.Head {
		t.Fatalf("fixture headCommit=%s runtime HEAD=%s", meta.ReviewScope.HeadCommit, snapshot.Head)
	}

	inventory := analysisruntime.EntrypointInventory{
		RunID: runID, Status: "COMPLETE", ExpectedEntrypoints: []analysisruntime.ExpectedEntrypoint{}, ChangeSetSHA256: snapshot.SHA256,
	}
	inventoryBytes, err := json.MarshalIndent(inventory, "", "  ")
	if err != nil { t.Fatal(err) }
	inventoryBytes = append(inventoryBytes, '\n')
	analysisDir := filepath.Dir(analysisPath)
	if err := os.WriteFile(filepath.Join(analysisDir, "entrypoint-inventory.json"), inventoryBytes, 0o644); err != nil { t.Fatal(err) }

	cert := analysisruntime.Certificate{
		RunID: runID,
		RuntimeVersion: "1.5.2",
		AnalysisSHA256: sha153Fixture(analysisBytes),
		ChangeSetSHA256: snapshot.SHA256,
		EntrypointInventorySHA256: sha153Fixture(inventoryBytes),
		BaseRef: snapshot.BaseRef,
		Head: snapshot.Head,
	}
	certBytes, err := json.MarshalIndent(cert, "", "  ")
	if err != nil { t.Fatal(err) }
	certBytes = append(certBytes, '\n')
	if err := os.WriteFile(filepath.Join(analysisDir, "change-analysis.cert.json"), certBytes, 0o644); err != nil { t.Fatal(err) }
}

func installCertifiedSchemas153(t *testing.T) {
	t.Helper()
	for _, name := range []string{"change-analysis.schema.json", "entrypoint-inventory.schema.json", "change-analysis-cert.schema.json"} {
		copyTask153CommandContract(t, ".", name)
	}
	writeFile(t, filepath.Join(".code-harness", "VERSION"), "1.5.2\n")
}

func sha153Fixture(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func baseFixtureContent153(p string) string {
	switch {
	case strings.HasSuffix(p, ".java"):
		return "package fixture; class FixtureBase { int value = 1; }\n"
	case strings.HasSuffix(p, "Mapper.xml"):
		return "<mapper id=\"base\"/>\n"
	case strings.HasSuffix(p, ".yml"):
		return "fixture: base\n"
	default:
		return "fixture-base\n"
	}
}

func finalFixtureContent153(p string) string {
	switch {
	case strings.HasSuffix(p, ".java"):
		return "package fixture; class FixtureHead { int value = 2; }\n"
	case strings.HasSuffix(p, "Mapper.xml"):
		return "<mapper id=\"head\"/>\n"
	case strings.HasSuffix(p, ".yml"):
		return "fixture: head\n"
	default:
		return "fixture-head\n"
	}
}
