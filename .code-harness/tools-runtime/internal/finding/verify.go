package finding

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/nav"
	"codea-harness-tools/internal/reviewrules"
	"codea-harness-tools/internal/reviewunit"
	"codea-harness-tools/internal/schema"
	"codea-harness-tools/internal/symbolid"
)

var findingRunID160 = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func LoadVerifyContext(repoRoot, runID, astGrepPath string) (VerifyContext, error) {
	root := strings.TrimSpace(repoRoot)
	if root == "" {
		root = "."
	}
	root = filepath.Clean(root)
	runID = strings.TrimSpace(runID)
	if !findingRunID160.MatchString(runID) {
		return VerifyContext{}, findingError160("FINDING_PROPOSAL_INVALID", "invalid runId %q", runID)
	}
	analysisRel := filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json"))
	analysisValue, cert, err := analysisruntime.LoadCertified(root, analysisRel)
	if err != nil {
		return VerifyContext{}, findingError160("FINDING_PROPOSAL_INVALID", "load Certified Analysis: %v", err)
	}
	units, err := reviewunit.Load(reviewunit.BuildInput{RunID: runID, CertifiedRunID: runID, RepoRoot: root})
	if err != nil {
		return VerifyContext{}, findingError160("FINDING_PROPOSAL_INVALID", "load ReviewUnit authority: %v", err)
	}
	if units.RunID != runID || units.ChangeAnalysisSHA256 != cert.AnalysisSHA256 {
		return VerifyContext{}, findingError160("FINDING_PROPOSAL_INVALID", "ReviewUnit authority is not bound to current Certified Analysis")
	}
	dispatch, err := loadDispatchAuthority160(root, runID, units)
	if err != nil {
		return VerifyContext{}, err
	}

	ranges := map[string]nav.SymbolInfo{}
	if strings.TrimSpace(astGrepPath) != "" {
		navigator := nav.Navigator{RepoRoot: root, AstGrepPath: astGrepPath}
		counts := map[string]int{}
		unique := map[string]nav.SymbolInfo{}
		seenIdentity := map[string]bool{}
		for _, loc := range analysisValue.SymbolLocations {
			ref, ok := symbolid.FromLocation(loc.Workspace, loc.Path, loc.Symbol)
			if !ok || ref.Workspace != symbolid.CurrentWorkspace || !strings.HasSuffix(strings.ToLower(ref.Path), ".java") {
				continue
			}
			key, _ := symbolid.Key(ref)
			if seenIdentity[key] {
				continue
			}
			seenIdentity[key] = true
			info, err := navigator.GetSymbolInfo(context.Background(), ref.Symbol, ref.Path)
			if err != nil {
				return VerifyContext{}, findingError160("FINDING_ANCHOR_NOT_VERIFIED", "resolve symbol %s at %s with pinned navigation: %v", ref.Symbol, ref.Path, err)
			}
			ranges[key] = info
			counts[ref.Symbol]++
			unique[ref.Symbol] = info
		}
		// Preserve the legacy bare-symbol lookup only when it is authoritative.
		// Ambiguous symbols are available exclusively through their exact identity.
		for symbol, count := range counts {
			if count == 1 {
				ranges[symbol] = unique[symbol]
			}
		}
	}
	return VerifyContext{
		trusted: true,
		repoRoot: root,
		analysis: analysisValue,
		units: units,
		dispatch: dispatch,
		symbolRanges: ranges,
	}, nil
}

func Verify(ctx VerifyContext, p Proposal) (VerifiedProposal, error) {
	if !ctx.trusted || strings.TrimSpace(ctx.repoRoot) == "" {
		return VerifiedProposal{}, findingError160("FINDING_PROPOSAL_INVALID", "VerifyContext is not Runtime-owned")
	}
	if err := validateProposalShape160(p); err != nil {
		return VerifiedProposal{}, findingError160("FINDING_PROPOSAL_INVALID", "%v", err)
	}
	unit, ok := findUnit160(ctx.units, strings.TrimSpace(p.ReviewUnitID))
	if !ok {
		return VerifiedProposal{}, findingError160("FINDING_SCOPE_VIOLATION", "ReviewUnit %s does not exist in current authority", p.ReviewUnitID)
	}
	dispatch, ok := findDispatch160(ctx.dispatch, unit.ID, strings.TrimSpace(p.RuleID))
	if !ok {
		return VerifiedProposal{}, findingError160("RULE_NOT_DISPATCHED", "rule %s was not dispatched to %s", p.RuleID, unit.ID)
	}
	resolvedAnchor, anchorDigest, err := verifyAnchor160(ctx, unit, dispatch, p.Anchor, p.EvidenceRefs)
	if err != nil {
		return VerifiedProposal{}, err
	}
	verifiedEvidence, evidenceDigest, err := verifyEvidence160(ctx, unit, dispatch, p.EvidenceRefs)
	if err != nil {
		return VerifiedProposal{}, err
	}
	anchorPath := resolvedAnchor.Path
	if anchorPath != "" {
		isTest := isTestPath160(anchorPath)
		if isTest && p.Category != "TEST_VALIDITY" {
			return VerifiedProposal{}, findingError160("FINDING_PROPOSAL_INVALID", "test path %s only permits TEST_VALIDITY proposals", anchorPath)
		}
		if !isTest && p.Category == "TEST_VALIDITY" {
			return VerifiedProposal{}, findingError160("FINDING_PROPOSAL_INVALID", "TEST_VALIDITY proposal must anchor in test scope")
		}
	}
	if p.IntroducedByChange && !introducedByChangeVerified160(ctx, unit, resolvedAnchor, verifiedEvidence) {
		return VerifiedProposal{}, findingError160("FINDING_INTRODUCED_BY_CHANGE_NOT_VERIFIED", "proposal %s is not tied to a verified changed hunk or contract relation", p.ProposalID)
	}
	p.Anchor = resolvedAnchor
	p.EvidenceRefs = verifiedEvidence
	return VerifiedProposal{Proposal: p, AnchorDigest: anchorDigest, EvidenceDigest: evidenceDigest}, nil
}

