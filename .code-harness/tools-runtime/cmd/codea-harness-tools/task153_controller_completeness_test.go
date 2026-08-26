package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func task153Task6RepoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", ".."))
}

func task153Task6Read(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(task153Task6RepoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read Task 6 release file %s: %v", rel, err)
	}
	return string(data)
}

func task153Task6RequireContains(t *testing.T, text string, required ...string) {
	t.Helper()
	for _, needle := range required {
		if !strings.Contains(text, needle) {
			t.Fatalf("Task 6 release contract missing %q", needle)
		}
	}
}

func Test153Task6ControllerCompletenessReleaseContract(t *testing.T) {
	workflow := task153Task6Read(t, ".github/workflows/task153-chain-reliability.yml")
	script := task153Task6Read(t, ".github/scripts/task153-real-review-chain-regression.ps1")
	task153Task6RequireContains(t, workflow,
		"runs-on: windows-latest",
		"Task 1 Controller EntryPoint completeness gate",
	)
	task153Task6RequireContains(t, script,
		"CONTROLLER_ENTRYPOINTS 3/3",
		"INCOMPLETE_DRAFT_REJECTED",
		"TASK153_REAL_REVIEW_CHAIN_RELIABILITY PASS",
	)
}
