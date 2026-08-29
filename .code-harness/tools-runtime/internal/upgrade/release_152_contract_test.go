package upgrade

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRelease152WorkspaceDependencyPackagingContract(t *testing.T) {
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

	if got := strings.TrimSpace(mustRead(filepath.Join(harnessRoot, "VERSION"))); got != "1.6.0" {
		t.Fatalf("current release VERSION=%q want 1.6.0", got)
	}

	changelog := mustRead(filepath.Join(repoRoot, "CHANGELOG.md"))
	for _, want := range []string{
		"## 1.5.2 - 2026-08-25",
		"Workspace Dependency Chain Navigation",
		"Navigation Scope",
		"Maven",
		"AMBIGUOUS_TEMPLATE_DISPATCH",
		"无 harness config migration",
	} {
		if !strings.Contains(changelog, want) {
			t.Fatalf("CHANGELOG missing 1.5.2 contract %q", want)
		}
	}

	workflow := mustRead(filepath.Join(repoRoot, ".github", "workflows", "package-windows-x64.yml"))
	for _, want := range []string{
		"6f4c050783a7ec21f370799c1a8c69c9b51a9e92",
		"codea-harness-1.6.0-windows-x64-install",
		"codea-harness-1.6.0-windows-x64-upgrade",
		"1.5.3 -> 1.6.0",
		"Workspace dependency Maven identity regression",
		"Template inheritance navigation regression",
		"Task 6 review isolation regression",
		"Task 5 real dual-project business regression",
		"workspaceDependencies",
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("package-windows-x64 missing preserved 1.5.2 release gate %q", want)
		}
	}
}
