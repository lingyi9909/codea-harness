package finding

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"codea-harness-tools/internal/reviewrules"
	"codea-harness-tools/internal/reviewunit"
	"codea-harness-tools/internal/symbolid"
)

func verifyAnchor160(ctx VerifyContext, unit reviewunit.Unit, dispatch reviewrules.Dispatch, anchor Anchor, evidence []EvidenceRef) (Anchor, string, error) {
	resolved := anchor
	switch anchor.Kind {
	case AnchorLine:
		p, ok := safeFindingPath160(anchor.Path)
		if !ok {
			return Anchor{}, "", findingError160("FINDING_SCOPE_VIOLATION", "invalid anchor path %q", anchor.Path)
		}
		if isDependencyPath160(ctx, p) {
			return Anchor{}, "", findingError160("FINDING_DEPENDENCY_SCOPE_FORBIDDEN", "dependency path %s", p)
		}
		if !unitCurrentPath160(unit, p) {
			return Anchor{}, "", findingError160("FINDING_SCOPE_VIOLATION", "anchor path %s is outside ReviewUnit", p)
		}
		if !sourceLineExists160(ctx.repoRoot, p, anchor.Line) {
			return Anchor{}, "", findingError160("FINDING_ANCHOR_NOT_VERIFIED", "line %d does not exist in %s", anchor.Line, p)
		}
		resolved.Path = p
		resolved.Symbol = strings.TrimSpace(anchor.Symbol)
		if resolved.Symbol != "" {
			info, symbolPath, err := verifyCurrentSymbolAtPath160(ctx, unit, resolved.Symbol, p)
			if err != nil {
				return Anchor{}, "", err
			}
			if symbolPath != p || info.LineStart < 1 || info.LineEnd < info.LineStart || anchor.Line < info.LineStart || anchor.Line > info.LineEnd {
				return Anchor{}, "", findingError160("FINDING_ANCHOR_NOT_VERIFIED", "line %s:%d is outside symbol %s range", p, anchor.Line, resolved.Symbol)
			}
		}
	case AnchorSymbol:
		symbol := strings.TrimSpace(anchor.Symbol)
		claimed := ""
		if anchor.Path != "" {
			var ok bool
			claimed, ok = safeFindingPath160(anchor.Path)
			if !ok {
				return Anchor{}, "", findingError160("FINDING_ANCHOR_NOT_VERIFIED", "invalid claimed path %q for symbol %s", anchor.Path, symbol)
			}
		}
		info, symbolPath, err := verifyCurrentSymbolAtPath160(ctx, unit, symbol, claimed)
		if err != nil {
			return Anchor{}, "", err
		}
		if claimed != "" && claimed != symbolPath {
			return Anchor{}, "", findingError160("FINDING_ANCHOR_NOT_VERIFIED", "symbol %s does not belong to claimed path", symbol)
		}
		if info.LineStart < 1 || info.LineEnd < info.LineStart || !sourceLineExists160(ctx.repoRoot, symbolPath, info.LineStart) || !sourceLineExists160(ctx.repoRoot, symbolPath, info.LineEnd) {
			return Anchor{}, "", findingError160("FINDING_ANCHOR_NOT_VERIFIED", "symbol %s source range is not present in current bytes", symbol)
		}
		resolved.Path = symbolPath
		resolved.Symbol = symbol
		resolved.Line = 0
	case AnchorFile:
		p, ok := safeFindingPath160(anchor.Path)
		if !ok {
			return Anchor{}, "", findingError160("FINDING_SCOPE_VIOLATION", "invalid file anchor path %q", anchor.Path)
		}
		if isDependencyPath160(ctx, p) {
			return Anchor{}, "", findingError160("FINDING_DEPENDENCY_SCOPE_FORBIDDEN", "dependency path %s", p)
		}
		if !unitCurrentPath160(unit, p) {
			return Anchor{}, "", findingError160("FINDING_SCOPE_VIOLATION", "file anchor %s is outside ReviewUnit", p)
		}
		if !ruleAllowsFileAnchor160(dispatch) {
			return Anchor{}, "", findingError160("FINDING_ANCHOR_NOT_VERIFIED", "rule %s requires a more specific anchor", dispatch.RuleID)
		}
		if _, err := os.Stat(filepath.Join(ctx.repoRoot, filepath.FromSlash(p))); err != nil {
			return Anchor{}, "", findingError160("FINDING_ANCHOR_NOT_VERIFIED", "file anchor %s is not present in current bytes", p)
		}
		resolved.Path = p
	case AnchorChangeSet:
		verifiedEvidence, err := canonicalVerifiedEvidence160(ctx, unit, evidence)
		if err != nil {
			return Anchor{}, "", err
		}
		if len(verifiedEvidence) < 2 {
			return Anchor{}, "", findingError160("FINDING_ANCHOR_NOT_VERIFIED", "CHANGESET anchor requires at least two distinct verified evidence refs")
		}
	default:
		return Anchor{}, "", findingError160("FINDING_ANCHOR_NOT_VERIFIED", "unsupported anchor kind %q", anchor.Kind)
	}
	data, err := json.Marshal(resolved)
	if err != nil {
		return Anchor{}, "", findingError160("FINDING_ANCHOR_NOT_VERIFIED", "canonicalize anchor: %v", err)
	}
	return resolved, fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

func verifyCurrentSymbol160(ctx VerifyContext, unit reviewunit.Unit, symbol string) (info struct {
	Symbol string
	Path string
	LineStart int
	LineEnd int
}, symbolPath string, err error) {
	return verifyCurrentSymbolAtPath160(ctx, unit, symbol, "")
}

func verifyCurrentSymbolAtPath160(ctx VerifyContext, unit reviewunit.Unit, symbol, claimedPath string) (info struct {
	Symbol string
	Path string
	LineStart int
	LineEnd int
}, symbolPath string, err error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return info, "", findingError160("FINDING_ANCHOR_NOT_VERIFIED", "empty symbol")
	}
	claimed := ""
	if strings.TrimSpace(claimedPath) != "" {
		var ok bool
		claimed, ok = safeFindingPath160(claimedPath)
		if !ok {
			return info, "", findingError160("FINDING_ANCHOR_NOT_VERIFIED", "invalid claimed path %q for symbol %s", claimedPath, symbol)
		}
		if !unitCurrentPath160(unit, claimed) {
			return info, "", findingError160("FINDING_SCOPE_VIOLATION", "symbol %s claimed path %s is outside ReviewUnit", symbol, claimed)
		}
	}

	candidates := map[string]struct{}{}
	currentSeen := false
	dependency := false
	for _, loc := range ctx.analysis.SymbolLocations {
		if strings.TrimSpace(loc.Symbol) != symbol {
			continue
		}
		p, ok := safeFindingPath160(loc.Path)
		if !ok {
			continue
		}
		if symbolid.NormalizeWorkspace(loc.Workspace) != symbolid.CurrentWorkspace {
			dependency = true
			continue
		}
		currentSeen = true
		if claimed != "" && p != claimed {
			continue
		}
		if !unitCurrentPath160(unit, p) {
			continue
		}
		candidates[p] = struct{}{}
	}
	if len(candidates) == 0 {
		if !currentSeen && dependency {
			return info, "", findingError160("FINDING_DEPENDENCY_SCOPE_FORBIDDEN", "symbol %s resolves only to dependency workspace", symbol)
		}
		if currentSeen {
			if claimed != "" {
				return info, "", findingError160("FINDING_ANCHOR_NOT_VERIFIED", "symbol %s is not verified at claimed path %s", symbol, claimed)
			}
			return info, "", findingError160("FINDING_SCOPE_VIOLATION", "symbol %s has no current path inside ReviewUnit", symbol)
		}
		return info, "", findingError160("FINDING_ANCHOR_NOT_VERIFIED", "symbol %s is not verified by Certified Analysis", symbol)
	}
	if len(candidates) != 1 {
		return info, "", findingError160("FINDING_ANCHOR_NOT_VERIFIED", "symbol %s has ambiguous current paths inside ReviewUnit", symbol)
	}
	currentPath := ""
	for p := range candidates { currentPath = p }

	ref := symbolid.Ref{Workspace: symbolid.CurrentWorkspace, Path: currentPath, Symbol: symbol}
	key, _ := symbolid.Key(ref)
	rangeInfo, ok := ctx.symbolRanges[key]
	if !ok {
		// Backward compatibility for Runtime-owned single-path contexts and older
		// focused unit tests. The path must still match the exact authority path.
		rangeInfo, ok = ctx.symbolRanges[symbol]
	}
	if !ok {
		return info, "", findingError160("FINDING_ANCHOR_NOT_VERIFIED", "symbol %s at %s has no pinned navigation range", symbol, currentPath)
	}
	rangePath, ok := safeFindingPath160(rangeInfo.Path)
	if !ok || rangePath != currentPath || strings.TrimSpace(rangeInfo.Symbol) != symbol {
		return info, "", findingError160("FINDING_ANCHOR_NOT_VERIFIED", "symbol %s navigation evidence does not match Certified Analysis identity %s", symbol, currentPath)
	}
	info.Symbol = rangeInfo.Symbol
	info.Path = rangePath
	info.LineStart = rangeInfo.LineStart
	info.LineEnd = rangeInfo.LineEnd
	return info, currentPath, nil
}

