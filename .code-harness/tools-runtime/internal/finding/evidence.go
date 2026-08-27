package finding

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"codea-harness-tools/internal/reviewrules"
	"codea-harness-tools/internal/reviewunit"
)

func verifyEvidence160(ctx VerifyContext, unit reviewunit.Unit, dispatch reviewrules.Dispatch, refs []EvidenceRef) ([]EvidenceRef, string, error) {
	verified := make([]EvidenceRef, 0, len(refs))
	kinds := map[string]bool{}
	for _, ref := range refs {
		v, err := verifyEvidenceRef160(ctx, unit, ref)
		if err != nil {
			return nil, "", err
		}
		kind := strings.ToUpper(strings.TrimSpace(v.Kind))
		kinds[kind] = true
		verified = append(verified, v)
	}
	for _, required := range dispatch.RequiredEvidence {
		kind := strings.ToUpper(strings.TrimSpace(required))
		if kind != "" && !kinds[kind] {
			return nil, "", findingError160("FINDING_EVIDENCE_NOT_VERIFIED", "rule %s requires %s evidence", dispatch.RuleID, kind)
		}
	}
	sort.Slice(verified, func(i, j int) bool {
		li, _ := json.Marshal(verified[i])
		lj, _ := json.Marshal(verified[j])
		return string(li) < string(lj)
	})
	data, err := json.Marshal(verified)
	if err != nil {
		return nil, "", findingError160("FINDING_EVIDENCE_NOT_VERIFIED", "canonicalize evidence: %v", err)
	}
	return verified, fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

func verifyEvidenceRef160(ctx VerifyContext, unit reviewunit.Unit, ref EvidenceRef) (EvidenceRef, error) {
	v := ref
	v.Kind = strings.ToUpper(strings.TrimSpace(ref.Kind))
	v.Value = strings.TrimSpace(ref.Value)
	if ref.Path != "" {
		p, ok := safeFindingPath160(ref.Path)
		if !ok {
			return EvidenceRef{}, findingError160("FINDING_EVIDENCE_NOT_VERIFIED", "invalid evidence path %q", ref.Path)
		}
		if isDependencyPath160(ctx, p) {
			return EvidenceRef{}, findingError160("FINDING_DEPENDENCY_SCOPE_FORBIDDEN", "dependency evidence path %s", p)
		}
		if !unitCurrentPath160(unit, p) {
			return EvidenceRef{}, findingError160("FINDING_EVIDENCE_NOT_VERIFIED", "evidence path %s is outside ReviewUnit", p)
		}
		v.Path = p
	}

	switch v.Kind {
	case "SYMBOL":
		_, symbolPath, err := verifyCurrentSymbol160(ctx, unit, v.Value)
		if err != nil {
			if strings.Contains(err.Error(), "FINDING_SCOPE_VIOLATION") {
				return EvidenceRef{}, findingError160("FINDING_EVIDENCE_NOT_VERIFIED", "symbol evidence is outside ReviewUnit")
			}
			return EvidenceRef{}, err
		}
		if v.Path != "" && v.Path != symbolPath {
			return EvidenceRef{}, findingError160("FINDING_EVIDENCE_NOT_VERIFIED", "symbol %s does not belong to %s", v.Value, v.Path)
		}
		v.Path = symbolPath
	case "CHAIN":
		if !unitHasSymbol160(unit, v.Value) {
			return EvidenceRef{}, findingError160("FINDING_EVIDENCE_NOT_VERIFIED", "chain symbol %s is not in ReviewUnit", v.Value)
		}
	case "SOURCE_RANGE":
		if v.Path == "" || !validCurrentRange160(ctx, v.Path, v.StartLine, v.EndLine) {
			return EvidenceRef{}, findingError160("FINDING_EVIDENCE_NOT_VERIFIED", "source range is not present in current bytes")
		}
		if v.Value != "" {
			info, symbolPath, err := verifyCurrentSymbol160(ctx, unit, v.Value)
			if err != nil || symbolPath != v.Path || v.StartLine < info.LineStart || v.EndLine > info.LineEnd {
				return EvidenceRef{}, findingError160("FINDING_EVIDENCE_NOT_VERIFIED", "source range is outside symbol %s", v.Value)
			}
		}
	case "CHANGED_RANGE":
		if v.Path == "" || !validCurrentRange160(ctx, v.Path, v.StartLine, v.EndLine) || !rangeOverlapsHunk160(unit, v.Path, v.StartLine, v.EndLine) {
			return EvidenceRef{}, findingError160("FINDING_EVIDENCE_NOT_VERIFIED", "changed range is not verified by ReviewUnit hunks")
		}
	case "RESOURCE_RELATION":
		if v.Path == "" || !resourceRelationVerified160(ctx, unit, v) {
			return EvidenceRef{}, findingError160("FINDING_EVIDENCE_NOT_VERIFIED", "resource relation is not verified")
		}
	default:
		return EvidenceRef{}, findingError160("FINDING_EVIDENCE_NOT_VERIFIED", "unsupported evidence kind %q", v.Kind)
	}
	return v, nil
}

func unitHasSymbol160(unit reviewunit.Unit, symbol string) bool {
	symbol = strings.TrimSpace(symbol)
	for _, s := range unit.Chain {
		if strings.TrimSpace(s) == symbol {
			return true
		}
	}
	for _, s := range unit.ContextSymbols {
		if strings.TrimSpace(s) == symbol {
			return true
		}
	}
	return false
}

func validCurrentRange160(ctx VerifyContext, p string, start, end int) bool {
	return start >= 1 && end >= start && sourceLineExists160(ctx.repoRoot, p, start) && sourceLineExists160(ctx.repoRoot, p, end)
}

func rangeOverlapsHunk160(unit reviewunit.Unit, p string, start, end int) bool {
	for _, h := range unit.ChangedHunks {
		hp, ok := safeFindingPath160(h.Path)
		if !ok || hp != p || h.NewStart < 1 || h.NewLines < 1 {
			continue
		}
		hEnd := h.NewStart + h.NewLines - 1
		if start <= hEnd && end >= h.NewStart {
			return true
		}
	}
	return false
}

func resourceRelationVerified160(ctx VerifyContext, unit reviewunit.Unit, ref EvidenceRef) bool {
	for _, relation := range ctx.analysis.ResourceRelations {
		p, ok := safeFindingPath160(relation.Path)
		if !ok || p != ref.Path || !unitCurrentPath160(unit, p) {
			continue
		}
		from := strings.TrimSpace(relation.FromSymbol)
		if from != "" && !unitHasSymbol160(unit, from) {
			continue
		}
		if ref.Value == "" || ref.Value == strings.TrimSpace(relation.Resource) || ref.Value == from || ref.Value == strings.TrimSpace(relation.Evidence) {
			return true
		}
	}
	return false
}

func introducedByChangeVerified160(ctx VerifyContext, unit reviewunit.Unit, anchor Anchor, refs []EvidenceRef) bool {
	if anchor.Kind == AnchorLine && anchor.Path != "" && rangeOverlapsHunk160(unit, anchor.Path, anchor.Line, anchor.Line) {
		return true
	}
	for _, ref := range refs {
		switch strings.ToUpper(strings.TrimSpace(ref.Kind)) {
		case "CHANGED_RANGE", "SOURCE_RANGE":
			if ref.Path != "" && rangeOverlapsHunk160(unit, ref.Path, ref.StartLine, ref.EndLine) {
				return true
			}
		case "RESOURCE_RELATION":
			if ref.Path != "" && rangeOverlapsAnyHunk160(unit, ref.Path) {
				return true
			}
			for _, relation := range ctx.analysis.ResourceRelations {
				p, ok := safeFindingPath160(relation.Path)
				if !ok || p != ref.Path {
					continue
				}
				for _, loc := range ctx.analysis.SymbolLocations {
					if strings.TrimSpace(loc.Symbol) != strings.TrimSpace(relation.FromSymbol) {
						continue
					}
					lp, ok := safeFindingPath160(loc.Path)
					if ok && rangeOverlapsAnyHunk160(unit, lp) {
						return true
					}
				}
			}
		}
	}
	return false
}

func rangeOverlapsAnyHunk160(unit reviewunit.Unit, p string) bool {
	for _, h := range unit.ChangedHunks {
		hp, ok := safeFindingPath160(h.Path)
		if ok && hp == p && h.NewStart > 0 && h.NewLines > 0 {
			return true
		}
	}
	return false
}
