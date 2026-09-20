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

func Test152WorkspaceToolsAreGloballyAllowlisted(t *testing.T) {
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

	agents := read(".code-harness/AGENTS.md")
	tools := read(".code-harness/tools/README.md")

	commands := []string{
		"codea-dcep-tools.exe workspace verify --id <id>",
		"codea-dcep-tools.exe nav workspace-inherited --workspace <id> --from <symbol> --method <method>",
		"codea-dcep-tools.exe nav workspace-superclass-call --workspace <id> --from <symbol> --method <method>",
		"codea-dcep-tools.exe nav workspace-template-dispatch --workspace <id> --from <symbol> --hook <hook> [--concrete <class>]",
	}
	for _, command := range commands {
		if !strings.Contains(agents, command) {
			t.Fatalf("AGENTS.md missing workspace Controlled Runtime allowlist command %q", command)
		}
		if !strings.Contains(tools, command) {
			t.Fatalf("tools/README.md missing workspace Controlled Runtime command %q", command)
		}
	}

	toolContracts := []string{
		"workspace_verify(id)",
		"workspace_inherited(workspace, from, method)",
		"workspace_superclass_call(workspace, from, method)",
		"workspace_template_dispatch(workspace, from, hook, concrete?)",
	}
	for _, contract := range toolContracts {
		if !strings.Contains(tools, contract) {
			t.Fatalf("tools/README.md missing workspace tool contract %q", contract)
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
		"precondition: no authoritative change-analysis.json/certificate",
		"change-analysis-draft.json",
		"change-analysis.cert.json",
		"workspace','verify','--id','company-framework",
		"workspace-inherited",
		"workspace-superclass-call",
		"workspace-template-dispatch",
		"WORKSPACE_INHERITANCE",
		"validate','--schema','.code-harness/contracts/change-analysis.schema.json",
		"analysis','certify','--input",
		"chain','discover','--input",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("formal Windows smoke missing real Task 4 certified bootstrap evidence %q", want)
		}
	}
}
