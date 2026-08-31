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
	read := func(rel string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		return string(data)
	}

	workflow := read(".github/workflows/package-windows-x64.yml")
	releaseDriver := read(".github/scripts/task161-release.ps1")
	chainRegression := read(".github/scripts/task153-real-review-chain-regression.ps1")

	if !strings.Contains(workflow, "task161-release.ps1") {
		t.Fatal("current package workflow missing 1.6.1 release driver")
	}
	if !strings.Contains(releaseDriver, "task153-real-review-chain-regression.ps1") {
		t.Fatal("current release driver missing preserved Task153 regression")
	}
	for _, want := range []string{
		"./internal/chain",
		"CHAIN_CANDIDATE_TAMPER_REJECTED",
		"CHAIN_EDIT_VERIFIED",
		"TASK153_REAL_REVIEW_CHAIN_RELIABILITY PASS",
	} {
		if !strings.Contains(chainRegression, want) {
			t.Fatalf("Task153 regression missing preserved chain contract %q", want)
		}
	}
}
