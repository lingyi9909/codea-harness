package finding

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type CertifiedFinding struct {
	ID                 string        `json:"id"`
	RuleID             string        `json:"ruleId"`
	ReviewUnitID       string        `json:"reviewUnitId"`
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

type CertifiedSet struct {
	RunID                  string             `json:"runId"`
	HarnessVersion         string             `json:"harnessVersion"`
	ChangeSetSHA256        string             `json:"changeSetSha256"`
	ChangeAnalysisSHA256   string             `json:"changeAnalysisSha256"`
	ReviewUnitsSHA256      string             `json:"reviewUnitsSha256"`
	RuleDispatchSHA256     string             `json:"ruleDispatchSha256"`
	FindingProposalsSHA256 string             `json:"findingProposalsSha256"`
	Findings               []CertifiedFinding `json:"findings"`
	SHA256                 string             `json:"sha256"`
}

type Certificate struct {
	RunID                   string `json:"runId"`
	CertifiedFindingsSHA256 string `json:"certifiedFindingsSha256"`
	ChangeSetSHA256         string `json:"changeSetSha256"`
	ChangeAnalysisSHA256    string `json:"changeAnalysisSha256"`
	ReviewUnitsSHA256       string `json:"reviewUnitsSha256"`
	RuleDispatchSHA256      string `json:"ruleDispatchSha256"`
	FindingProposalsSHA256  string `json:"findingProposalsSha256"`
	Mode                    string `json:"mode"`
	ScopeSHA256             string `json:"scopeSha256,omitempty"`
}

type Rejection struct {
	ProposalID string `json:"proposalId"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

type CertifyContext struct {
	Verify                 VerifyContext
	RunID                  string
	HarnessVersion         string
	ChangeSetSHA256        string
	ChangeAnalysisSHA256   string
	ReviewUnitsSHA256      string
	RuleDispatchSHA256     string
	FindingProposalsSHA256 string
	Mode                   string
	ScopeSHA256            string
}

func Certify(ctx CertifyContext, proposals []Proposal) (CertifiedSet, Certificate, []Rejection, error) {
	if !findingRunID160.MatchString(strings.TrimSpace(ctx.RunID)) || strings.TrimSpace(ctx.RunID) != ctx.RunID {
		return CertifiedSet{}, Certificate{}, nil, findingError160("FINDING_CERTIFY_CONTEXT_INVALID", "invalid runId %q", ctx.RunID)
	}
	if strings.TrimSpace(ctx.HarnessVersion) == "" {
		return CertifiedSet{}, Certificate{}, nil, findingError160("FINDING_CERTIFY_CONTEXT_INVALID", "harnessVersion is required")
	}
	if !validSHA160(ctx.ChangeSetSHA256) || !validSHA160(ctx.ChangeAnalysisSHA256) || !validSHA160(ctx.ReviewUnitsSHA256) || !validSHA160(ctx.RuleDispatchSHA256) || !validSHA160(ctx.FindingProposalsSHA256) {
		return CertifiedSet{}, Certificate{}, nil, findingError160("FINDING_CERTIFY_CONTEXT_INVALID", "authority hashes must be lowercase sha256")
	}
	mode := strings.ToUpper(strings.TrimSpace(ctx.Mode))
	if mode != "FULL" && mode != "TARGETED" {
		return CertifiedSet{}, Certificate{}, nil, findingError160("FINDING_CERTIFY_CONTEXT_INVALID", "invalid mode %q", ctx.Mode)
	}
	if !ctx.Verify.trusted || ctx.Verify.units.RunID != ctx.RunID || ctx.Verify.dispatch.RunID != ctx.RunID {
		return CertifiedSet{}, Certificate{}, nil, findingError160("FINDING_CERTIFY_CONTEXT_INVALID", "VerifyContext is not bound to the certification run")
	}
	if ctx.Verify.units.ChangeSetSHA256 != ctx.ChangeSetSHA256 || ctx.Verify.units.ChangeAnalysisSHA256 != ctx.ChangeAnalysisSHA256 || string(ctx.Verify.units.Mode) != mode {
		return CertifiedSet{}, Certificate{}, nil, findingError160("FINDING_CERTIFY_CONTEXT_INVALID", "VerifyContext authority identity differs from certification context")
	}
	if mode == "TARGETED" && strings.TrimSpace(ctx.Verify.units.ReviewScopeSHA256) != strings.TrimSpace(ctx.ScopeSHA256) {
		return CertifiedSet{}, Certificate{}, nil, findingError160("FINDING_CERTIFY_CONTEXT_INVALID", "TARGETED scope identity differs from ReviewUnit authority")
	}
	verified := make([]VerifiedProposal, 0, len(proposals))
	rejections := make([]Rejection, 0)
	for _, proposal := range proposals {
		result, err := Verify(ctx.Verify, proposal)
		if err != nil {
			rejections = append(rejections, Rejection{ProposalID: strings.TrimSpace(proposal.ProposalID), Code: findingCode160(err), Message: err.Error()})
			continue
		}
		verified = append(verified, result)
	}
	sort.Slice(rejections, func(i, j int) bool { return rejections[i].ProposalID < rejections[j].ProposalID })
	set := CertifiedSet{RunID: ctx.RunID, HarnessVersion: strings.TrimSpace(ctx.HarnessVersion), ChangeSetSHA256: ctx.ChangeSetSHA256, ChangeAnalysisSHA256: ctx.ChangeAnalysisSHA256, ReviewUnitsSHA256: ctx.ReviewUnitsSHA256, RuleDispatchSHA256: ctx.RuleDispatchSHA256, FindingProposalsSHA256: ctx.FindingProposalsSHA256, Findings: dedupVerified160(verified)}
	unsigned, err := canonicalCertifiedSet160(set, false)
	if err != nil { return CertifiedSet{}, Certificate{}, nil, findingError160("FINDING_CERTIFY_ENCODE_FAILED", "%v", err) }
	set.SHA256 = hashFindingBytes160(unsigned)
	setBytes, err := canonicalCertifiedSet160(set, true)
	if err != nil { return CertifiedSet{}, Certificate{}, nil, findingError160("FINDING_CERTIFY_ENCODE_FAILED", "%v", err) }
	cert := Certificate{RunID: ctx.RunID, CertifiedFindingsSHA256: hashFindingBytes160(setBytes), ChangeSetSHA256: ctx.ChangeSetSHA256, ChangeAnalysisSHA256: ctx.ChangeAnalysisSHA256, ReviewUnitsSHA256: ctx.ReviewUnitsSHA256, RuleDispatchSHA256: ctx.RuleDispatchSHA256, FindingProposalsSHA256: ctx.FindingProposalsSHA256, Mode: mode, ScopeSHA256: strings.TrimSpace(ctx.ScopeSHA256)}
	if cert.ScopeSHA256 != "" && !validSHA160(cert.ScopeSHA256) {
		return CertifiedSet{}, Certificate{}, nil, findingError160("FINDING_CERTIFY_CONTEXT_INVALID", "scopeSha256 must be lowercase sha256")
	}
	return set, cert, rejections, nil
}

func canonicalCertifiedSet160(set CertifiedSet, includeSHA bool) ([]byte, error) {
	candidate := set
	if !includeSHA { candidate.SHA256 = "" }
	if candidate.Findings == nil { candidate.Findings = []CertifiedFinding{} }
	data, err := json.MarshalIndent(candidate, "", "  ")
	if err != nil { return nil, err }
	return append(data, '\n'), nil
}

func canonicalCertificate160(cert Certificate) ([]byte, error) {
	data, err := json.MarshalIndent(cert, "", "  ")
	if err != nil { return nil, err }
	return append(data, '\n'), nil
}

func hashFindingBytes160(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

func validSHA160(value string) bool {
	if len(value) != 64 { return false }
	for _, ch := range value {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') { return false }
	}
	return true
}

func findingCode160(err error) string {
	if err == nil { return "" }
	text := err.Error()
	if idx := strings.IndexByte(text, ':'); idx > 0 { return strings.TrimSpace(text[:idx]) }
	return "FINDING_REJECTED"
}
