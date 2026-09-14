package knowledge

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validBinding170() []byte {
	return []byte("version: 1\nprojectId: order-service\nsources:\n  - id: order-rule\n    root: PROJECT\n    path: docs/business/order.md\n    kind: RULES\n    required: true\n")
}

func activeRule170(ruleID, projectID, selector string) []byte {
	return []byte(fmt.Sprintf(`---
ruleId: '%s'
projectId: '%s'
status: ACTIVE
version: "1"
owner: order-team
source: requirements/order.md
approvalRef: approvals/ORDER-APPROVED
appliesTo:
  paths: ['%s']
  entryPoints: []
supersedes: []
exceptions: []
---
Orders must preserve the approved state transition.
`, ruleID, projectID, selector))
}

func Test170KnowledgeParseBindingStrict(t *testing.T) {
	binding, digest, err := ParseBinding170(validBinding170())
	if err != nil { t.Fatal(err) }
	if binding.ProjectID != "order-service" || len(binding.Sources) != 1 || len(digest) != 64 { t.Fatalf("unexpected binding: %+v digest=%q", binding, digest) }

	cases := map[string]string{
		"duplicate-source": "version: 1\nprojectId: order-service\nsources:\n  - {id: x, root: PROJECT, path: docs/a.md, kind: RULES, required: true}\n  - {id: x, root: PROJECT, path: docs/b.md, kind: RULES, required: true}\n",
		"multi-doc": "version: 1\nprojectId: order-service\nsources: []\n---\nversion: 1\nprojectId: other\nsources: []\n",
		"ads": "version: 1\nprojectId: order-service\nsources:\n  - {id: x, root: PROJECT, path: 'docs/a.md:stream', kind: RULES, required: true}\n",
		"unc-team-root": "version: 1\nprojectId: order-service\nteamRoot: '\\\\server\\share'\nsources:\n  - {id: x, root: TEAM, path: docs/a.md, kind: RULES, required: true}\n",
		"url-team-root": "version: 1\nprojectId: order-service\nteamRoot: 'https://example.invalid/team'\nsources:\n  - {id: x, root: TEAM, path: docs/a.md, kind: RULES, required: true}\n",
		"traversal": "version: 1\nprojectId: order-service\nsources:\n  - {id: x, root: PROJECT, path: ../a.md, kind: RULES, required: true}\n",
		"rules-optional": "version: 1\nprojectId: order-service\nsources:\n  - {id: x, root: PROJECT, path: docs/a.md, kind: RULES, required: false}\n",
		"unknown-root": "version: 1\nprojectId: order-service\nsources:\n  - {id: x, root: OTHER, path: docs/a.md, kind: RULES, required: true}\n",
		"unknown-kind": "version: 1\nprojectId: order-service\nsources:\n  - {id: x, root: PROJECT, path: docs/a.md, kind: OTHER, required: true}\n",
		"empty-project": "version: 1\nprojectId: ''\nsources: []\n",
		"duplicate-key": "version: 1\nprojectId: order-service\nprojectId: other\nsources: []\n",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) { if _, _, err := ParseBinding170([]byte(raw)); err == nil { t.Fatalf("invalid binding accepted: %s", raw) } })
	}
}

