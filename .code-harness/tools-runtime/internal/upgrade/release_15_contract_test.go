package upgrade

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRelease15DocumentationAndChainFrameworkContract(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", "..", ".."))
	harnessRoot := filepath.Join(repoRoot, ".code-harness")

	mustRead := func(path string) string {
		t.Helper()
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	readme := mustRead(filepath.Join(repoRoot, "README.md"))
	for _, want := range []string{
		"## 1.5.0",
		"Chain 仅接入 Review",
		"不支持 Test/Debug/Fix Chain",
		"chains/**",
	} {
		if !strings.Contains(readme, want) {
			t.Fatalf("README missing historical 1.5 contract %q", want)
		}
	}

	changelog := mustRead(filepath.Join(repoRoot, "CHANGELOG.md"))
	for _, want := range []string{
		"## 1.5.0 - 2026-08-24",
		"Chain Management",
		"Review Consumes Verified Chains",
		"不支持 Test/Debug/Fix Chain",
	} {
		if !strings.Contains(changelog, want) {
			t.Fatalf("CHANGELOG missing historical 1.5 contract %q", want)
		}
	}

	upgradeDoc := mustRead(filepath.Join(harnessRoot, "upgrade.md"))
	for _, want := range []string{
		"chains/**",
		"byte-for-byte",
		"1.4.0 → 1.5.0",
	} {
		if !strings.Contains(upgradeDoc, want) {
			t.Fatalf("upgrade.md missing 1.5 preservation contract %q", want)
		}
	}

	for _, rel := range []string{
		"contracts/chain.schema.json",
		"contracts/chain-validation-result.schema.json",
		"templates/chain.template.yaml",
		"skills/discover-chain/SKILL.md",
		"skills/validate-chain/SKILL.md",
		"tools-runtime/internal/chain",
		"tools-runtime/internal/reviewscope",
	} {
		if _, err := os.Stat(filepath.Join(harnessRoot, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("current release missing preserved 1.5 Chain Framework %s: %v", rel, err)
		}
	}

	workflow := mustRead(filepath.Join(repoRoot, ".github", "workflows", "package-windows-x64.yml"))
	releaseDriver := mustRead(filepath.Join(repoRoot, ".github", "scripts", "task161-release.ps1"))
	if !strings.Contains(workflow, "task161-release.ps1") {
		t.Fatal("package workflow no longer routes through current release certification driver")
	}
	if !strings.Contains(releaseDriver, "task153-real-review-chain-regression.ps1") {
		t.Fatal("current release certification no longer executes Chain regression")
	}
}