func loadDispatchAuthority160(root, runID string, units reviewunit.Manifest) (reviewrules.Manifest, error) {
	artifact := filepath.Join(root, ".code-harness", "runs", runID, "analysis", "rule-dispatch.json")
	raw, err := os.ReadFile(artifact)
	if err != nil {
		return reviewrules.Manifest{}, findingError160("FINDING_PROPOSAL_INVALID", "read RuleDispatch authority: %v", err)
	}
	schemaBytes, err := os.ReadFile(filepath.Join(root, ".code-harness", "contracts", "rule-dispatch.schema.json"))
	if err != nil {
		return reviewrules.Manifest{}, findingError160("FINDING_PROPOSAL_INVALID", "read RuleDispatch schema: %v", err)
	}
	if err := schema.ValidateJSON(schemaBytes, raw); err != nil {
		return reviewrules.Manifest{}, findingError160("FINDING_PROPOSAL_INVALID", "RuleDispatch schema invalid: %v", err)
	}
	var manifest reviewrules.Manifest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&manifest); err != nil {
		return reviewrules.Manifest{}, findingError160("FINDING_PROPOSAL_INVALID", "decode RuleDispatch: %v", err)
	}
	if manifest.RunID != runID || manifest.ReviewUnitsSHA256 != units.SHA256 {
		return reviewrules.Manifest{}, findingError160("FINDING_PROPOSAL_INVALID", "RuleDispatch is stale for current ReviewUnit")
	}
	wantSHA := strings.TrimSpace(manifest.SHA256)
	candidate := manifest
	candidate.SHA256 = ""
	unsigned, err := reviewrules.CanonicalBytes(candidate)
	if err != nil {
		return reviewrules.Manifest{}, findingError160("FINDING_PROPOSAL_INVALID", "canonicalize RuleDispatch: %v", err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(unsigned)) != wantSHA {
		return reviewrules.Manifest{}, findingError160("FINDING_PROPOSAL_INVALID", "RuleDispatch sha256 mismatch")
	}
	canonical, err := reviewrules.CanonicalBytes(manifest)
	if err != nil || !bytes.Equal(raw, canonical) {
		return reviewrules.Manifest{}, findingError160("FINDING_PROPOSAL_INVALID", "RuleDispatch bytes are not canonical")
	}
	rules, catalogSHA, err := reviewrules.LoadCatalog(filepath.Join(root, ".code-harness", "review-rules", "spring-v1.yaml"))
	if err != nil {
		return reviewrules.Manifest{}, findingError160("FINDING_PROPOSAL_INVALID", "load current rule catalog: %v", err)
	}
	expected, err := reviewrules.BuildDispatch(units, rules, catalogSHA)
	if err != nil {
		return reviewrules.Manifest{}, findingError160("FINDING_PROPOSAL_INVALID", "rebuild RuleDispatch: %v", err)
	}
	expectedBytes, err := reviewrules.CanonicalBytes(expected)
	if err != nil || !bytes.Equal(canonical, expectedBytes) {
		return reviewrules.Manifest{}, findingError160("FINDING_PROPOSAL_INVALID", "RuleDispatch no longer matches current ReviewUnit/catalog")
	}
	return manifest, nil
}

func findUnit160(m reviewunit.Manifest, id string) (reviewunit.Unit, bool) {
	for _, unit := range m.Units {
		if strings.TrimSpace(unit.ID) == id {
			return unit, true
		}
	}
	return reviewunit.Unit{}, false
}

func findDispatch160(m reviewrules.Manifest, unitID, ruleID string) (reviewrules.Dispatch, bool) {
	for _, dispatch := range m.Dispatches {
		if strings.TrimSpace(dispatch.ReviewUnitID) == unitID && strings.TrimSpace(dispatch.RuleID) == ruleID {
			return dispatch, true
		}
	}
	return reviewrules.Dispatch{}, false
}

func isTestPath160(p string) bool {
	p = strings.ToLower(strings.ReplaceAll(p, "\\", "/"))
	return strings.HasPrefix(p, "src/test/") || strings.Contains(p, "/src/test/")
}

func findingError160(code, format string, args ...any) error {
	return fmt.Errorf("%s: %s", code, fmt.Sprintf(format, args...))
}
