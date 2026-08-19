package upgrade

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}
func validConfig(review string) string {
	return "version: 1\nproject:\n  type: maven\n  root: .\n  module: \"\"\n" + review + `integrationTest:
  executable: mvn
  args:
    - test
  reportDir: target/surefire-reports
  timeoutSeconds: 600
service:
  executable: mvn
  args:
    - spring-boot:run
  startupTimeoutSeconds: 120
  readiness:
    type: log
    pattern: Started
  logFile: null
stopService:
  mode: processTree
initialization:
  status: READY
  unresolved: []
scope:
  sourceIncludes:
    - src/main/java/**
  testIncludes:
    - src/test/java/**
write:
  allowedTestPaths:
    - src/test/**
  allowedProductionPaths:
    - src/main/**
  deniedPaths: []
runs:
  directory: .code-harness/runs
`
}
const harnessSchema = `{"type":"object","required":["version","project","review","integrationTest","service","stopService","initialization","scope","write","runs"],"properties":{"review":{"type":"object","required":["baseRef","includeWorkingTree"],"properties":{"baseRef":{"type":"string","minLength":1},"includeWorkingTree":{"type":"boolean"}}}}}`

func makePair(t *testing.T, config string) (string, string) {
	t.Helper()
	root := t.TempDir()
	target := filepath.Join(root, ".code-harness")
	source := filepath.Join(root, ".code-harness-upgrade")
	write(t, target, "VERSION", "1.1.0\n")
	write(t, target, "harness.yaml", config)
	write(t, target, "project.md", "keep-project\n")
	write(t, target, "runs/keep.txt", "keep-run\n")
	write(t, target, "AGENTS.md", "old\n")
	write(t, target, "skills/stale/SKILL.md", "stale\n")
	write(t, target, "bin/codea-harness-tools.exe", "old-runtime")
	write(t, source, "VERSION", "1.1.1\n")
	for _, rel := range []string{"AGENTS.md", "bootstrap.md", "upgrade.md", "harness.template.yaml", "project.template.md", "agents/x.md", "skills/x/SKILL.md", "contracts/upgrade-result.schema.json", "tools/README.md"} {
		write(t, source, rel, "new "+rel+"\n")
	}
	write(t, source, "contracts/harness-config.schema.json", harnessSchema)
	write(t, source, "bin/codea-harness-tools.exe", "new-runtime")
	write(t, source, "bin/ast-grep.exe", "ast-grep")
	return source, target
}

func TestUpgradeReplacesManagedTreeAndReportsStaleFiles(t *testing.T) {
	source, target := makePair(t, validConfig("review:\n  baseRef: origin/develop\n  includeWorkingTree: true\n"))
	result := Run(Options{SourceDir: source, TargetDir: target, Refs: StaticRefs{RemoteBranches: []string{"origin/develop"}}})
	if result.Status != StatusUpgraded {
		t.Fatalf("result=%+v", result)
	}
	if _, err := os.Stat(filepath.Join(target, "skills", "stale", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("stale framework file still exists")
	}
	found := false
	for _, p := range result.RemovedFiles {
		if p == "skills/stale/SKILL.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("removedFiles=%v", result.RemovedFiles)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatal("source package not consumed on success")
	}
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(target), ".code-harness-*"))
	if len(matches) != 0 {
		t.Fatalf("stage/backup leaked: %v", matches)
	}
}
func TestUpgradeFailureRollsBackAndKeepsSource(t *testing.T) {
	source, target := makePair(t, validConfig(""))
	write(t, source, "contracts/harness-config.schema.json", `{"type":"object","required":["mustNotExist"]}`)
	before, _ := os.ReadFile(filepath.Join(target, "AGENTS.md"))
	result := Run(Options{SourceDir: source, TargetDir: target, Refs: StaticRefs{RemoteBranches: []string{"origin/develop"}}})
	if result.Status != StatusUpgradeFailed || !result.RollbackPerformed {
		t.Fatalf("result=%+v", result)
	}
	after, _ := os.ReadFile(filepath.Join(target, "AGENTS.md"))
	if string(after) != string(before) {
		t.Fatal("target not restored")
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatal("source must remain after failure")
	}
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(target), ".code-harness-*"))
	for _, m := range matches {
		if filepath.Clean(m) != filepath.Clean(source) {
			t.Fatalf("failure leaked temp path %s", m)
		}
	}
}
func TestUpgradeAddsReviewAndPreservesExisting(t *testing.T) {
	source, target := makePair(t, validConfig(""))
	r := Run(Options{SourceDir: source, TargetDir: target, Refs: StaticRefs{RemoteBranches: []string{"origin/develop"}}})
	if r.Status != StatusUpgraded {
		t.Fatal(r.Errors)
	}
	b, _ := os.ReadFile(filepath.Join(target, "harness.yaml"))
	if !strings.Contains(string(b), "baseRef: origin/develop") {
		t.Fatal("migration missing")
	}
}
func TestStagedSelfReplacementNeverWritesOverLivePath(t *testing.T) {
	root := t.TempDir()
	dst := filepath.Join(root, "codea-harness-tools.exe")
	staged := filepath.Join(root, "new.tmp")
	write(t, root, "codea-harness-tools.exe", "old")
	write(t, root, "new.tmp", "new")
	if err := replaceRunningExecutable(staged, dst, dst); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(dst)
	if string(b) != "new" {
		t.Fatalf("got %q", b)
	}
}