func Test170KnowledgeParseRuleAndApplicability(t *testing.T) {
	raw := activeRule170("ORDER-STATE-001", "order-service", "src/main/java/example/order/")
	rule, body, err := ParseRule170(raw)
	if err != nil { t.Fatal(err) }
	if rule.RuleID != "ORDER-STATE-001" || strings.TrimSpace(body) == "" { t.Fatalf("unexpected rule/body: %+v %q", rule, body) }
	if !Applies170(rule, Unit170{ID: "u1", Paths: []string{"src/main/java/example/order/OrderService.java"}}) { t.Fatal("directory prefix did not apply") }
	if Applies170(rule, Unit170{ID: "u2", Paths: []string{"src/main/java/example/payment/PaymentService.java"}}) { t.Fatal("rule leaked into sibling module") }

	all, _, err := ParseRule170(activeRule170("ALL-001", "*", "./"))
	if err != nil || !Applies170(all, Unit170{ID: "u3", Paths: []string{"any/project/path.txt"}}) { t.Fatalf("./ selector must apply to all project units: %v", err) }

	exactRaw := []byte(`---
ruleId: RISK-001
projectId: order-service
status: ACTIVE
version: "1"
owner: risk
source: requirements/risk.md
approvalRef: approvals/RISK
appliesTo:
  paths: []
  entryPoints: [com.acme.RiskService#check(java.lang.String)]
supersedes: []
exceptions: []
---
Check the exact overload.
`)
	exact, _, err := ParseRule170(exactRaw)
	if err != nil { t.Fatal(err) }
	if !Applies170(exact, Unit170{ID: "s", EntryPoints: []string{"com.acme.RiskService#check(java.lang.String)"}}) { t.Fatal("exact entry point did not apply") }
	if Applies170(exact, Unit170{ID: "l", EntryPoints: []string{"com.acme.RiskService#check(java.lang.Long)"}}) { t.Fatal("signature selector matched different overload") }
	if Applies170(exact, Unit170{ID: "b", EntryPoints: []string{"com.acme.RiskService"}}) { t.Fatal("bare class matched exact method selector") }

	bad := [][]byte{
		[]byte("ruleId: NO-FRONTMATTER\nbody"),
		[]byte("---\nruleId: DUP\nruleId: DUP2\n---\nbody\n"),
		[]byte("---\nruleId: X\n---\n---\nruleId: Y\n---\nbody\n"),
		[]byte("---\nruleId: X\nprojectId: order-service\nstatus: ACTIVE\nversion: '1'\nowner: x\nsource: x\napprovalRef: y\nappliesTo: {paths: ['./'], entryPoints: []}\nsupersedes: []\nexceptions: []\n---\n   \n"),
	}
	for _, input := range bad { if _, _, err := ParseRule170(input); err == nil { t.Fatalf("invalid rule accepted: %q", input) } }
}

func Test170KnowledgeLoadProjectAndTeamSources(t *testing.T) {
	repo := t.TempDir()
	team := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "docs", "business"), 0o755); err != nil { t.Fatal(err) }
	if err := os.MkdirAll(filepath.Join(team, "refs"), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(repo, "docs", "business", "order.md"), activeRule170("ORDER-STATE-001", "order-service", "src/main/java/example/order/"), 0o644); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(team, "refs", "platform.md"), []byte("platform reference\n"), 0o644); err != nil { t.Fatal(err) }
	binding := Binding170{Version: 1, ProjectID: "order-service", TeamRoot: team, Sources: []Source170{
		{ID: "order-rule", Root: "PROJECT", Path: "docs/business/order.md", Kind: "RULES", Required: true},
		{ID: "platform-ref", Root: "TEAM", Path: "refs/platform.md", Kind: "REFERENCE", Required: true},
	}}
	result, err := Load170(LoadInput170{RunID: "review-t7-load", RepoRoot: repo, Binding: binding, BindingSHA256: strings.Repeat("a", 64), Units: []Unit170{{ID: "unit-order", Paths: []string{"src/main/java/example/order/OrderService.java"}}, {ID: "unit-other", Paths: []string{"src/main/java/example/payment/Payment.java"}}}})
	if err != nil { t.Fatal(err) }
	if result.Manifest.Status != "READY" || len(result.Documents) != 2 { t.Fatalf("unexpected result: %+v docs=%d", result.Manifest, len(result.Documents)) }
	if len(result.Manifest.Checks) != 1 || result.Manifest.Checks[0].ReviewUnitID != "unit-order" || result.Manifest.Checks[0].RuleKey != "BUSINESS:order-rule:ORDER-STATE-001" || result.Manifest.Checks[0].Status != "READY" { t.Fatalf("unexpected checks: %+v", result.Manifest.Checks) }
	for _, source := range result.Manifest.Sources { if source.SHA256 == "" || len(source.SHA256) != 64 { t.Fatalf("source digest missing: %+v", source) } }
}

