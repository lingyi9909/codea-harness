package upgrade

import (
	"os"
	"path/filepath"
	"testing"
)

func make13Pair(t *testing.T, config string) (string, string) {
	t.Helper()
	source, target := makePair(t, config)
	write(t, target, "VERSION", "1.2.0\n")
	write(t, source, "VERSION", "1.3.0\n")

	for _, rel := range []string{
		"database.template.yaml",
		"contracts/database-config.schema.json",
		"contracts/database-evidence.schema.json",
		"contracts/test-target-selection.schema.json",
		"skills/select-test-targets/SKILL.md",
		"skills/query-database/SKILL.md",
	} {
		write(t, target, rel, "1.2 "+rel+"\n")
		write(t, source, rel, "1.3 "+rel+"\n")
	}

	for _, rel := range []string{
		"agents/api-doc-agent.md",
		"skills/discover-api/SKILL.md",
		"skills/generate-api-doc/SKILL.md",
		"contracts/api-doc.schema.json",
		"tools-runtime/internal/report/review.go",
		"tools-runtime/internal/report/apidoc.go",
		"tools-runtime/internal/nav/extended.go",
	} {
		write(t, source, rel, "1.3 "+rel+"\n")
	}
	return source, target
}

func TestUpgrade120To130InstallsNewFrameworkAndPreservesProjectState(t *testing.T) {
	source, target := make13Pair(t, validConfig("review:\n  baseRef: origin/develop\n  includeWorkingTree: true\n"))
	originalDB := []byte("version: 1\nenvironment: TEST\npassword: sentinel-secret\n")
	if err := os.WriteFile(filepath.Join(target, "database.yaml"), originalDB, 0o600); err != nil {
		t.Fatal(err)
	}
	write(t, target, "skills/stale/SKILL.md", "stale\n")

	result := Run(Options{SourceDir: source, TargetDir: target, Refs: StaticRefs{RemoteBranches: []string{"origin/develop"}}})
	if result.Status != StatusUpgraded {
		t.Fatalf("result=%+v", result)
	}
	if result.FromVersion != "1.2.0" || result.ToVersion != "1.3.0" {
		t.Fatalf("unexpected versions: %+v", result)
	}

	gotDB, err := os.ReadFile(filepath.Join(target, "database.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(gotDB) != string(originalDB) {
		t.Fatalf("database.yaml changed: got %q want %q", gotDB, originalDB)
	}

	for _, rel := range []string{
		"agents/api-doc-agent.md",
		"skills/discover-api/SKILL.md",
		"skills/generate-api-doc/SKILL.md",
		"contracts/api-doc.schema.json",
		"tools-runtime/internal/report/review.go",
		"tools-runtime/internal/report/apidoc.go",
		"tools-runtime/internal/nav/extended.go",
	} {
		if _, err := os.Stat(filepath.Join(target, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("1.3 framework path missing %s: %v", rel, err)
		}
	}

	if _, err := os.Stat(filepath.Join(target, "skills", "stale", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("stale framework file survived 1.2.0 -> 1.3.0 replace")
	}
	if !contains(result.RemovedFiles, "skills/stale/SKILL.md") {
		t.Fatalf("stale path missing from removedFiles: %v", result.RemovedFiles)
	}
	if !contains(result.PreservedFiles, "database.yaml") ||
		!contains(result.PreservedFiles, "project.md") ||
		!contains(result.PreservedFiles, "runs/**") {
		t.Fatalf("project state missing from preservedFiles: %v", result.PreservedFiles)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatal("source package must be consumed after successful upgrade")
	}
}
