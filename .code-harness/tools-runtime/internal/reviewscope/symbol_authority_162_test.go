package reviewscope

import "testing"

func Test162NavigationEvidenceRetainsDuplicateSymbolsAcrossModulePaths(t *testing.T) {
	evidence, err := buildNavigationEvidence([]SymbolLocation{
		{Workspace: "current", Symbol: "UserService.create", Path: "module-a/src/main/java/com/acme/UserService.java", Role: "Service", Source: "FIND_SYMBOL"},
		{Workspace: "current", Symbol: "UserService.create", Path: "module-b/src/main/java/com/acme/UserService.java", Role: "Service", Source: "FIND_SYMBOL"},
	})
	if err != nil {
		t.Fatalf("duplicate symbol names in distinct module paths must remain valid navigation evidence: %v", err)
	}
	if len(evidence.locations) != 2 {
		t.Fatalf("expected two distinct navigation locations, got %d", len(evidence.locations))
	}
}
