package finding

import (
	"testing"

	"codea-harness-tools/internal/reviewrules"
)

func TestVerifyChangedRangeRequiresFullHunkCoverage(t *testing.T) {
	cases := []struct {
		name      string
		startLine int
		endLine   int
		wantOK    bool
	}{
		{name: "exact_3_4", startLine: 3, endLine: 4, wantOK: true},
		{name: "starts_before_hunk", startLine: 2, endLine: 4, wantOK: false},
		{name: "ends_after_hunk", startLine: 3, endLine: 5, wantOK: false},
		{name: "wide_range", startLine: 1, endLine: 100, wantOK: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := verifyContext160(t)
			addChangedRangeDispatch160(&ctx)
			p := changedRangeProposal160(tc.startLine, tc.endLine)
			_, err := Verify(ctx, p)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("changed range %d-%d must verify: %v", tc.startLine, tc.endLine, err)
				}
				return
			}
			assertFindingCode160(t, err, "FINDING_EVIDENCE_NOT_VERIFIED")
		})
	}
}

func TestVerifyIntroducedByChangeRejectsPartialChangedRange(t *testing.T) {
	ctx := verifyContext160(t)
	addChangedRangeDispatch160(&ctx)
	p := changedRangeProposal160(2, 4)
	p.ProposalID = "P-PARTIAL-INTRODUCED"
	p.IntroducedByChange = true

	_, err := Verify(ctx, p)
	assertFindingCode160(t, err, "FINDING_EVIDENCE_NOT_VERIFIED")
}

func TestVerifyChangeSetRejectsDuplicateEvidenceRefs(t *testing.T) {
	ctx := verifyContext160(t)
	addChangedRangeDispatch160(&ctx)
	p := changedRangeProposal160(3, 4)
	p.ProposalID = "P-CHANGESET-DUPLICATE"
	p.Anchor = Anchor{Kind: AnchorChangeSet}
	p.EvidenceRefs = []EvidenceRef{
		{Kind: "CHANGED_RANGE", Path: servicePath160, StartLine: 3, EndLine: 4},
		{Kind: "CHANGED_RANGE", Path: servicePath160, StartLine: 3, EndLine: 4},
	}

	_, err := Verify(ctx, p)
	assertFindingCode160(t, err, "FINDING_ANCHOR_NOT_VERIFIED")
}

func addChangedRangeDispatch160(ctx *VerifyContext) {
	ctx.dispatch.Dispatches = append(ctx.dispatch.Dispatches, reviewrules.Dispatch{
		ReviewUnitID:     "RU-TASK3",
		RuleID:           "SPRING-RANGE-TEST-001",
		RuleVersion:      1,
		Kind:             reviewrules.KindAgent,
		SeverityDefault:  "high",
		RequiredEvidence: []string{"CHANGED_RANGE"},
	})
}

func changedRangeProposal160(startLine, endLine int) Proposal {
	return Proposal{
		ProposalID:   "P-CHANGED-RANGE",
		ReviewUnitID: "RU-TASK3",
		RuleID:       "SPRING-RANGE-TEST-001",
		Category:     "PRODUCTION_CODE",
		Severity:     "high",
		Anchor:       Anchor{Kind: AnchorFile, Path: servicePath160},
		EvidenceRefs: []EvidenceRef{{
			Kind:      "CHANGED_RANGE",
			Path:      servicePath160,
			StartLine: startLine,
			EndLine:   endLine,
		}},
		Problem:            "changed range evidence regression",
		Impact:             "partial overlap could be accepted",
		Recommendation:     "require full hunk coverage",
		NeedsTest:          true,
		IntroducedByChange: false,
		Confidence:         0.9,
	}
}
