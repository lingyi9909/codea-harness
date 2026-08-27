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
)

func verifyAnchor160(ctx VerifyContext, unit reviewunit.Unit, dispatch reviewrules.Dispatch, anchor Anchor, evidence []EvidenceRef) (Anchor, string, error) {
	resolved := anchor
	switch anchor.Kind {
	case AnchorLine:
		p, ok := safeFindingPath160(anchor.Path)
		if !ok {
			return Anchor{}, "", findingError160("FINDING_SCOPE_VIOLATION", "invalid anchor path %q", anchor.Path)
		}
		if dependencyPath160(ctx, p) {
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
			info, symbolPath, err := verifyCurrentSymbol160(ctx, unit, resolved.Symbol)
			if err != nil {
				return Anchor{}, "", err
			}
			if symbolPath != p || info.LineStart < 1 || info.LineEnd < info.LineStart || anchor.Line < info.LineStart || anchor.Line > info.LineEnd {
				return Anchor{}, "", findingError160("FINDING_ANCHOR_NOT_VERIFIED", "line %s:%d is outside symbol %s range", p, anchor.Line, resolved.Symbol)
			}
		}
	case AnchorSymbol:
		symbol := strings.TrimSpace(anchor.Symbol)
		info, symbolPath, err := verifyCurrentSymbol160(ctx, unit, symbol)
		if err != nil {
			return Anchor{}, "", err
		}
		if anchor.Path != "" {
			claimed, ok := safeFindingPath160(anchor.Path)
			if !ok || claimed != symbolPath {
				return Anchor{}, "", findingError160("FINDING_ANCHOR_NOT_VERIFIED", "symbol %s does not belong to claimed path", symbol)
			}
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
		if dependencyPath160(ctx, p) {
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
		if len(evidence) < 2 {
			return Anchor{}, "", findingError160("FINDING_ANCHOR_NOT_VERIFIED", "CHANGESET anchor requires at least two evidence refs")
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
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return info, "", findingError160("FINDING_ANCHOR_NOT_VERIFIED", "empty symbol")
	}
	currentPath := ""
	dependency := false
	for _, loc := range ctx.analysis.SymbolLocations {
		if strings.TrimSpace(loc.Symbol) != symbol {
			continue
		}
		p, ok := safeFindingPath160(loc.Path)
		if !ok {
			continue
		}
		workspace := strings.TrimSpace(loc.Workspace)
		if workspace == "" || workspace == "current" {
			if currentPath != "" && currentPath != p {
				return info, "", findingError160("FINDING_ANCHOR_NOT_VERIFIED", "symbol %s has ambiguous current paths", symbol)
			}
			currentPath = p
		} else {
			dependency = true
		}
	}
	if currentPath == "" {
		if dependency {
			return info, "", findingError160("FINDING_DEPENDENCY_SCOPE_FORBIDDEN", "symbol %s resolves only to dependency workspace", symbol)
		}
		return info, "", findingError160("FINDING_ANCHOR_NOT_VERIFIED", "symbol %s is not verified by Certified Analysis", symbol)
	}
	if !unitCurrentPath160(unit, currentPath) {
		return info, "", findingError160("FINDING_SCOPE_VIOLATION", "symbol %s path %s is outside ReviewUnit", symbol, currentPath)
	}
	rangeInfo, ok := ctx.symbolRanges[symbol]
	if !ok {
		return info, "", findingError160("FINDING_ANCHOR_NOT_VERIFIED", "symbol %s has no pinned navigation range", symbol)
	}
	rangePath, ok := safeFindingPath160(rangeInfo.Path)
	if !ok || rangePath != currentPath || rangeInfo.Symbol != symbol {
		return info, "", findingError160("FINDING_ANCHOR_NOT_VERIFIED", "symbol %s navigation evidence does not match Certified Analysis", symbol)
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
		if ok && p == candidate && strings.TrimSpace(file.Workspace) == "current" {
			return true
		}
	}
	return false
}

func dependencyPath160(ctx VerifyContext, candidate string) bool {
	for _, loc := range ctx.analysis.SymbolLocations {
		p, ok := safeFindingPath160(loc.Path)
		if ok && p == candidate {
			workspace := strings.TrimSpace(loc.Workspace)
			if workspace != "" && workspace != "current" {
				return true
			}
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
