package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readTask4ContractFile(t *testing.T, parts ...string) string {
	t.Helper()
	pathParts := append([]string{"..", "..", ".."}, parts...)
	data, err := os.ReadFile(filepath.Join(pathParts...))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestTask4OrchestratorUsesControlledReviewChainResolution(t *testing.T) {
	text := readTask4ContractFile(t, "agents", "orchestrator.md")
	for _, want := range []string{
		"Review Consumes Verified Chains（1.5 Task 4）",
		"chain review-context --input",
		"STALE_REQUIRES_DECISION",
		"使用本次临时发现的 Chain 继续评审",
		"刷新项目 Chain",
		"停止本次评审",
		"Chain = 业务上下文边界",
		"不得自动保存 DISCOVERED Chain",
		"是否沉淀到项目 `.code-harness/chains/`？",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("Task 4 orchestrator contract missing %q", want)
		}
	}
}

func TestTask4ReviewerConsumesContextWithoutChangingScopeTruth(t *testing.T) {
	text := readTask4ContractFile(t, "agents", "reviewer.md")
	for _, want := range []string{
		"Review Chain Context（1.5 Task 4）",
		"ACCEPTED + VALID",
		"DISCOVERED + TEMPORARY",
		"Chain 不能替代 Change Set",
		"Chain 不能替代 Runtime verified ReviewScopeSelection",
		"STALE Chain 不得静默复用",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("Task 4 reviewer contract missing %q", want)
		}
	}
}

func TestTask4SkillsKeepChainAsContextOnly(t *testing.T) {
	analyze := readTask4ContractFile(t, "skills", "analyze-change", "SKILL.md")
	for _, want := range []string{
		"Review Chain Context（1.5 Task 4）",
		"先完成 ChangeAnalysis",
		"chain review-context --input",
		"不得因为存在 Chain 而减少 changedFiles",
		"不得因为存在 Chain 而减少 scopedFiles",
	} {
		if !strings.Contains(analyze, want) {
			t.Fatalf("Task 4 analyze-change contract missing %q", want)
		}
	}

	review := readTask4ContractFile(t, "skills", "review-code", "SKILL.md")
	for _, want := range []string{
		"Review Chain Context（1.5 Task 4）",
		"chainContext 只提供业务上下文",
		"Finding.file 仍由原 FULL/TARGETED Scope Gate 决定",
		"临时 DISCOVERED Chain 不授权 Project State 写入",
	} {
		if !strings.Contains(review, want) {
			t.Fatalf("Task 4 review-code contract missing %q", want)
		}
	}
}
