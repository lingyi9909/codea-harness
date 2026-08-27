package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readHarnessContract(t *testing.T, parts ...string) string {
	t.Helper()
	path := filepath.Join(append([]string{"..", "..", ".."}, parts...)...)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Task 4 contract assertions intentionally validate semantics, not Markdown spacing.
func TestTargetedReviewIntentContract(t *testing.T) {
	text := readHarnessContract(t, "agents", "orchestrator.md")
	for _, want := range []string{
		"Runtime ReviewOptions",
		"harness review list",
		"direct TARGETED CLASS",
		"direct TARGETED METHOD",
		"AUTO_FULL",
		"AUTO_SINGLE",
		"USER_SELECTION",
		"全部评审",
		"按业务链评审",
		"仅查看调用链",
		"Controller CLASS",
		"Controller METHOD",
		"自动包含",
		"Service/其他下游 target",
		"不得默认 `ALL`",
		"Review Scope Selection 不等于 Test/Fix Approval",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("orchestrator missing targeted review rule %q", want)
		}
	}
	if strings.Contains(text, "默认 `harness review` 始终是 FULL") {
		t.Fatal("orchestrator still contains superseded plain review always-FULL rule")
	}
}

func TestReviewListSeparatesConfirmedAndUnresolved(t *testing.T) {
	text := readHarnessContract(t, "agents", "orchestrator.md")
	for _, want := range []string{
		"已确认调用链",
		"候选/未解析",
		"不得把 candidate/unresolved 包装成 confirmed",
		"不调用 `review-code`",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("review list contract missing %q", want)
		}
	}
}

func TestAnalyzeChangeDefinesFullAndTargetedCoverage(t *testing.T) {
	text := readHarnessContract(t, "skills", "analyze-change", "SKILL.md")
	for _, want := range []string{
		"FULL",
		"TARGETED",
		"review-scope.schema.json",
		"scopedFiles",
		"selectedCallChains",
		"symbolLocations",
		"exact repository path",
		"Controller CLASS",
		"Controller METHOD",
		"Service/其他下游 target",
		"AUTO_FULL",
		"AUTO_SINGLE",
		"USER_SELECTION",
		"不允许 sampled review",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("analyze-change missing scoped rule %q", want)
		}
	}
}

func TestReviewerDoesNotPromoteTargetedToFullReview(t *testing.T) {
	text := readHarnessContract(t, "agents", "reviewer.md")
	for _, want := range []string{
		"TARGETED",
		"本结论只覆盖本次定向评审范围，不代表整个 Change Set 已完成评审",
		"selectedCallChains",
		"scopedFiles",
		"symbolLocations",
		"exact repository path",
		"Runtime 的 C1..Cn",
		"Controller CLASS",
		"Controller METHOD",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("reviewer missing targeted safety rule %q", want)
		}
	}
}
