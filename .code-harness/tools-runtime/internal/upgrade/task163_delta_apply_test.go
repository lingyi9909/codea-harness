package upgrade

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"
)

type fileIdentity struct {
	info    os.FileInfo
	modTime time.Time
}

func snapshotIdentity(t *testing.T, path string) fileIdentity {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := f.Stat()
	closeErr := f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	return fileIdentity{info: info, modTime: info.ModTime()}
}

func assertIdentityUnchanged(t *testing.T, path string, before fileIdentity) {
	t.Helper()
	after := snapshotIdentity(t, path).info
	if !os.SameFile(before.info, after) {
		t.Fatalf("unchanged file was replaced: %s", path)
	}
	if !after.ModTime().Equal(before.modTime) {
		t.Fatalf("unchanged file mtime changed: %s: before=%s after=%s", path, before.modTime, after.ModTime())
	}
}

func makeDeltaPair(t *testing.T) (string, string) {
	t.Helper()
	source, target := makePair(t, validConfig("review:\n  baseRef: origin/develop\n  includeWorkingTree: true\n"))
	if err := removeManaged(target); err != nil {
		t.Fatal(err)
	}
	if err := copyManaged(source, target); err != nil {
		t.Fatal(err)
	}
	write(t, target, "VERSION", "1.1.0\n")
	return source, target
}

func Test163Task4UpgradeDeltaLeavesUnchangedRuntimeAndFrameworkFilesUntouched(t *testing.T) {
	source, target := makeDeltaPair(t)
	runtimePath := filepath.Join(target, "bin", "codea-dcep-tools.exe")
	agentPath := filepath.Join(target, "AGENTS.md")
	runsReadmePath := filepath.Join(target, "runs", "README.md")
	configPath := filepath.Join(target, "harness.yaml")
	write(t, source, "runs/README.md", "managed runs documentation\n")
	write(t, target, "runs/README.md", "managed runs documentation\n")

	fixed := time.Unix(1_700_000_000, 123_000_000)
	for _, path := range []string{runtimePath, agentPath, runsReadmePath, configPath} {
		if err := os.Chtimes(path, fixed, fixed); err != nil {
			t.Fatal(err)
		}
	}
	runtimeBefore := snapshotIdentity(t, runtimePath)
	agentBefore := snapshotIdentity(t, agentPath)
	runsReadmeBefore := snapshotIdentity(t, runsReadmePath)
	configBefore := snapshotIdentity(t, configPath)

	result := Run(Options{SourceDir: source, TargetDir: target, Refs: StaticRefs{RemoteBranches: []string{"origin/develop"}}, RunningExecutable: runtimePath})
	if result.Status != StatusUpgraded {
		t.Fatalf("result=%+v", result)
	}
	assertIdentityUnchanged(t, runtimePath, runtimeBefore)
	assertIdentityUnchanged(t, agentPath, agentBefore)
	assertIdentityUnchanged(t, runsReadmePath, runsReadmeBefore)
	assertIdentityUnchanged(t, configPath, configBefore)

	updated := append([]string(nil), result.UpdatedFiles...)
	sort.Strings(updated)
	if want := []string{"VERSION"}; !reflect.DeepEqual(updated, want) {
		t.Fatalf("updatedFiles=%v want only actual byte changes %v", updated, want)
	}
}

