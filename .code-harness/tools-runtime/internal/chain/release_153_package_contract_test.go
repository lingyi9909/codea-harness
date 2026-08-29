package chain

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func Test153ReleasePackageKeepsInstalledChainValidateProbe(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "..", ".."))
	workflowPath := filepath.Join(repoRoot, ".github", "workflows", "package-windows-x64.yml")
	data, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read package workflow: %v", err)
	}
	workflow := string(data)
	for _, want := range []string{
		"6f4c050783a7ec21f370799c1a8c69c9b51a9e92",
		"codea-harness-1.6.0-windows-x64-install.zip",
		"codea-harness-1.6.0-windows-x64-upgrade.zip",
		"@('chain','validate')",
		"installed chain validate capability probe failed",
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("current package workflow missing preserved 1.5.3 release contract %q", want)
		}
	}
}
