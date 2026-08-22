package upgrade

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func make14Pair(t *testing.T, config string) (string, string) {
	t.Helper()
	source, target := makePair(t, config)
	write(t, target, "VERSION", "1.3.2\n")
	write(t, source, "VERSION", "1.4.0\n")

	schemaBytes, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "harness-config.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, source, "contracts/harness-config.schema.json", string(schemaBytes))

	for _, rel := range []string{
		"contracts/review-scope.schema.json",
		"contracts/apply-request.schema.json",
		"contracts/apply-result.schema.json",
		"tools-runtime/internal/reviewscope/reviewscope.go",
		"tools-runtime/internal/apply/apply.go",
		"tools-runtime/internal/report/review.go",
	} {
		write(t, source, rel, "1.4 "+rel+"\n")
	}
	return source, target
}

func TestUpgrade132To140MigratesResourceScopesAndPreservesProjectState(t *testing.T) {
	config := validConfig("review:\n  baseRef: origin/release-user-custom\n  includeWorkingTree: false\n")
	config = strings.Replace(config, "version: 1\n", "version: 1\n# keep-user-comment\n", 1)
	config = strings.Replace(config, "timeoutSeconds: 600", "timeoutSeconds: 777", 1)
	config = strings.Replace(config, "    - src/main/java/**\n", "    - modules/order/src/main/java/**\n", 1)
	config = strings.Replace(config, "    - src/test/java/**\n", "    - modules/order/src/test/java/**\n", 1)

	source, target := make14Pair(t, config)
	originalProject := []byte("# user project notes\nmodule=order-service\n")
	originalDatabase := []byte("version: 1\nenvironment: TEST\npassword: keep-secret\n")
	originalRun := []byte("keep-run-evidence\n")
	if err := os.WriteFile(filepath.Join(target, "project.md"), originalProject, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "database.yaml"), originalDatabase, 0o600); err != nil {
		t.Fatal(err)
	}
	write(t, target, "runs/run-140/evidence.json", string(originalRun))
	write(t, target, "skills/stale-132/SKILL.md", "stale 1.3.2 framework\n")

	result := Run(Options{SourceDir: source, TargetDir: target, Refs: StaticRefs{RemoteBranches: []string{"origin/develop"}}})
	if result.Status != StatusUpgraded {
		t.Fatalf("result=%+v", result)
	}
	if result.FromVersion != "1.3.2" || result.ToVersion != "1.4.0" {
		t.Fatalf("unexpected versions: %+v", result)
	}
	if !contains(result.Migrations, "upgrade-config-v1-to-v2-resource-scopes") {
		t.Fatalf("missing 1.4 config migration marker: %v", result.Migrations)
	}

	gotConfigBytes, err := os.ReadFile(filepath.Join(target, "harness.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	gotConfig := string(gotConfigBytes)
	for _, want := range []string{
		"version: 2",
		"# keep-user-comment",
		"baseRef: origin/release-user-custom",
		"includeWorkingTree: false",
		"timeoutSeconds: 777",
		"- modules/order/src/main/java/**",
		"- modules/order/src/test/java/**",
		"mapperIncludes:\n    - src/main/resources/**/*Mapper.xml",
		"configIncludes:\n    - src/main/resources/**/*.yml",
	} {
		if !strings.Contains(gotConfig, want) {
			t.Fatalf("migrated harness.yaml missing %q:\n%s", want, gotConfig)
		}
	}

	for path, want := range map[string][]byte{
		"project.md":                    originalProject,
		"database.yaml":                 originalDatabase,
		"runs/run-140/evidence.json":    originalRun,
	} {
		got, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("read preserved %s: %v", path, err)
		}
		if string(got) != string(want) {
			t.Fatalf("project state %s changed: got %q want %q", path, got, want)
		}
	}

	for _, rel := range []string{
		"contracts/review-scope.schema.json",
		"contracts/apply-request.schema.json",
		"contracts/apply-result.schema.json",
		"tools-runtime/internal/reviewscope/reviewscope.go",
		"tools-runtime/internal/apply/apply.go",
		"tools-runtime/internal/report/review.go",
	} {
		if _, err := os.Stat(filepath.Join(target, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("1.4 framework path missing %s: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(target, "skills", "stale-132", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("stale 1.3.2 framework file survived 1.4 replace")
	}
	if !contains(result.RemovedFiles, "skills/stale-132/SKILL.md") {
		t.Fatalf("stale path missing from removedFiles: %v", result.RemovedFiles)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatal("upgrade source must be consumed after successful 1.4 upgrade")
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(target), ".code-harness-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("stage/backup leaked after 1.4 upgrade: %v", matches)
	}
}
