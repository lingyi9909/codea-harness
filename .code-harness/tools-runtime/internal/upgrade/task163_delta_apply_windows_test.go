//go:build windows

package upgrade

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func Test163Task4WindowsChangedInstalledRunningRuntimeIsReplaced(t *testing.T) {
	if os.Getenv("CODEA_163_INSTALLED_RUNTIME_HELPER") == "1" {
		result := Run(Options{
			SourceDir:         os.Getenv("CODEA_163_SOURCE"),
			TargetDir:         os.Getenv("CODEA_163_TARGET"),
			Refs:              StaticRefs{RemoteBranches: []string{"origin/develop"}},
			RunningExecutable: os.Args[0],
		})
		if result.Status != StatusUpgraded || !contains(result.UpdatedFiles, "bin/codea-dcep-tools.exe") {
			t.Fatalf("installed Runtime upgrade failed: %+v", result)
		}
		return
	}

	source, target := makeDeltaPair(t)
	testExe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	installedExe := filepath.Join(target, "bin", "codea-dcep-tools.exe")
	testBytes, err := os.ReadFile(testExe)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installedExe, testBytes, 0o755); err != nil {
		t.Fatal(err)
	}
	wantRuntime, err := os.ReadFile(filepath.Join(source, "bin", "codea-dcep-tools.exe"))
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(installedExe, "-test.run=^Test163Task4WindowsChangedInstalledRunningRuntimeIsReplaced$")
	cmd.Env = append(os.Environ(),
		"CODEA_163_INSTALLED_RUNTIME_HELPER=1",
		"CODEA_163_SOURCE="+source,
		"CODEA_163_TARGET="+target,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("installed Runtime helper failed: %v\n%s", err, out)
	}
	gotRuntime, err := os.ReadFile(installedExe)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotRuntime) != string(wantRuntime) {
		t.Fatalf("installed running Runtime bytes were not updated")
	}
}

func Test163Task4WindowsUnchangedRunningRuntimeIsNeverReplaced(t *testing.T) {
	source, target := makeDeltaPair(t)
	runtimePath := filepath.Join(target, "bin", "codea-dcep-tools.exe")
	before := snapshotIdentity(t, runtimePath)
	runtimePath16, err := syscall.UTF16PtrFromString(runtimePath)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(
		runtimePath16,
		syscall.GENERIC_READ,
		syscall.FILE_SHARE_READ,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(handle)

	result := Run(Options{SourceDir: source, TargetDir: target, Refs: StaticRefs{RemoteBranches: []string{"origin/develop"}}, RunningExecutable: runtimePath})
	if result.Status != StatusUpgraded {
		t.Fatalf("unchanged locked Runtime must be skipped: result=%+v", result)
	}
	assertIdentityUnchanged(t, runtimePath, before)
}

func Test163Task4WindowsApplyFailureAfterRuntimeUpdateRollsBackAllBytes(t *testing.T) {
	source, target := makePair(t, validConfig("review:\n  baseRef: origin/develop\n  includeWorkingTree: true\n"))
	write(t, source, "VERSION", "1.6.3\n")
	write(t, target, "AGENTS.md", "installed agents\n")
	write(t, target, "bin/codea-dcep-tools.exe", "installed runtime\n")
	writeInventory(t, target)
	writeInventory(t, source)
	agentPath := filepath.Join(target, "AGENTS.md")
	runtimePath := filepath.Join(target, "bin", "codea-dcep-tools.exe")
	configPath := filepath.Join(target, "harness.yaml")
	manifestPath := filepath.Join(target, "RELEASE-MANIFEST.json")
	wantAgent, _ := os.ReadFile(agentPath)
	wantRuntime, _ := os.ReadFile(runtimePath)
	wantConfig, _ := os.ReadFile(configPath)
	wantManifest, _ := os.ReadFile(manifestPath)
	runtimeBefore := snapshotIdentity(t, runtimePath)

	configPath16, err := syscall.UTF16PtrFromString(configPath)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(
		configPath16,
		syscall.GENERIC_READ,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_DELETE,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(handle)

	result := Run(Options{
		SourceDir:         source,
		TargetDir:         target,
		Refs:              StaticRefs{RemoteBranches: []string{"origin/develop"}},
		RunningExecutable: runtimePath,
	})
	if result.Status != StatusUpgradeFailed || !result.RollbackPerformed {
		t.Fatalf("result=%+v", result)
	}
	if !contains(result.Migrations, "upgrade-config-v1-to-v2-resource-scopes") {
		t.Fatalf("expected config migration before locked write failure: %+v", result)
	}
	if len(result.Errors) == 0 || !strings.Contains(strings.ToLower(result.Errors[0]), "harness.yaml") {
		t.Fatalf("expected harness.yaml write failure after managed apply: %+v", result.Errors)
	}
	runtimeAfter := snapshotIdentity(t, runtimePath).info
	if os.SameFile(runtimeBefore.info, runtimeAfter) {
		t.Fatal("Runtime identity unchanged: failure did not occur after Runtime update and rollback")
	}
	for path, want := range map[string][]byte{
		agentPath:    wantAgent,
		runtimePath:  wantRuntime,
		configPath:   wantConfig,
		manifestPath: wantManifest,
	} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Fatalf("rollback changed %s: got=%q want=%q", path, got, want)
		}
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("failed upgrade consumed source: %v", err)
	}
}

func Test163Task4WindowsInventoryCollisionIsCaseInsensitive(t *testing.T) {
	source, target := makeDeltaPair(t)
	writeInventory(t, target)
	write(t, target, "tools/user.txt", "user-owned")
	write(t, source, "tools/User.txt", "package-owned")
	writeInventory(t, source)

	result := Run(Options{SourceDir: source, TargetDir: target})
	if result.Status != StatusManualActionRequired {
		t.Fatalf("case-insensitive unowned collision accepted: %+v", result)
	}
	if got, err := os.ReadFile(filepath.Join(target, "tools", "user.txt")); err != nil || string(got) != "user-owned" {
		t.Fatalf("unowned Windows path changed: %q %v", got, err)
	}
}
