package reviewrules

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestCatalogLoadsExactlyTenSpringV1Rules(t *testing.T) {
	rules, sha, err := LoadCatalog(filepath.Join("..", "..", "..", "review-rules", "spring-v1.yaml"))
	if err != nil {
		t.Fatalf("load spring-v1 catalog: %v", err)
	}
	if len(rules) != 10 {
		t.Fatalf("spring-v1 must contain exactly 10 rules, got %d", len(rules))
	}
	got := make([]string, 0, len(rules))
	for _, rule := range rules {
		got = append(got, rule.ID)
		if rule.Version != 1 || rule.Kind != KindAgent {
			t.Fatalf("rule %s must be version=1 kind=AGENT, got version=%d kind=%s", rule.ID, rule.Version, rule.Kind)
		}
		if strings.TrimSpace(rule.SeverityDefault) == "" || len(rule.Roles) == 0 || len(rule.RequiredEvidence) == 0 || strings.TrimSpace(rule.Prompt) == "" {
			t.Fatalf("rule %s missing required catalog fields: %+v", rule.ID, rule)
		}
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
		t.Fatalf("unexpected spring-v1 rule ids:\n%s", strings.Join(got, "\n"))
	}
	if len(sha) != 64 {
		t.Fatalf("catalog sha must be sha256 hex, got %q", sha)
	}
}

func TestCatalogRejectsDuplicateRuleID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "duplicate.yaml")
	data := `version: 1
rules:
  - id: SPRING-TX-001
    version: 1
    kind: AGENT
    severityDefault: high
    roles: [Service]
    requiredEvidence: [SYMBOL]
    prompt: "检查事务语义。"
  - id: SPRING-TX-001
    version: 1
    kind: AGENT
    severityDefault: high
    roles: [Service]
    requiredEvidence: [SYMBOL]
    prompt: "重复规则。"
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadCatalog(path); err == nil || !strings.Contains(err.Error(), "RULE_CATALOG_DUPLICATE_ID") {
		t.Fatalf("duplicate catalog rule id must fail closed, got %v", err)
	}
}
