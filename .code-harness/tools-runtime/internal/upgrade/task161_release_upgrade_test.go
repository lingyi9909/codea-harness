package upgrade

import (
	"os"
	"path/filepath"
	"testing"
)

func Test161UpgradeRequiresRenamedRuntime(t *testing.T) {
	var hasNew, hasOld bool
	for _, rel := range requiredSource {
		if rel == "bin/codea-dcep-tools.exe" {
			hasNew = true
		}
		if rel == "bin/codea-harness-tools.exe" {
			hasOld = true
		}
	}
	if !hasNew {
		t.Fatal("1.6.1 upgrade package must require bin/codea-dcep-tools.exe")
	}
	if hasOld {
		t.Fatal("1.6.1 upgrade package must not require legacy bin/codea-harness-tools.exe")
	}
}

func Test161ApplyStagedRemovesLegacyRuntimeAndInstallsRenamedRuntime(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	stage := filepath.Join(root, "stage")
	for _, dir := range []string{filepath.Join(target, "bin"), filepath.Join(stage, "bin")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// applyStaged operates on a fully prepared stage, which includes preserved
	// Project State as well as Framework Managed files. Keep this fixture faithful
	// to that contract instead of constructing a bin-only stage.
	config := []byte("version: 2\nproject:\n  type: maven\n  root: .\nreview:\n  baseRef: origin/develop\n  includeWorkingTree: true\n")
	for _, base := range []string{target, stage} {
		if err := os.WriteFile(filepath.Join(base, "harness.yaml"), config, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(target, "bin", "codea-harness-tools.exe"), []byte("legacy"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stage, "bin", "codea-dcep-tools.exe"), []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := applyStaged(stage, target, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "bin", "codea-harness-tools.exe")); !os.IsNotExist(err) {
		t.Fatalf("legacy runtime still exists after staged apply: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(target, "bin", "codea-dcep-tools.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("renamed runtime content mismatch: %q", got)
	}
}
