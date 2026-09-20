package finding

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

type certifiedCandidate160 struct {
	verified VerifiedProposal
	identity string
}

func semanticIdentity160(v VerifiedProposal) string {
	anchor := v.Proposal.Anchor
	path := strings.TrimSpace(anchor.Path)
	if p, ok := safeFindingPath160(path); ok {
		path = p
	}
	canonical := strings.TrimSpace(anchor.Symbol)
	if canonical == "" {
		resources := make([]string, 0)
		for _, ref := range v.Proposal.EvidenceRefs {
			if strings.EqualFold(strings.TrimSpace(ref.Kind), "RESOURCE_RELATION") {
				resources = append(resources, strings.TrimSpace(ref.Path)+"|"+strings.TrimSpace(ref.Value))
			}
		}
		sort.Strings(resources)
		canonical = strings.Join(resources, ",")
	}
	return strings.Join([]string{
		strings.TrimSpace(v.Proposal.RuleID),
		string(anchor.Kind),
		path,
		canonical,
		v.EvidenceDigest,
	}, "\x00")
}

func certifiedFindingID160(identity string) string {
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(identity)))
	return "CF-" + digest[:16]
}

func dedupVerified160(in []VerifiedProposal) []CertifiedFinding {
	candidates := make([]certifiedCandidate160, 0, len(in))
	for _, verified := range in {
		candidates = append(candidates, certifiedCandidate160{verified: verified, identity: semanticIdentity160(verified)})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].identity != candidates[j].identity {
			return candidates[i].identity < candidates[j].identity
		}
		return strings.TrimSpace(candidates[i].verified.Proposal.ProposalID) < strings.TrimSpace(candidates[j].verified.Proposal.ProposalID)
	})
	out := make([]CertifiedFinding, 0, len(candidates))
	last := ""
	for _, candidate := range candidates {
		if candidate.identity == last {
			continue
		}
		last = candidate.identity
		p := candidate.verified.Proposal
		out = append(out, CertifiedFinding{
			ID:                 certifiedFindingID160(candidate.identity),
			RuleID:             strings.TrimSpace(p.RuleID),
			ReviewUnitID:       strings.TrimSpace(p.ReviewUnitID),
			Category:           p.Category,
			Severity:           strings.ToLower(strings.TrimSpace(p.Severity)),
			Anchor:             p.Anchor,
			EvidenceRefs:       append([]EvidenceRef(nil), p.EvidenceRefs...),
			Problem:            strings.TrimSpace(p.Problem),
			Impact:             strings.TrimSpace(p.Impact),
			Recommendation:     strings.TrimSpace(p.Recommendation),
			NeedsTest:          p.NeedsTest,
			IntroducedByChange: p.IntroducedByChange,
			Confidence:         p.Confidence,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
