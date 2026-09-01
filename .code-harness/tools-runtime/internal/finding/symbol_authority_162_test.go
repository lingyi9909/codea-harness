package finding

import (
	"testing"

	"codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/nav"
	"codea-harness-tools/internal/reviewunit"
	"codea-harness-tools/internal/symbolid"
)

func Test162FindingAnchorUsesExactModulePathForDuplicateSymbol(t *testing.T) {
	ctx := verifyContext160(t)
	moduleA := "module-a/src/main/java/com/acme/UserService.java"
	moduleB := "module-b/src/main/java/com/acme/UserService.java"
	content := "package com.acme;\npublic class UserService {\n  public void create() {}\n}\n"
	writeSource160(t, ctx.repoRoot, moduleA, content)
	writeSource160(t, ctx.repoRoot, moduleB, content)

	ctx.analysis.SymbolLocations = []analysis.SymbolLocation{
		{Workspace: "current", Symbol: "UserService.create", Path: moduleA, Role: "Service", Source: "FIND_SYMBOL"},
		{Workspace: "current", Symbol: "UserService.create", Path: moduleB, Role: "Service", Source: "FIND_SYMBOL"},
	}
	ctx.units.Units = []reviewunit.Unit{{
		ID: "RU-TASK3",
		EntryPoint: "UserService.create",
		Chain: []string{"UserService.create"},
		Files: []reviewunit.FileRef{{Path: moduleB, Role: "Service", Changed: true, Workspace: "current"}},
		ChangedHunks: []reviewunit.HunkRef{{Path: moduleB, NewStart: 3, NewLines: 1}},
	}}
	ctx.symbolRanges = map[string]nav.SymbolInfo{}
	for _, p := range []string{moduleA, moduleB} {
		key, ok := symbolid.Key(symbolid.Ref{Workspace: "current", Path: p, Symbol: "UserService.create"})
		if !ok { t.Fatalf("invalid test symbol ref for %s", p) }
		ctx.symbolRanges[key] = nav.SymbolInfo{Symbol: "UserService.create", Path: p, LineStart: 3, LineEnd: 3}
	}

	p := baseProposal160()
	p.Anchor = Anchor{Kind: AnchorLine, Path: moduleB, Line: 3, Symbol: "UserService.create"}
	p.EvidenceRefs = []EvidenceRef{
		{Kind: "CHAIN", Value: "UserService.create"},
		{Kind: "SYMBOL", Value: "UserService.create", Path: moduleB},
	}
	verified, err := Verify(ctx, p)
	if err != nil { t.Fatalf("path-qualified duplicate finding anchor must verify: %v", err) }
	if verified.Proposal.Anchor.Path != moduleB { t.Fatalf("finding anchor crossed modules: got %q want %q", verified.Proposal.Anchor.Path, moduleB) }
	if len(verified.Proposal.EvidenceRefs) == 0 { t.Fatal("expected verified evidence") }
	for _, ref := range verified.Proposal.EvidenceRefs {
		if ref.Kind == "SYMBOL" && ref.Path != moduleB { t.Fatalf("symbol evidence crossed modules: %+v", ref) }
	}
}
