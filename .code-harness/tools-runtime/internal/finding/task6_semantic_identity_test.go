package finding

import "testing"

func TestSemanticIdentityIncludesEvidenceDigest(t *testing.T) {
	v := VerifiedProposal{Proposal: Proposal{RuleID: "R", Anchor: Anchor{Kind: AnchorLine, Path: "src/main/java/A.java", Line: 1}}, EvidenceDigest: "digest-a"}
	left := semanticIdentity160(v)
	v.EvidenceDigest = "digest-b"
	right := semanticIdentity160(v)
	if left == right { t.Fatal("semantic dedup identity must include evidenceDigest") }
}
