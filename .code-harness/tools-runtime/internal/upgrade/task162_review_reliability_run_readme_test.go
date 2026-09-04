package upgrade

import (
	"os"
	"path/filepath"
	"testing"
)

func Test162ReviewReliabilityRunReadmeManagedForUpgrade(t *testing.T) {
	if isProjectState("runs/README.md") {
		t.Fatal("runs/README.md must be framework-managed documentation, not Project State")
	}
	if !isManaged("runs/README.md") {
		t.Fatal("runs/README.md must be managed so upgrades install the Task 3 documentation")
	}

	for _, rel := range []string{
		"runs/keep.bin",
		"runs/run-1/review.md",
		"runs/run-1/analysis/change-set.json",
	} {
		if !isProjectState(rel) {
			t.Fatalf("%s must remain Project State", rel)
		}
		if isManaged(rel) {
			t.Fatalf("%s must never become framework-managed", rel)
		}
	}
}

func Test162ReviewReliabilityCopyManagedCopiesOnlyRunReadme(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(src, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("runs/README.md", "Task 3 Review Run documentation")
	write("runs/run-1/review.md", "local run state")
	write("runs/run-1/analysis/change-set.json", "{}")

	if err := copyManaged(src, dst); err != nil {
		t.Fatalf("copyManaged failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dst, "runs", "README.md")); err != nil {
		t.Fatalf("managed runs/README.md was not copied: %v", err)
	}
	for _, rel := range []string{"runs/run-1/review.md", "runs/run-1/analysis/change-set.json"} {
		if _, err := os.Stat(filepath.Join(dst, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Fatalf("Project State leaked through copyManaged: %s", rel)
		}
	}
}
