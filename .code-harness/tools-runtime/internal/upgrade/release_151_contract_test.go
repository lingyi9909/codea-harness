package upgrade

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRelease151ChainDiscoverBootstrapPackagingContract(t *testing.T) {
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

	if got := strings.TrimSpace(mustRead(filepath.Join(harnessRoot, "VERSION"))); got != "1.5.1" {
		t.Fatalf("release VERSION=%q want 1.5.1", got)
	}

	changelog := mustRead(filepath.Join(repoRoot, "CHANGELOG.md"))
	for _, want := range []string{
		"## 1.5.1 - 2026-08-24",
		"Chain Discover Bootstrap Fix",
		"无需先执行 harness review",
		"1.5.0 → 1.5.1",
	} {
		if !strings.Contains(changelog, want) {
			t.Fatalf("CHANGELOG missing 1.5.1 contract %q", want)
		}
	}

	workflow := mustRead(filepath.Join(repoRoot, ".github", "workflows", "package-windows-x64.yml"))
	for _, want := range []string{
		"834a2c82e00476a081a1640bd88e3c9b881ba9a7",
		"codea-harness-1.5.1-windows-x64-install",
		"codea-harness-1.5.1-windows-x64-upgrade",
		"1.5.0 to 1.5.1",
		"chain discover",
		"NewController",
		"chains",
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("package-windows-x64 missing 1.5.1 release gate %q", want)
		}
	}
}
