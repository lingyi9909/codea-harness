package finding

import (
	"strings"
	"testing"

	"codea-harness-tools/internal/nav"
	"codea-harness-tools/internal/reviewcontext"
	"codea-harness-tools/internal/reviewrules"
)

func Test170ContextRelationEvidenceAcceptsVerifiedCurrentRelation(t *testing.T) {
	ctx := verifyContext160(t)
	ctx.dispatch.Dispatches = append(ctx.dispatch.Dispatches, reviewrules.Dispatch{
		ReviewUnitID: "RU-TASK3", RuleID: "SPRING-REL-170", RuleVersion: 1,
		Kind: reviewrules.KindAgent, SeverityDefault: "high", RequiredEvidence: []string{"CONTEXT_RELATION", "CHANGED_RANGE"},
	})
	target := nav.ReviewRef170{Workspace: "current", Path: controllerPath160, Side: "CURRENT", Kind: "METHOD", OwnerFQCN: "com.acme.OrderController", Name: "approve", ParameterTypes: []string{}}
	targetKey, err := nav.ReviewRefKey170(target)
	if err != nil { t.Fatal(err) }
	ctx.reviewContext170 = &reviewcontext.Context170{
		RunID: "run-task3", Phase: "RULES",
		Relations: []nav.Relation170{{
			ID: "REL-170-1", Kind: "JAVA_CALL", Resolution: "EXACT",
			From: nav.ReviewRef170{Workspace: "current", Path: servicePath160, Side: "CURRENT", Kind: "METHOD", OwnerFQCN: "com.acme.OrderServiceImpl", Name: "approve", ParameterTypes: []string{}},
			Targets: []nav.ReviewRef170{target},
			Evidence: []nav.SourceRange170{{Ref: nav.ReviewRef170{Workspace: "current", Path: servicePath160, Side: "CURRENT", Kind: "METHOD", OwnerFQCN: "com.acme.OrderServiceImpl", Name: "approve", ParameterTypes: []string{}}, StartLine: 3, EndLine: 3, StartColumn: 3, EndColumn: 20}},
			Assumptions: []string{},
		}},
		Checks: []reviewcontext.Check170{{ReviewUnitID: "RU-TASK3", RuleID: "SPRING-REL-170", Status: "READY", RelationIDs: []string{"REL-170-1"}, Reasons: []string{}}},
	}
	p := baseProposal160()
	p.ProposalID = "P-REL-170"
	p.RuleID = "SPRING-REL-170"
	p.EvidenceRefs = []EvidenceRef{
		{Kind: "CONTEXT_RELATION", RelationID: "REL-170-1", Workspace: "current", SourceSide: "CURRENT", Value: targetKey},
		{Kind: "CHANGED_RANGE", Path: servicePath160, StartLine: 3, EndLine: 3},
	}
	if _, err := Verify(ctx, p); err != nil {
		t.Fatalf("verified CURRENT relation must pass: %v", err)
	}

	attacks := []struct{name string; mutate func(*Proposal)}{
		{"forged relation", func(p *Proposal){ p.EvidenceRefs[0].RelationID = "REL-FORGED" }},
		{"workspace tamper", func(p *Proposal){ p.EvidenceRefs[0].Workspace = "dependency" }},
		{"base as current", func(p *Proposal){ p.EvidenceRefs[0].SourceSide = "BASE" }},
		{"target tamper", func(p *Proposal){ p.EvidenceRefs[0].Value = strings.ReplaceAll(targetKey, "OrderController", "OtherController") }},
	}
	for _, tc := range attacks {
		t.Run(tc.name, func(t *testing.T) {
			bad := p
			bad.EvidenceRefs = append([]EvidenceRef(nil), p.EvidenceRefs...)
			tc.mutate(&bad)
			if _, err := Verify(ctx, bad); err == nil || !strings.Contains(err.Error(), "CONTEXT_RELATION_NOT_VERIFIED") {
				t.Fatalf("attack must be rejected with context authority error, got %v", err)
			}
		})
	}
}

func Test170ContextRelationCannotBorrowAnotherUnitCheck(t *testing.T) {
	ctx := verifyContext160(t)
	ctx.dispatch.Dispatches = append(ctx.dispatch.Dispatches, reviewrules.Dispatch{ReviewUnitID: "RU-TASK3", RuleID: "SPRING-REL-170", RuleVersion: 1, Kind: reviewrules.KindAgent, RequiredEvidence: []string{"CONTEXT_RELATION"}})
	ctx.reviewContext170 = &reviewcontext.Context170{
		RunID: "run-task3", Phase: "RULES",
		Relations: []nav.Relation170{{ID: "REL-OTHER", Kind: "JAVA_CALL", Resolution: "EXACT", From: nav.ReviewRef170{Workspace:"current", Path:servicePath160, Side:"CURRENT", Kind:"METHOD", OwnerFQCN:"com.acme.OrderServiceImpl", Name:"approve", ParameterTypes:[]string{}}, Targets: []nav.ReviewRef170{{Workspace:"current", Path:controllerPath160, Side:"CURRENT", Kind:"METHOD", OwnerFQCN:"com.acme.OrderController", Name:"approve", ParameterTypes:[]string{}}}, Evidence: []nav.SourceRange170{{Ref:nav.ReviewRef170{Workspace:"current", Path:servicePath160, Side:"CURRENT", Kind:"METHOD", OwnerFQCN:"com.acme.OrderServiceImpl", Name:"approve", ParameterTypes:[]string{}}, StartLine:3, EndLine:3, StartColumn:3, EndColumn:20}}, Assumptions:[]string{}}},
		Checks: []reviewcontext.Check170{{ReviewUnitID:"RU-OTHER", RuleID:"SPRING-REL-170", Status:"READY", RelationIDs:[]string{"REL-OTHER"}, Reasons:[]string{}}},
	}
	p := baseProposal160()
	p.RuleID = "SPRING-REL-170"
	p.EvidenceRefs = []EvidenceRef{{Kind:"CONTEXT_RELATION", RelationID:"REL-OTHER", Workspace:"current", SourceSide:"CURRENT"}}
	if _, err := Verify(ctx, p); err == nil || !strings.Contains(err.Error(), "CONTEXT_RELATION_NOT_VERIFIED") {
		t.Fatalf("cross-unit relation borrowing must fail, got %v", err)
	}
}
