package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func Test163Task2ActiveContractsSplitProjectFromAffectedDiscovery(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate Task 2 contract test source")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", "..", "..", ".."))
	read := func(rel string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		return string(data)
	}

	reviewer := read(".code-harness/agents/reviewer.md")
	for _, want := range []string{
		"公开 `harness chain discover [target]` 固定走 **PROJECT DISCOVERY**",
		"不执行 `analysis snapshot` / `analyze-change` / ChangeAnalysis certification",
		"Review 内部 **AFFECTED DISCOVERY**",
	} {
		if !strings.Contains(reviewer, want) {
			t.Fatalf("Reviewer Task 2 authority split missing %q", want)
		}
	}
	if strings.Contains(reviewer, "对于 `harness chain discover [target]`，Reviewer 继续协调 `analyze-change → discover-chain → Controlled Runtime`") {
		t.Fatal("Reviewer still binds public PROJECT discovery to analyze-change")
	}

	discoverSkill := read(".code-harness/skills/discover-chain/SKILL.md")
	for _, want := range []string{
		"harness chain discover OrderController",
		"harness chain discover OrderController.approve",
		"PROJECT DISCOVERY 不依赖非空 Git Change Set",
		"AFFECTED DISCOVERY — Review Internal Only",
	} {
		if !strings.Contains(discoverSkill, want) {
			t.Fatalf("discover-chain Task 2 contract missing %q", want)
		}
	}
}
