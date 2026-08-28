package reviewrules

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestSpringV1ContainsExactTenHighValueRules(t *testing.T) {
	rules := loadSpringV1ContractRules160(t)
	got := make([]string, 0, len(rules))
	for _, rule := range rules {
		got = append(got, rule.ID)
	}
	sort.Strings(got)
	want := []string{
		"MYBATIS-BIND-001",
		"MYBATIS-CONTRACT-001",
		"MYBATIS-ISOLATION-001",
		"MYBATIS-SQL-001",
		"SPRING-AUTH-001",
		"SPRING-CONFIG-001",
		"SPRING-TX-001",
		"SPRING-TX-002",
		"SPRING-TX-003",
		"SPRING-VALIDATION-001",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("spring-v1 rule IDs changed:\n%s", strings.Join(got, "\n"))
	}

	prompts := springV1PromptMap160(rules)
	requirePromptFragments160(t, prompts["MYBATIS-SQL-001"], "UPDATE/DELETE", "WHERE", "SQL 证据")
	requirePromptFragments160(t, prompts["MYBATIS-ISOLATION-001"], "tenant", "org", "user", "变更前", "verified contract evidence")
	requirePromptFragments160(t, prompts["MYBATIS-BIND-001"], "${}", "不能把所有 `${}`")
	requirePromptFragments160(t, prompts["MYBATIS-CONTRACT-001"], "statement", "param", "result", "verified resource relation")
	requirePromptFragments160(t, prompts["SPRING-TX-001"], "caller", "callee", "同 Bean", "代理语义")
	requirePromptFragments160(t, prompts["SPRING-TX-002"], "checked exception", "rollback", "默认行为")
	requirePromptFragments160(t, prompts["SPRING-TX-003"], "readOnly", "verified write path")
	requirePromptFragments160(t, prompts["SPRING-CONFIG-001"], "changed key")
}

func TestSpringV1RulesRequireCurrentChangeEvidence(t *testing.T) {
	for _, rule := range loadSpringV1ContractRules160(t) {
		prompt := strings.TrimSpace(rule.Prompt)
		if !containsAny160(prompt, "本次", "当前变更", "changed/new", "changed key") {
			t.Fatalf("rule %s must explicitly bind review to the current change: %q", rule.ID, prompt)
		}
		if !containsAny160(prompt, "证据", "evidence", "verified") {
			t.Fatalf("rule %s must explicitly require evidence: %q", rule.ID, prompt)
		}
		if !strings.Contains(prompt, "证据不足时不提出 Finding Proposal") {
			t.Fatalf("rule %s must forbid evidence-free certainty: %q", rule.ID, prompt)
		}
	}
}

func TestSpringV1HasNoStyleOrNamingRules(t *testing.T) {
	for _, rule := range loadSpringV1ContractRules160(t) {
		upperID := strings.ToUpper(rule.ID)
		if strings.Contains(upperID, "STYLE") || strings.Contains(upperID, "NAMING") || strings.Contains(upperID, "FORMAT") {
			t.Fatalf("style/naming rule is forbidden in spring-v1: %s", rule.ID)
		}
		prompt := rule.Prompt
		for _, forbidden := range []string{"检查命名风格", "检查代码格式", "检查缩进", "检查重复代码", "建议重构"} {
			if strings.Contains(prompt, forbidden) {
				t.Fatalf("rule %s contains forbidden low-value guidance %q: %q", rule.ID, forbidden, prompt)
			}
		}
	}
}

func TestAuthRuleDoesNotRequireOneHardCodedAnnotationName(t *testing.T) {
	prompt := springV1PromptMap160(loadSpringV1ContractRules160(t))["SPRING-AUTH-001"]
	for _, annotation := range []string{"@PreAuthorize", "@Secured", "@RolesAllowed", "@RequiresPermissions"} {
		if strings.Contains(prompt, annotation) {
			t.Fatalf("auth rule must not depend on one hard-coded annotation name %s: %q", annotation, prompt)
		}
	}
	if !strings.Contains(prompt, "verified auth pattern") || !strings.Contains(prompt, "explicit evidence") {
		t.Fatalf("auth rule must require project-local verified auth pattern or explicit evidence: %q", prompt)
	}
	if !strings.Contains(prompt, "固定注解") {
		t.Fatalf("auth rule must explicitly reject fixed-annotation absence as sole evidence: %q", prompt)
	}
}

func TestValidationRuleRequiresVerifiedDangerousPath(t *testing.T) {
	prompt := springV1PromptMap160(loadSpringV1ContractRules160(t))["SPRING-VALIDATION-001"]
	requirePromptFragments160(t, prompt, "verified chain", "有风险操作", "普通 DTO 无注解不能直接报")
}

func loadSpringV1ContractRules160(t *testing.T) []Rule {
	t.Helper()
	path := filepath.Join("..", "..", "..", "review-rules", "spring-v1.yaml")
	rules, _, err := LoadCatalog(path)
	if err != nil {
		t.Fatalf("load spring-v1 catalog: %v", err)
	}
	if len(rules) != 10 {
		t.Fatalf("spring-v1 must contain exactly 10 rules, got %d", len(rules))
	}
	return rules
}

func springV1PromptMap160(rules []Rule) map[string]string {
	out := make(map[string]string, len(rules))
	for _, rule := range rules {
		out[rule.ID] = rule.Prompt
	}
	return out
}

func requirePromptFragments160(t *testing.T, prompt string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		if !strings.Contains(prompt, fragment) {
			t.Fatalf("prompt missing required contract fragment %q: %q", fragment, prompt)
		}
	}
}

func containsAny160(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}
