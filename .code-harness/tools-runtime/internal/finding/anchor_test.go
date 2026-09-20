package finding

import "testing"

func TestVerifyAcceptsExactLineAnchorInCurrentSymbol(t *testing.T) {
	ctx := verifyContext160(t)
	p := baseProposal160()
	verified, err := Verify(ctx, p)
	if err != nil {
		t.Fatalf("exact current-symbol line must verify: %v", err)
	}
	if verified.AnchorDigest == "" || verified.EvidenceDigest == "" {
		t.Fatalf("verified proposal must carry anchor/evidence digests: %#v", verified)
	}
}

func TestVerifyRejectsInventedLine(t *testing.T) {
	ctx := verifyContext160(t)
	p := baseProposal160()
	p.Anchor.Line = 999
	_, err := Verify(ctx, p)
	assertFindingCode160(t, err, "FINDING_ANCHOR_NOT_VERIFIED")
}

func TestVerifyRejectsInventedSymbol(t *testing.T) {
	ctx := verifyContext160(t)
	p := baseProposal160()
	p.Anchor.Symbol = "OrderServiceImpl.invented"
	_, err := Verify(ctx, p)
	assertFindingCode160(t, err, "FINDING_ANCHOR_NOT_VERIFIED")
}

func TestVerifyAcceptsSymbolAnchorForMissingConstraintFinding(t *testing.T) {
	ctx := verifyContext160(t)
	p := Proposal{
		ProposalID: "P-SYMBOL",
		ReviewUnitID: "RU-TASK3",
		RuleID: "SPRING-AUTH-001",
		Category: "PRODUCTION_CODE",
		Severity: "high",
		Anchor: Anchor{Kind: AnchorSymbol, Symbol: "OrderController.approve"},
		EvidenceRefs: []EvidenceRef{
			{Kind: "CHAIN", Value: "OrderController.approve"},
			{Kind: "SYMBOL", Value: "OrderController.approve"},
		},
		Problem: "入口缺少约束",
		Impact: "请求可能绕过业务限制",
		Recommendation: "在入口补充约束",
		NeedsTest: true,
		IntroducedByChange: false,
		Confidence: 0.9,
	}
	if _, err := Verify(ctx, p); err != nil {
		t.Fatalf("verified current symbol must be a legal anchor for missing-constraint findings: %v", err)
	}
}

func TestVerifyAcceptsFileAnchorWhenRuleAllowsFileEvidence(t *testing.T) {
	ctx := verifyContext160(t)
	p := Proposal{
		ProposalID: "P-FILE",
		ReviewUnitID: "RU-TASK3",
		RuleID: "SPRING-CONFIG-001",
		Category: "PRODUCTION_CODE",
		Severity: "high",
		Anchor: Anchor{Kind: AnchorFile, Path: yamlPath160},
		EvidenceRefs: []EvidenceRef{{Kind: "CHANGED_RANGE", Path: yamlPath160, StartLine: 2, EndLine: 3}},
		Problem: "连接配置变更存在高风险",
		Impact: "可能导致连接行为异常",
		Recommendation: "恢复安全配置并补充验证",
		NeedsTest: true,
		IntroducedByChange: true,
		Confidence: 0.91,
	}
	if _, err := Verify(ctx, p); err != nil {
		t.Fatalf("YamlConfig rule with CHANGED_RANGE evidence must allow FILE anchor: %v", err)
	}
}

func TestVerifyChangeSetRequiresTwoVerifiedEvidenceRefs(t *testing.T) {
	ctx := verifyContext160(t)
	p := Proposal{
		ProposalID: "P-CHANGESET",
		ReviewUnitID: "RU-TASK3",
		RuleID: "SPRING-AUTH-001",
		Category: "PRODUCTION_CODE",
		Severity: "high",
		Anchor: Anchor{Kind: AnchorChangeSet},
		EvidenceRefs: []EvidenceRef{{Kind: "SYMBOL", Value: "OrderController.approve"}},
		Problem: "跨文件约束不一致",
		Impact: "可能造成行为不一致",
		Recommendation: "统一跨文件约束",
		NeedsTest: true,
		IntroducedByChange: false,
		Confidence: 0.9,
	}
	_, err := Verify(ctx, p)
	assertFindingCode160(t, err, "FINDING_ANCHOR_NOT_VERIFIED")
}
