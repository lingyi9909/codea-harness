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

func TestTargetedReviewIntentContract(t *testing.T) {
	text := readHarnessContract(t, "agents", "orchestrator.md")
	for _, want := range []string{
		"harness review` → FULL",
		"harness review list` → LIST",
		"harness review <Class>` → TARGETED CLASS",
		"harness review <Class.method>` → TARGETED METHOD",
		"不得默认 `ALL`",
		"Review Scope Selection 不等于 Test/Fix Approval",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("orchestrator missing targeted review rule %q", want)
		}
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
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("reviewer missing targeted safety rule %q", want)
		}
	}
}
