package analysis

import (
	"strings"
	"testing"
)

func TestValidateSemanticProposalRefs167(t *testing.T) {
	locations := []SymbolLocation{
		{Workspace: "current", Path: "src/A.java", Symbol: "A.run"},
		{Workspace: "current", Path: "src/B.java", Symbol: "B.go"},
	}
	valid := ChangeAnalysis{SymbolLocations: locations, CallChains: []CallChain{{
		EntryPoint: "A.run", Chain: []string{"A.run", "B.go"},
		EntryPointRef: &SymbolRef{Workspace: "current", Path: "src/A.java", Symbol: "A.run"},
		ChainRefs:     []SymbolRef{{Workspace: "current", Path: "src/A.java", Symbol: "A.run"}, {Workspace: "current", Path: "src/B.java", Symbol: "B.go"}},
	}}}
	if err := ValidateSemanticProposalRefs167(valid); err != nil {
		t.Fatalf("valid refs: %v", err)
	}

	legacy := ChangeAnalysis{SymbolLocations: locations, CallChains: []CallChain{{EntryPoint: "A.run", Chain: []string{"A.run", "B.go"}}}}
	if err := ValidateSemanticProposalRefs167(legacy); err != nil {
		t.Fatalf("unambiguous legacy refs: %v", err)
	}

	cases := []struct {
		name, want string
		mutate     func(*ChangeAnalysis)
	}{
		{"length", "length", func(a *ChangeAnalysis) { a.CallChains[0].ChainRefs = a.CallChains[0].ChainRefs[:1] }},
		{"order", "does not match", func(a *ChangeAnalysis) { a.CallChains[0].ChainRefs[1].Symbol = "A.run" }},
		{"entry", "entryPointRef", func(a *ChangeAnalysis) { a.CallChains[0].EntryPointRef.Symbol = "B.go" }},
		{"missing", "exact location", func(a *ChangeAnalysis) { a.CallChains[0].ChainRefs[1].Path = "src/Missing.java" }},
		{"ambiguous legacy", "ambiguous", func(a *ChangeAnalysis) {
			a.CallChains[0].ChainRefs = nil
			a.CallChains[0].EntryPointRef = nil
			a.SymbolLocations = append(a.SymbolLocations, SymbolLocation{Workspace: "dependency", Path: "dep/A.java", Symbol: "A.run"})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := valid
			a.SymbolLocations = append([]SymbolLocation(nil), valid.SymbolLocations...)
			a.CallChains = append([]CallChain(nil), valid.CallChains...)
			a.CallChains[0].ChainRefs = append([]SymbolRef(nil), valid.CallChains[0].ChainRefs...)
			ref := *valid.CallChains[0].EntryPointRef
			a.CallChains[0].EntryPointRef = &ref
			tc.mutate(&a)
			if err := ValidateSemanticProposalRefs167(a); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v want %q", err, tc.want)
			}
		})
	}
}
