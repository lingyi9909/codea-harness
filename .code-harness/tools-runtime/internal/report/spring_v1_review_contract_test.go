package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSpringV1ReviewerGuidanceRequiresDispatchedRuleEvidence(t *testing.T) {
	skill := readTask5ReviewGuidance160(t, "skills", "review-code", "SKILL.md")
	reviewer := readTask5ReviewGuidance160(t, "agents", "reviewer.md")
	combined := skill + "\n" + reviewer

	for _, required := range []string{
		"RuleDispatch",
		"reviewUnitId",
		"ruleId",
		"0..N",
		"Finding Proposal",
		"证据不足",
	} {
		if !strings.Contains(combined, required) {
			t.Fatalf("Task 5 reviewer guidance missing %q", required)
		}
	}
}

func TestSpringV1ReviewerGuidanceForbidsRulePassAndMatcherCertainty(t *testing.T) {
	skill := readTask5ReviewGuidance160(t, "skills", "review-code", "SKILL.md")
	reviewer := readTask5ReviewGuidance160(t, "agents", "reviewer.md")
	combined := skill + "\n" + reviewer

	for _, required := range []string{
		"不得把 rule passed 输出为 Finding",
		"matcher hit 不等于 bug",
		"不得为了覆盖已分发规则而强行提出 Proposal",
	} {
		if !strings.Contains(combined, required) {
			t.Fatalf("Task 5 reviewer guidance missing low-noise rule %q", required)
		}
	}
}

func TestSpringV1ReviewerGuidancePreservesNoiseAndScopeBoundaries(t *testing.T) {
	skill := readTask5ReviewGuidance160(t, "skills", "review-code", "SKILL.md")
	reviewer := readTask5ReviewGuidance160(t, "agents", "reviewer.md")
	combined := skill + "\n" + reviewer

	for _, forbiddenFinding := range []string{
		"命名",
		"格式",
		"缩进",
		"重复代码",
		"建议重构",
		"未变化配置",
		"scope 外潜在问题",
		"workspace dependency",
	} {
		if !strings.Contains(combined, forbiddenFinding) {
			t.Fatalf("Task 5 guidance must explicitly preserve forbidden-noise boundary %q", forbiddenFinding)
		}
	}
	if !strings.Contains(combined, "TEST_VALIDITY") {
		t.Fatal("Task 5 guidance must preserve existing Test Validity boundary")
	}
}

func readTask5ReviewGuidance160(t *testing.T, parts ...string) string {
	t.Helper()
	pathParts := append([]string{"..", "..", ".."}, parts...)
	data, err := os.ReadFile(filepath.Join(pathParts...))
	if err != nil {
		t.Fatalf("read Task 5 guidance %s: %v", filepath.Join(parts...), err)
	}
	return string(data)
}
