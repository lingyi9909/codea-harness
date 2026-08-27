package finding

import "testing"

func TestCertifyProducesOneFindingForSemanticDuplicates(t *testing.T) {
	ctx := certifyContext160(t)
	first := baseProposal160()
	first.ProposalID = "P-DUP-A"
	second := first
	second.ProposalID = "P-DUP-B"
	second.Problem = "同一语义问题的另一种中文描述"
	second.Impact = "另一种影响措辞"
	second.Recommendation = "另一种修复建议"

	set, _, rejections, err := Certify(ctx, []Proposal{first, second})
	if err != nil {
		t.Fatalf("certify semantic duplicates: %v", err)
	}
	if len(rejections) != 0 {
		t.Fatalf("valid semantic duplicates must not be rejected: %+v", rejections)
	}
	if len(set.Findings) != 1 {
		t.Fatalf("semantic duplicates must collapse to one Certified Finding, got %d", len(set.Findings))
	}
}

func TestCertifyRejectsUnverifiedProposalWithoutBlockingOtherValidProposal(t *testing.T) {
	ctx := certifyContext160(t)
	valid := baseProposal160()
	valid.ProposalID = "P-VALID"
	invalid := baseProposal160()
	invalid.ProposalID = "P-INVALID"
	invalid.Anchor.Line = 999

	set, _, rejections, err := Certify(ctx, []Proposal{invalid, valid})
	if err != nil {
		t.Fatalf("single proposal rejection must not abort certification: %v", err)
	}
	if len(set.Findings) != 1 || len(rejections) != 1 {
		t.Fatalf("expected one Certified Finding and one rejection, findings=%d rejections=%+v", len(set.Findings), rejections)
	}
	if rejections[0].ProposalID != "P-INVALID" {
		t.Fatalf("unexpected rejection identity: %+v", rejections[0])
	}
}

func certifyContext160(t *testing.T) CertifyContext {
	t.Helper()
	return CertifyContext{
		Verify:                 verifyContext160(t),
		RunID:                  "run-task4",
		HarnessVersion:         "1.6.0",
		ChangeSetSHA256:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ChangeAnalysisSHA256:   "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ReviewUnitsSHA256:      "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		RuleDispatchSHA256:     "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		FindingProposalsSHA256: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		Mode:                   "FULL",
	}
}
