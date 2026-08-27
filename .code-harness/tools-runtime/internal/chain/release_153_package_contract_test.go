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
		"6f290d8ff160767bb981278aa123aa1621ea3343",
		"codea-harness-1.5.3-windows-x64-install.zip",
		"codea-harness-1.5.3-windows-x64-upgrade.zip",
		"@('chain','validate')",
		"installed chain validate capability probe failed",
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("1.5.3 package workflow missing release contract %q", want)
		}
	}
}