func Test170KnowledgeRequiredUnknownApplicabilityBlocksSelectedUnits(t *testing.T) {
	repo := t.TempDir()
	binding := Binding170{Version: 1, ProjectID: "order-service", Sources: []Source170{{ID: "missing-rule", Root: "PROJECT", Path: "docs/missing.md", Kind: "RULES", Required: true}}}
	result, err := Load170(LoadInput170{RunID: "review-t7-missing", RepoRoot: repo, Binding: binding, BindingSHA256: strings.Repeat("b", 64), Units: []Unit170{{ID: "unit-a", Paths: []string{"src/main/java/A.java"}}, {ID: "unit-b", Paths: []string{"src/main/java/B.java"}}}})
	if err != nil { t.Fatal(err) }
	if result.Manifest.Status != "PARTIAL" || len(result.Manifest.Checks) != 2 { t.Fatalf("required unreadable rule must conservatively block selected units: %+v", result.Manifest) }
	for _, check := range result.Manifest.Checks { if check.Status != "BLOCKED" || !strings.Contains(strings.Join(check.Reasons, " "), "SOURCE") { t.Fatalf("unexpected blocked check: %+v", check) } }
}

func Test170KnowledgeStateDuplicateAndProjectRulesFailClosed(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "rules"), 0o755); err != nil { t.Fatal(err) }
	write := func(name string, content []byte) { t.Helper(); if err := os.WriteFile(filepath.Join(repo, "rules", name), content, 0o644); err != nil { t.Fatal(err) } }
	write("a.md", activeRule170("ORDER-001", "order-service", "./"))
	write("b.md", activeRule170("ORDER-001", "order-service", "./"))
	binding := Binding170{Version: 1, ProjectID: "order-service", Sources: []Source170{
		{ID: "a", Root: "PROJECT", Path: "rules/a.md", Kind: "RULES", Required: true},
		{ID: "b", Root: "PROJECT", Path: "rules/b.md", Kind: "RULES", Required: true},
	}}
	result, err := Load170(LoadInput170{RunID: "review-t7-dup", RepoRoot: repo, Binding: binding, BindingSHA256: strings.Repeat("c", 64), Units: []Unit170{{ID: "unit-a", Paths: []string{"src/main/java/A.java"}}}})
	if err != nil { t.Fatal(err) }
	if result.Manifest.Status != "PARTIAL" || !strings.Contains(strings.Join(result.Manifest.Issues, " "), "DUPLICATE_RULE_ID") { t.Fatalf("duplicate formal ruleId must block: %+v", result.Manifest) }

	write("mismatch.md", activeRule170("OTHER-001", "other-project", "./"))
	binding.Sources = []Source170{{ID: "mismatch", Root: "PROJECT", Path: "rules/mismatch.md", Kind: "RULES", Required: true}}
	result, err = Load170(LoadInput170{RunID: "review-t7-project", RepoRoot: repo, Binding: binding, BindingSHA256: strings.Repeat("d", 64), Units: []Unit170{{ID: "unit-a", Paths: []string{"src/main/java/A.java"}}}})
	if err != nil { t.Fatal(err) }
	if result.Manifest.Status != "PARTIAL" || len(result.Manifest.Checks) != 1 || result.Manifest.Checks[0].Status != "BLOCKED" { t.Fatalf("project mismatch became effective: %+v", result.Manifest) }
}

func Test170KnowledgeBudgetCountsUniqueFilesAndCapsExperience(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "docs"), 0o755); err != nil { t.Fatal(err) }
	refContent := []byte(strings.Repeat("r", 1024))
	if err := os.WriteFile(filepath.Join(repo, "docs", "shared.md"), refContent, 0o644); err != nil { t.Fatal(err) }
	sources := []Source170{
		{ID: "shared-a", Root: "PROJECT", Path: "docs/shared.md", Kind: "REFERENCE", Required: true},
		{ID: "shared-b", Root: "PROJECT", Path: "docs/shared.md", Kind: "REFERENCE", Required: true},
	}
	for i := 0; i < 7; i++ {
		name := fmt.Sprintf("exp-%d.md", i)
		if err := os.WriteFile(filepath.Join(repo, "docs", name), []byte("experience\n"), 0o644); err != nil { t.Fatal(err) }
		sources = append(sources, Source170{ID: fmt.Sprintf("exp-%d", i), Root: "PROJECT", Path: "docs/" + name, Kind: "EXPERIENCE", Required: false})
	}
	result, err := Load170(LoadInput170{RunID: "review-t7-budget", RepoRoot: repo, Binding: Binding170{Version: 1, ProjectID: "p", Sources: sources}, BindingSHA256: strings.Repeat("e", 64), Units: []Unit170{{ID: "u", Paths: []string{"src/main/java/A.java"}}}})
	if err != nil { t.Fatal(err) }
	if len(result.Documents) != 7 { t.Fatalf("same final file must be read once per content but returned per declared source until budget; want shared x2 + max 5 experiences =7, got %d", len(result.Documents)) }
	blockedExperience := 0
	for _, source := range result.Manifest.Sources { if source.Kind == "EXPERIENCE" && source.Status == "BLOCKED" { blockedExperience++ } }
	if blockedExperience != 2 { t.Fatalf("experience cap must block exactly two sources, got %d: %+v", blockedExperience, result.Manifest.Sources) }
}

