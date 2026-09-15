package finding

import (
	"strings"
	"testing"

	"codea-harness-tools/internal/knowledge"
	"codea-harness-tools/internal/reviewrules"
)

func Test170BusinessRuleEvidenceRequiresBoundKnowledgeAndCodeEvidence(t *testing.T) {
	ctx := verifyContext160(t)
	manifest := knowledge.Manifest170{
		RunID:         "run-task3",
		ProjectID:     "order-service",
		Status:        "READY",
		BindingSHA256: strings.Repeat("1", 64),
		Sources: []knowledge.SourceRecord170{{
			SourceID: "order-rules", Root: "PROJECT", Path: "docs/order-rules.md", Kind: "RULES", Required: true,
			SHA256: strings.Repeat("2", 64), Status: "READY", Reasons: []string{},
			Rule: &knowledge.Rule170{
				RuleID: "ORDER-001", ProjectID: "order-service", Status: "ACTIVE", Version: "2026-09",
				Owner: "risk", Source: "docs/order-rules.md", ApprovalRef: "APP-1",
				AppliesTo: knowledge.AppliesTo170{Paths: []string{"src/main/"}}, Supersedes: []string{}, Exceptions: []string{},
			},
		}},
		Checks: []knowledge.BusinessCheck170{{ReviewUnitID: "RU-TASK3", RuleKey: "BUSINESS:order-rules:ORDER-001", SourceID: "order-rules", RuleID: "ORDER-001", Status: "READY", Reasons: []string{}}},
		Issues: []string{},
	}
	digest, err := knowledge.Digest170(manifest)
	if err != nil {
		t.Fatal(err)
	}
	ctx.knowledge170 = &manifest
	ctx.dispatch.KnowledgeSHA256 = digest
	ctx.dispatch.Dispatches = append(ctx.dispatch.Dispatches, reviewrules.Dispatch{
		ReviewUnitID: "RU-TASK3", RuleID: "BUSINESS:order-rules:ORDER-001", RuleVersion: 1, Kind: reviewrules.KindAgent, SeverityDefault: "medium",
		RequiredEvidence: []string{"BUSINESS_RULE", "CHANGED_RANGE"}, DispatchReason: []string{"BUSINESS_RULE:order-rules:ORDER-001"},
	})
	p := baseProposal160()
	p.ProposalID = "P-BUSINESS-170"
	p.RuleID = "BUSINESS:order-rules:ORDER-001"
	p.EvidenceRefs = []EvidenceRef{
		{Kind: "BUSINESS_RULE", SourceID: "order-rules", RuleID: "ORDER-001", SourceSHA256: strings.Repeat("2", 64)},
		{Kind: "CHANGED_RANGE", Path: servicePath160, StartLine: 3, EndLine: 3},
	}
	if _, err := Verify(ctx, p); err != nil {
		t.Fatalf("bound business evidence plus code evidence must pass: %v", err)
	}

	badDigest := p
	badDigest.EvidenceRefs = append([]EvidenceRef(nil), p.EvidenceRefs...)
	badDigest.EvidenceRefs[0].SourceSHA256 = strings.Repeat("3", 64)
	if _, err := Verify(ctx, badDigest); err == nil || !strings.Contains(err.Error(), "BUSINESS_RULE_NOT_VERIFIED") {
		t.Fatalf("forged business source digest must fail, got %v", err)
	}

	noCode := p
	noCode.EvidenceRefs = []EvidenceRef{{Kind: "BUSINESS_RULE", SourceID: "order-rules", RuleID: "ORDER-001", SourceSHA256: strings.Repeat("2", 64)}}
	ctx.dispatch.Dispatches[len(ctx.dispatch.Dispatches)-1].RequiredEvidence = []string{"BUSINESS_RULE"}
	if _, err := Verify(ctx, noCode); err == nil || !strings.Contains(err.Error(), "BUSINESS_RULE_CODE_EVIDENCE_REQUIRED") {
		t.Fatalf("business document alone must never certify a code finding, got %v", err)
	}
}

func Test170KnowledgeDocumentCannotBecomeCodeAnchor(t *testing.T) {
	ctx := verifyContext160(t)
	ctx.knowledge170 = &knowledge.Manifest170{
		RunID: "run-task3", ProjectID: "order-service", Status: "READY", BindingSHA256: strings.Repeat("1", 64),
		Sources: []knowledge.SourceRecord170{{
			SourceID: "order-rules", Root: "PROJECT", Path: servicePath160, Kind: "RULES", Required: true,
			SHA256: strings.Repeat("2", 64), Status: "READY",
			Rule: &knowledge.Rule170{RuleID: "ORDER-001", ProjectID: "order-service", Status: "ACTIVE", Version: "1", Owner: "risk", Source: servicePath160, ApprovalRef: "A", AppliesTo: knowledge.AppliesTo170{Paths: []string{"./"}}},
		}},
	}
	p := baseProposal160()
	if _, _, err := verifyAnchor160(ctx, ctx.units.Units[0], ctx.dispatch.Dispatches[0], p.Anchor, p.EvidenceRefs); err == nil || !strings.Contains(err.Error(), "KNOWLEDGE_ANCHOR_FORBIDDEN") {
		t.Fatalf("knowledge source must not become code anchor, got %v", err)
	}
}
