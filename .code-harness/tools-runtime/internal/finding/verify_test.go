package finding

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/nav"
	"codea-harness-tools/internal/reviewrules"
	"codea-harness-tools/internal/reviewunit"
)

const (
	servicePath160    = "src/main/java/com/acme/OrderServiceImpl.java"
	controllerPath160 = "src/main/java/com/acme/OrderController.java"
	yamlPath160       = "src/main/resources/application.yml"
	testPath160       = "src/test/java/com/acme/OrderServiceTest.java"
	dependencyPath160 = "workspace/shared/src/main/java/com/acme/SharedPolicy.java"
)

func TestVerifyRejectsRuleNotDispatchedForUnit(t *testing.T) {
	ctx := verifyContext160(t)
	p := baseProposal160()
	p.RuleID = "SPRING-NOT-DISPATCHED-999"
	_, err := Verify(ctx, p)
	assertFindingCode160(t, err, "RULE_NOT_DISPATCHED")
}

func TestVerifyRejectsScopeOutsideFile(t *testing.T) {
	ctx := verifyContext160(t)
	p := baseProposal160()
	p.Anchor = Anchor{Kind: AnchorFile, Path: "src/main/java/com/acme/Outside.java"}
	_, err := Verify(ctx, p)
	assertFindingCode160(t, err, "FINDING_SCOPE_VIOLATION")
}

func TestVerifyRejectsWorkspaceDependencyPath(t *testing.T) {
	ctx := verifyContext160(t)
	p := baseProposal160()
	p.Anchor = Anchor{Kind: AnchorFile, Path: dependencyPath160}
	_, err := Verify(ctx, p)
	assertFindingCode160(t, err, "FINDING_DEPENDENCY_SCOPE_FORBIDDEN")
}

func TestVerifyRejectsIntroducedByChangeWithoutHunkOrContractEvidence(t *testing.T) {
	ctx := verifyContext160(t)
	p := Proposal{
		ProposalID: "P-INTRODUCED",
		ReviewUnitID: "RU-TASK3",
		RuleID: "SPRING-AUTH-001",
		Category: "PRODUCTION_CODE",
		Severity: "high",
		Anchor: Anchor{Kind: AnchorLine, Path: controllerPath160, Line: 3, Symbol: "OrderController.approve"},
		EvidenceRefs: []EvidenceRef{
			{Kind: "CHAIN", Value: "OrderController.approve"},
			{Kind: "SYMBOL", Value: "OrderController.approve"},
		},
		Problem: "新增入口缺少约束",
		Impact: "可能绕过业务限制",
		Recommendation: "补充最小约束",
		NeedsTest: true,
		IntroducedByChange: true,
		Confidence: 0.9,
	}
	_, err := Verify(ctx, p)
	assertFindingCode160(t, err, "FINDING_INTRODUCED_BY_CHANGE_NOT_VERIFIED")
}

func TestVerifyPreservesExistingTestValidityBoundary(t *testing.T) {
	ctx := verifyContext160(t)
	p := baseProposal160()
	p.ProposalID = "P-TEST-VALIDITY"
	p.Anchor = Anchor{Kind: AnchorLine, Path: testPath160, Line: 3}
	p.Category = "PRODUCTION_CODE"
	_, err := Verify(ctx, p)
	assertFindingCode160(t, err, "FINDING_PROPOSAL_INVALID")

	p.Category = "TEST_VALIDITY"
	if _, err := Verify(ctx, p); err != nil {
		t.Fatalf("TEST_VALIDITY must remain allowed on test scope when other runtime evidence is verified: %v", err)
	}
}

