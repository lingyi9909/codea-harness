package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func Test152Task5RequiresRealDualProjectWindowsE2E(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", "..", "..", ".."))

	scriptPath := filepath.Join(repoRoot, ".github", "scripts", "task152-task5-real-business-regression.ps1")
	scriptBytes, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("Task 5 real Windows E2E driver missing: %v", err)
	}
	script := string(scriptBytes)
	for _, want := range []string{
		"TASK5_REAL_DUAL_PROJECT_BUSINESS_REGRESSION",
		"STAGED",
		"UNSTAGED",
		"UNTRACKED",
		"find-symbol",
		"find-references",
		"find-implementations",
		"workspace','verify",
		"workspace-inherited",
		"workspace-superclass-call",
		"workspace-template-dispatch",
		"XxxService.submit",
		"AbstractTemplate.execute",
		"XxxServiceImpl.doExecute",
		"XxxMapper.updateStatus",
		"XxxMapper.xml",
		"change-analysis.schema.json",
		"reviewCoverage",
		"chain','discover",
		"WORKSPACE_DEPENDENCY_VERSION_MISMATCH",
		"WORKSPACE_DEPENDENCY_NOT_CONFIGURED",
		"AMBIGUOUS_TEMPLATE_DISPATCH",
		"WORKSPACE_DEPENDENCY_SOURCE_NOT_FOUND",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("Task 5 real Windows E2E driver missing evidence %q", want)
		}
	}

	workflowBytes, err := os.ReadFile(filepath.Join(repoRoot, ".github", "workflows", "task152-workspace-navigation.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(workflowBytes)
	for _, want := range []string{
		"Task 5 real dual-project business regression",
		"task152-task5-real-business-regression.ps1",
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("Task 5 workflow gate missing %q", want)
		}
	}
}