func Test170KnowledgeRejectsNonUTF8NonRegularAndEscapingSymlink(t *testing.T) {
	repo := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "docs", "dir.md"), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(repo, "docs", "binary.md"), []byte{0xff, 0xfe, 0xfd}, 0o644); err != nil { t.Fatal(err) }
	outsideFile := filepath.Join(outside, "outside.md")
	if err := os.WriteFile(outsideFile, []byte("outside\n"), 0o644); err != nil { t.Fatal(err) }
	link := filepath.Join(repo, "docs", "escape.md")
	if err := os.Symlink(outsideFile, link); err != nil { t.Fatalf("test requires symlink support: %v", err) }
	binding := Binding170{Version: 1, ProjectID: "p", Sources: []Source170{
		{ID: "dir", Root: "PROJECT", Path: "docs/dir.md", Kind: "REFERENCE", Required: true},
		{ID: "binary", Root: "PROJECT", Path: "docs/binary.md", Kind: "REFERENCE", Required: true},
		{ID: "escape", Root: "PROJECT", Path: "docs/escape.md", Kind: "REFERENCE", Required: true},
	}}
	result, err := Load170(LoadInput170{RunID: "review-t7-files", RepoRoot: repo, Binding: binding, BindingSHA256: strings.Repeat("f", 64), Units: []Unit170{{ID: "u"}}})
	if err != nil { t.Fatal(err) }
	if result.Manifest.Status != "PARTIAL" { t.Fatalf("invalid local materials must be partial: %+v", result.Manifest) }
	joined := strings.Join(result.Manifest.Issues, " ")
	for _, want := range []string{"NOT_REGULAR", "UTF8", "OUTSIDE_ROOT"} { if !strings.Contains(joined, want) { t.Fatalf("missing %s issue: %s", want, joined) } }
}

func Test170KnowledgeVerifyDetectsBindingAndSourceChanges(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "docs"), 0o755); err != nil { t.Fatal(err) }
	path := filepath.Join(repo, "docs", "rule.md")
	if err := os.WriteFile(path, activeRule170("ORDER-001", "order-service", "./"), 0o644); err != nil { t.Fatal(err) }
	binding := Binding170{Version: 1, ProjectID: "order-service", Sources: []Source170{{ID: "rule", Root: "PROJECT", Path: "docs/rule.md", Kind: "RULES", Required: true}}}
	input := LoadInput170{RunID: "review-t7-verify", RepoRoot: repo, Binding: binding, BindingSHA256: strings.Repeat("1", 64), Units: []Unit170{{ID: "u", Paths: []string{"src/main/java/A.java"}}}}
	loaded, err := Load170(input)
	if err != nil { t.Fatal(err) }
	if err := Verify170(input, loaded.Manifest); err != nil { t.Fatalf("fresh knowledge rejected: %v", err) }
	if err := os.WriteFile(path, activeRule170("ORDER-001", "order-service", "src/main/java/changed/"), 0o644); err != nil { t.Fatal(err) }
	if err := Verify170(input, loaded.Manifest); err == nil || !strings.Contains(err.Error(), "KNOWLEDGE_SOURCE_CHANGED") { t.Fatalf("source mutation must be rejected, got %v", err) }
	input.BindingSHA256 = strings.Repeat("2", 64)
	if err := Verify170(input, loaded.Manifest); err == nil || !strings.Contains(err.Error(), "KNOWLEDGE_BINDING_CHANGED") { t.Fatalf("binding mutation must be rejected, got %v", err) }
}

func Test170KnowledgeBindingDigestMatchesRawBytes(t *testing.T) {
	raw := validBinding170()
	_, digest, err := ParseBinding170(raw)
	if err != nil { t.Fatal(err) }
	want := fmt.Sprintf("%x", sha256.Sum256(raw))
	if digest != want { t.Fatalf("binding digest mismatch: got=%s want=%s", digest, want) }
}