func verifyContext160(t *testing.T) VerifyContext {
	t.Helper()
	root := t.TempDir()
	writeSource160(t, root, servicePath160, "package com.acme;\npublic class OrderServiceImpl {\n  public void approve() {\n    save();\n  }\n  void save() {}\n}\n")
	writeSource160(t, root, controllerPath160, "package com.acme;\npublic class OrderController {\n  public void approve() { service.approve(); }\n}\n")
	writeSource160(t, root, yamlPath160, "spring:\n  datasource:\n    url: jdbc:mysql://localhost/test\n")
	writeSource160(t, root, testPath160, "package com.acme;\npublic class OrderServiceTest {\n  public void approve_test() {}\n}\n")

	units := reviewunit.Manifest{
		RunID: "run-task3",
		HarnessVersion: "1.6.0",
		Mode: reviewunit.ModeFull,
		ChangeSetSHA256: strings.Repeat("a", 64),
		ChangeAnalysisSHA256: strings.Repeat("b", 64),
		SHA256: strings.Repeat("c", 64),
		Units: []reviewunit.Unit{{
			ID: "RU-TASK3",
			EntryPoint: "OrderController.approve",
			Chain: []string{"OrderController.approve", "OrderServiceImpl.approve"},
			Files: []reviewunit.FileRef{
				{Path: controllerPath160, Role: "Controller", Changed: true, Workspace: "current"},
				{Path: servicePath160, Role: "Service", Changed: true, Workspace: "current"},
				{Path: yamlPath160, Role: "YamlConfig", Changed: true, Workspace: "current"},
				{Path: testPath160, Role: "Test", Changed: true, Workspace: "current"},
			},
			ChangedHunks: []reviewunit.HunkRef{
				{Path: servicePath160, NewStart: 3, NewLines: 2},
				{Path: yamlPath160, NewStart: 2, NewLines: 2},
				{Path: testPath160, NewStart: 3, NewLines: 1},
			},
		}},
	}
	dispatch := reviewrules.Manifest{
		RunID: "run-task3",
		ReviewUnitsSHA256: units.SHA256,
		RuleCatalogSHA256: strings.Repeat("d", 64),
		SHA256: strings.Repeat("e", 64),
		Dispatches: []reviewrules.Dispatch{
			{ReviewUnitID: "RU-TASK3", RuleID: "SPRING-TX-001", RuleVersion: 1, Kind: reviewrules.KindAgent, SeverityDefault: "high", RequiredEvidence: []string{"CHAIN", "SYMBOL"}},
			{ReviewUnitID: "RU-TASK3", RuleID: "SPRING-AUTH-001", RuleVersion: 1, Kind: reviewrules.KindAgent, SeverityDefault: "high", RequiredEvidence: []string{"CHAIN", "SYMBOL"}},
			{ReviewUnitID: "RU-TASK3", RuleID: "SPRING-CONFIG-001", RuleVersion: 1, Kind: reviewrules.KindAgent, SeverityDefault: "high", RequiredEvidence: []string{"CHANGED_RANGE"}},
		},
	}
	analysisValue := analysis.ChangeAnalysis{
		SymbolLocations: []analysis.SymbolLocation{
			{Workspace: "current", Symbol: "OrderServiceImpl.approve", Path: servicePath160, Role: "Service", Source: "AST_GREP"},
			{Workspace: "current", Symbol: "OrderController.approve", Path: controllerPath160, Role: "Controller", Source: "AST_GREP"},
			{Workspace: "dependency", Symbol: "SharedPolicy.check", Path: dependencyPath160, Role: "Service", Source: "WORKSPACE_DEPENDENCY"},
		},
	}
	return VerifyContext{
		trusted: true,
		repoRoot: root,
		analysis: analysisValue,
		units: units,
		dispatch: dispatch,
		symbolRanges: map[string]nav.SymbolInfo{
			"OrderServiceImpl.approve": {Symbol: "OrderServiceImpl.approve", Path: servicePath160, LineStart: 3, LineEnd: 5},
			"OrderController.approve": {Symbol: "OrderController.approve", Path: controllerPath160, LineStart: 3, LineEnd: 3},
		},
	}
}

func baseProposal160() Proposal {
	return Proposal{
		ProposalID: "P-001",
		ReviewUnitID: "RU-TASK3",
		RuleID: "SPRING-TX-001",
		Category: "PRODUCTION_CODE",
		Severity: "high",
		Anchor: Anchor{Kind: AnchorLine, Path: servicePath160, Line: 3, Symbol: "OrderServiceImpl.approve"},
		EvidenceRefs: []EvidenceRef{
			{Kind: "CHAIN", Value: "OrderServiceImpl.approve"},
			{Kind: "SYMBOL", Value: "OrderServiceImpl.approve"},
		},
		Problem: "事务代理可能失效",
		Impact: "事务边界可能不生效",
		Recommendation: "通过代理边界调用",
		NeedsTest: true,
		IntroducedByChange: false,
		Confidence: 0.93,
	}
}

func writeSource160(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil { t.Fatal(err) }
}

func assertFindingCode160(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), code) {
		t.Fatalf("expected %s, got %v", code, err)
	}
}
