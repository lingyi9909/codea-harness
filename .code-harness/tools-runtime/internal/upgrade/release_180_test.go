package upgrade

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const (
	primaryAgent180 = ".opencode/agents/orchestrator.md"
	primaryTool180  = ".opencode/tools/codea-review.ts"
)

func write180HostInventory(t *testing.T, root, hostRoot string) {
	t.Helper()
	writeInventory(t, root)
	data, err := os.ReadFile(filepath.Join(root, manifestPath))
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	fields := [][3]string{
		{"path", "upgradeSource", "sha256"},
		{"command", "commandUpgradeSource", "commandSha256"},
		{"submissionTool", "submissionToolUpgradeSource", "submissionToolSha256"},
		{"primaryCommand", "primaryCommandUpgradeSource", "primaryCommandSha256"},
		{"primaryAgent", "primaryAgentUpgradeSource", "primaryAgentSha256"},
		{"primaryTool", "primaryToolUpgradeSource", "primaryToolSha256"},
	}
	host := map[string]string{"host": "opencode", "mode": "subagent"}
	files := reviewerHostFilesForVersion("1.8.0")
	if len(files) != len(fields) {
		t.Fatalf("1.8 Host files=%d fields=%d", len(files), len(fields))
	}
	for i, rel := range files {
		hash, err := hashFile(filepath.Join(hostRoot, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		host[fields[i][0]] = rel
		host[fields[i][1]] = "host/" + rel
		host[fields[i][2]] = hash
	}
	manifest["hostAgents"] = map[string]any{"reviewer": host}
	data, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, manifestPath, string(data))
}

func make167To180Pair(t *testing.T) (string, string) {
	t.Helper()
	source167, target := makeHostUpgrade167Pair(t)
	result := Run(Options{SourceDir: source167, TargetDir: target})
	if result.Status != StatusUpgraded {
		t.Fatalf("prepare 1.6.7 baseline: %+v", result)
	}

	projectRoot := filepath.Dir(target)
	source := filepath.Join(projectRoot, ".code-harness-upgrade-180")
	if err := copyTree(target, source, nil); err != nil {
		t.Fatal(err)
	}
	write(t, source, "VERSION", "1.8.0\n")
	write(t, source, "AGENTS.md", "1.8 framework\n")

	for _, rel := range reviewerHostFilesForVersion("1.8.0") {
		var data []byte
		var err error
		switch rel {
		case primaryCommand167:
			data, err = os.ReadFile(filepath.Join("..", "..", "..", "commands", "harness-review.md"))
		case primaryAgent180:
			data, err = os.ReadFile(filepath.Join("..", "..", "..", "agents", "orchestrator.md"))
		case primaryTool180:
			data, err = os.ReadFile(filepath.Join("..", "..", "..", "tools", "codea-review.ts"))
		default:
			data, err = os.ReadFile(filepath.Join(projectRoot, filepath.FromSlash(rel)))
		}
		if err != nil {
			t.Fatalf("read 1.8 Host %s: %v", rel, err)
		}
		write(t, source, "host/"+rel, string(data))
	}
	write180HostInventory(t, source, filepath.Join(source, "host"))
	return source, target
}

func Test180UpgradeInstallsPrimaryReviewHostAndPreservesUnknownFiles(t *testing.T) {
	source, target := make167To180Pair(t)
	projectRoot := filepath.Dir(target)
	write(t, projectRoot, ".opencode/user-keep.json", "keep host user file")
	write(t, target, "tools/user-keep.txt", "keep framework-adjacent user file")

	preserved := map[string][]byte{}
	for _, path := range []string{
		filepath.Join(projectRoot, ".opencode/user-keep.json"),
		filepath.Join(target, "tools/user-keep.txt"),
		filepath.Join(target, "harness.yaml"),
		filepath.Join(target, "project.md"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		preserved[path] = data
	}

	result := Run(Options{SourceDir: source, TargetDir: target})
	if result.Status != StatusUpgraded || result.FromVersion != "1.6.7" || result.ToVersion != "1.8.0" {
		t.Fatalf("upgrade: %+v", result)
	}
	for _, rel := range []string{primaryCommand167, primaryAgent180, primaryTool180} {
		got, err := os.ReadFile(filepath.Join(projectRoot, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("installed Host missing %s: %v", rel, err)
		}
		if len(got) == 0 {
			t.Fatalf("installed Host empty %s", rel)
		}
		if !contains(result.UpdatedFiles, rel) {
			t.Fatalf("Host update evidence missing %s: %+v", rel, result.UpdatedFiles)
		}
	}
	for path, want := range preserved {
		got, err := os.ReadFile(path)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("preserved file changed %s err=%v", path, err)
		}
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("successful upgrade did not consume source: %v", err)
	}
}

func Test180UpgradeRejectsUserPrimaryToolConflictBeforeFrameworkWrite(t *testing.T) {
	source, target := make167To180Pair(t)
	projectRoot := filepath.Dir(target)
	write(t, projectRoot, primaryTool180, "user-owned tool")
	beforeFramework := snapshotRollbackTree(t, target)
	beforeHost := snapshotRollbackTree(t, filepath.Join(projectRoot, ".opencode"))

	result := Run(Options{SourceDir: source, TargetDir: target})
	if result.Status != StatusManualActionRequired || result.RollbackPerformed {
		t.Fatalf("conflict accepted: %+v", result)
	}
	if !reflect.DeepEqual(beforeFramework, snapshotRollbackTree(t, target)) ||
		!reflect.DeepEqual(beforeHost, snapshotRollbackTree(t, filepath.Join(projectRoot, ".opencode"))) {
		t.Fatal("Host conflict modified installation")
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("rejected upgrade consumed source: %v", err)
	}
}

func Test180UpgradeRollsBackFrameworkAndPartialPrimaryHost(t *testing.T) {
	source, target := make167To180Pair(t)
	projectRoot := filepath.Dir(target)
	beforeFramework := snapshotRollbackTree(t, target)
	beforeHost := snapshotRollbackTree(t, filepath.Join(projectRoot, ".opencode"))
	previous := reviewerHostInstallHook
	reviewerHostInstallHook = func(_ int, rel string) error {
		if rel == primaryTool180 {
			return errors.New("injected 1.8 primary tool failure")
		}
		return nil
	}
	t.Cleanup(func() { reviewerHostInstallHook = previous })

	result := Run(Options{SourceDir: source, TargetDir: target})
	if result.Status != StatusUpgradeFailed || !result.RollbackPerformed {
		t.Fatalf("rollback: %+v", result)
	}
	if !reflect.DeepEqual(beforeFramework, snapshotRollbackTree(t, target)) ||
		!reflect.DeepEqual(beforeHost, snapshotRollbackTree(t, filepath.Join(projectRoot, ".opencode"))) {
		t.Fatal("rollback did not restore framework and Host")
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("failed upgrade consumed source: %v", err)
	}
}

func Test180SameVersionIsNoopBeforeHostMutation(t *testing.T) {
	source, target := make167To180Pair(t)
	first := Run(Options{SourceDir: source, TargetDir: target})
	if first.Status != StatusUpgraded {
		t.Fatalf("initial upgrade: %+v", first)
	}

	projectRoot := filepath.Dir(target)
	source2 := filepath.Join(projectRoot, ".code-harness-upgrade-180-noop")
	if err := copyTree(target, source2, nil); err != nil {
		t.Fatal(err)
	}
	write(t, source2, "host/"+primaryTool180, "would be invalid if inspected")
	beforeFramework := snapshotRollbackTree(t, target)
	beforeHost := snapshotRollbackTree(t, filepath.Join(projectRoot, ".opencode"))

	result := Run(Options{SourceDir: source2, TargetDir: target})
	if result.Status != StatusAlreadyUpToDate {
		t.Fatalf("same-version result: %+v", result)
	}
	if !reflect.DeepEqual(beforeFramework, snapshotRollbackTree(t, target)) ||
		!reflect.DeepEqual(beforeHost, snapshotRollbackTree(t, filepath.Join(projectRoot, ".opencode"))) {
		t.Fatal("same-version noop changed installation")
	}
}
