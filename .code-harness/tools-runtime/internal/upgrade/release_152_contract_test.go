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

	if got := strings.TrimSpace(mustRead(filepath.Join(harnessRoot, "VERSION"))); got == "" {
		t.Fatal("current release VERSION must not be empty")
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
	releaseDriver := mustRead(filepath.Join(repoRoot, ".github", "scripts", "task161-release.ps1"))
	for _, want := range []string{"task161-release.ps1"} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("package-windows-x64 missing current release driver %q", want)
		}
	}
	for _, want := range []string{
		"task152-workspace-smoke.ps1",
		"task152-task5-real-business-regression.ps1",
		"task153-real-review-chain-regression.ps1",
	} {
		if !strings.Contains(releaseDriver, want) {
			t.Fatalf("current release driver missing preserved 1.5.2/1.5.3 regression %q", want)
		}
	}

	config := mustRead(filepath.Join(harnessRoot, "harness.template.yaml"))
	if !strings.Contains(config, "workspaceDependencies") {
		t.Fatal("current harness template lost workspaceDependencies contract")
	}
}
