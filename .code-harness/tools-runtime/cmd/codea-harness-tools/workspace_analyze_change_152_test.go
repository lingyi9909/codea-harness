package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func Test152AnalyzeChangeDeclaresVerifiedWorkspaceFallback(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", "..", "..", ".."))

	read := func(rel string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}

	analyze := read(".code-harness/skills/analyze-change/SKILL.md")
	for _, want := range []string{
		"workspace_verify",
		"workspace_inherited",
		"workspace_superclass_call",
		"workspace_template_dispatch",
		"只读取显式 `harness.yaml.workspaceDependencies`",
		"workspace verify <id>",
		"只有 `VERIFIED` 才允许 workspace navigation",
		"workspace-inherited",
		"workspace-superclass-call",
		"workspace-template-dispatch",
		"source=WORKSPACE_INHERITANCE",
		"继续 confirmed callChain",
		"不得扫描任意 sibling",
	} {
		if !strings.Contains(analyze, want) {
			t.Fatalf("analyze-change missing workspace bootstrap contract %q", want)
		}
	}

	reviewer := read(".code-harness/agents/reviewer.md")
	for _, want := range []string{
		"workspace dependency 只允许作为 Navigation / Chain Context",
		"不得进入 `reviewCoverage.reviewedFiles`",
		"不得进入 Review Scope",
		"不得产生 Finding",
	} {
		if !strings.Contains(reviewer, want) {
			t.Fatalf("reviewer missing workspace review-isolation contract %q", want)
		}
	}
}

func Test152WindowsSmokeContainsRealAnalyzeChangeDiscoverBootstrap(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", "..", "..", ".."))
	data, err := os.ReadFile(filepath.Join(repoRoot, ".github", "scripts", "task152-workspace-smoke.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, want := range []string{
		"TASK4_WORKSPACE_ANALYZE_CHANGE_BOOTSTRAP",
		"harness chain discover XxxController",
		"precondition: no change-analysis.json",
		"workspace','verify','--id','company-framework",
		"workspace-inherited",
		"workspace-superclass-call",
		"workspace-template-dispatch",
		"WORKSPACE_INHERITANCE",
		"change-analysis.json",
		"validate','--schema','.code-harness/contracts/change-analysis.schema.json",
		"chain','discover','--input",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("formal Windows smoke missing real Task 4 bootstrap evidence %q", want)
		}
	}
}