func ruleAllowsFileAnchor160(dispatch reviewrules.Dispatch) bool {
	for _, required := range dispatch.RequiredEvidence {
		switch strings.ToUpper(strings.TrimSpace(required)) {
		case "SYMBOL", "CHAIN":
			return false
		}
	}
	return true
}

func safeFindingPath160(raw string) (string, bool) {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if raw == "" || path.IsAbs(raw) {
		return "", false
	}
	clean := path.Clean(raw)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	return clean, true
}

func unitCurrentPath160(unit reviewunit.Unit, candidate string) bool {
	for _, file := range unit.Files {
		p, ok := safeFindingPath160(file.Path)
		if ok && p == candidate && symbolid.NormalizeWorkspace(file.Workspace) == symbolid.CurrentWorkspace {
			return true
		}
	}
	return false
}

func isDependencyPath160(ctx VerifyContext, candidate string) bool {
	for _, loc := range ctx.analysis.SymbolLocations {
		p, ok := safeFindingPath160(loc.Path)
		if ok && p == candidate && symbolid.NormalizeWorkspace(loc.Workspace) != symbolid.CurrentWorkspace {
			return true
		}
	}
	return false
}

func sourceLineExists160(root, rel string, line int) bool {
	if line < 1 {
		return false
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return false
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	return line <= len(lines)
}
