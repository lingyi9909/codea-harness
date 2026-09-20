package finding

import (
	"codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/nav"
	"codea-harness-tools/internal/reviewrules"
	"codea-harness-tools/internal/reviewunit"
)

type AnchorKind string

const (
	AnchorLine      AnchorKind = "LINE"
	AnchorSymbol    AnchorKind = "SYMBOL"
	AnchorFile      AnchorKind = "FILE"
	AnchorChangeSet AnchorKind = "CHANGESET"
)

type Anchor struct {
	Kind   AnchorKind `json:"kind"`
	Path   string     `json:"path,omitempty"`
	Line   int        `json:"line,omitempty"`
	Symbol string     `json:"symbol,omitempty"`
}

type EvidenceRef struct {
	Kind      string `json:"kind"`
	Value     string `json:"value,omitempty"`
	Path      string `json:"path,omitempty"`
	StartLine int    `json:"startLine,omitempty"`
	EndLine   int    `json:"endLine,omitempty"`
}

type Proposal struct {
	ProposalID         string        `json:"proposalId"`
	ReviewUnitID       string        `json:"reviewUnitId"`
	RuleID             string        `json:"ruleId"`
	Category           string        `json:"category"`
	Severity           string        `json:"severity"`
	Anchor             Anchor        `json:"anchor"`
	EvidenceRefs       []EvidenceRef `json:"evidenceRefs"`
	Problem            string        `json:"problem"`
	Impact             string        `json:"impact"`
	Recommendation     string        `json:"recommendation"`
	NeedsTest          bool          `json:"needsTest"`
	IntroducedByChange bool          `json:"introducedByChange"`
	Confidence         float64       `json:"confidence"`
}

type VerifiedProposal struct {
	Proposal       Proposal
	AnchorDigest   string
	EvidenceDigest string
}

type VerifyContext struct {
	trusted      bool
	repoRoot     string
	analysis     analysis.ChangeAnalysis
	units        reviewunit.Manifest
	dispatch     reviewrules.Manifest
	symbolRanges map[string]nav.SymbolInfo
}