func Test163Task4UpgradeDeltaAddsUpdatesAndRemovesOnlyChangedManagedFiles(t *testing.T) {
	source, target := makeDeltaPair(t)
	write(t, source, "skills/x/SKILL.md", "after! skill\n")
	write(t, target, "skills/x/SKILL.md", "before skill\n")
	write(t, source, "contracts/added.schema.json", "{}\n")
	write(t, target, "tools/removed.txt", "obsolete\n")
	unchangedPath := filepath.Join(target, "AGENTS.md")
	unchangedBefore := snapshotIdentity(t, unchangedPath)
	writeInventory(t, target)
	writeInventory(t, source)

	result := Run(Options{SourceDir: source, TargetDir: target, Refs: StaticRefs{RemoteBranches: []string{"origin/develop"}}})
	if result.Status != StatusUpgraded {
		t.Fatalf("result=%+v", result)
	}
	updated := append([]string(nil), result.UpdatedFiles...)
	sort.Strings(updated)
	wantUpdated := []string{"RELEASE-MANIFEST.json", "VERSION", "contracts/added.schema.json", "skills/x/SKILL.md"}
	if !reflect.DeepEqual(updated, wantUpdated) {
		t.Fatalf("updatedFiles=%v want=%v", updated, wantUpdated)
	}
	if !reflect.DeepEqual(result.RemovedFiles, []string{"tools/removed.txt"}) {
		t.Fatalf("removedFiles=%v", result.RemovedFiles)
	}
	assertIdentityUnchanged(t, unchangedPath, unchangedBefore)
	if got, err := os.ReadFile(filepath.Join(target, "skills", "x", "SKILL.md")); err != nil || string(got) != "after! skill\n" {
		t.Fatalf("updated skill got=%q err=%v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(target, "contracts", "added.schema.json")); err != nil || string(got) != "{}\n" {
		t.Fatalf("added contract got=%q err=%v", got, err)
	}
	if _, err := os.Stat(filepath.Join(target, "tools", "removed.txt")); !os.IsNotExist(err) {
		t.Fatalf("removed managed file still exists: %v", err)
	}
}

func Test163Task4UpgradeDeltaReplacesChangedRunningRuntime(t *testing.T) {
	source, target := makeDeltaPair(t)
	runtimePath := filepath.Join(target, "bin", "codea-dcep-tools.exe")
	write(t, target, "bin/codea-dcep-tools.exe", "installed-runtime")
	before := snapshotIdentity(t, runtimePath)

	result := Run(Options{SourceDir: source, TargetDir: target, Refs: StaticRefs{RemoteBranches: []string{"origin/develop"}}, RunningExecutable: runtimePath})
	if result.Status != StatusUpgraded {
		t.Fatalf("result=%+v", result)
	}
	after := snapshotIdentity(t, runtimePath).info
	if os.SameFile(before.info, after) {
		t.Fatal("changed running Runtime was not replaced")
	}
	if got, err := os.ReadFile(runtimePath); err != nil || string(got) != "new-runtime" {
		t.Fatalf("runtime got=%q err=%v", got, err)
	}
	if !contains(result.UpdatedFiles, "bin/codea-dcep-tools.exe") {
		t.Fatalf("changed Runtime missing from updatedFiles: %v", result.UpdatedFiles)
	}
}

func Test163Task4UpgradeDeltaSupportsManagedDirectoryToFileTransition(t *testing.T) {
	source, target := makeDeltaPair(t)
	write(t, target, "tools/shape/old.txt", "old\n")
	write(t, source, "tools/shape", "new file\n")
	writeInventory(t, target)
	writeInventory(t, source)

	result := Run(Options{SourceDir: source, TargetDir: target, Refs: StaticRefs{RemoteBranches: []string{"origin/develop"}}})
	if result.Status != StatusUpgraded {
		t.Fatalf("result=%+v", result)
	}
	if got, err := os.ReadFile(filepath.Join(target, "tools", "shape")); err != nil || string(got) != "new file\n" {
		t.Fatalf("transition result=%q err=%v", got, err)
	}
}

func Test163Task4UpgradeDeltaSupportsManagedFileToDirectoryTransition(t *testing.T) {
	source, target := makeDeltaPair(t)
	write(t, target, "tools/shape", "old file\n")
	write(t, source, "tools/shape/new.txt", "new\n")
	writeInventory(t, target)
	writeInventory(t, source)

	result := Run(Options{SourceDir: source, TargetDir: target, Refs: StaticRefs{RemoteBranches: []string{"origin/develop"}}})
	if result.Status != StatusUpgraded {
		t.Fatalf("result=%+v", result)
	}
	if got, err := os.ReadFile(filepath.Join(target, "tools", "shape", "new.txt")); err != nil || string(got) != "new\n" {
		t.Fatalf("transition result=%q err=%v", got, err)
	}
}
