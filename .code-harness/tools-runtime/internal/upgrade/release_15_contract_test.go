package upgrade

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRelease15DocumentationAndWindowsPackagingContract(t *testing.T) {
	harnessRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	repoRoot := filepath.Dir(harnessRoot)

	mustRead := func(path string) string {
		t.Helper()
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	if got := strings.TrimSpace(mustRead(filepath.Join(harnessRoot, "VERSION"))); got != "1.5.0" {
		t.Fatalf("release VERSION=%q want 1.5.0", got)
	}

	readme := mustRead(filepath.Join(repoRoot, "README.md"))
	for _, want := range []string{
		"## 1.5.0",
		"codea-harness-1.5.0-windows-x64-install.zip",
		"codea-harness-1.5.0-windows-x64-upgrade.zip",
		"Chain 仅接入 Review",
		"不支持 Test/Debug/Fix Chain",
		"chains/**",
	} {
		if !strings.Contains(readme, want) {
			t.Fatalf("README missing 1.5 release contract %q", want)
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
			t.Fatalf("CHANGELOG missing 1.5 release contract %q", want)
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

	workflow := mustRead(filepath.Join(repoRoot, ".github", "workflows", "package-windows-x64.yml"))
	for _, want := range []string{
		"go test -count=1 ./internal/chain ./internal/reviewscope ./internal/coverage ./internal/report",
		"bedf2cde3784a6ee15d408271a023a95570c46b8",
		"contracts/chain.schema.json",
		"contracts/chain-validation-result.schema.json",
		"templates/chain.template.yaml",
		"skills/discover-chain/SKILL.md",
		"skills/validate-chain/SKILL.md",
		"codea-harness-1.5.0-windows-x64-install",
		"codea-harness-1.5.0-windows-x64-upgrade",
		"chains",
		"chain validate",
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("package-windows-x64 missing 1.5 release gate %q", want)
		}
	}
}
