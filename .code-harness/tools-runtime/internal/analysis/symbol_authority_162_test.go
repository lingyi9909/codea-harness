package analysis

import (
	"strings"
	"testing"
)

func Test162DuplicateModuleSymbolLocationsCanCoexist(t *testing.T) {
	analysis := ChangeAnalysis{SymbolLocations: []SymbolLocation{
		{Workspace: "current", Symbol: "UserService.create", Path: "module-a/src/main/java/com/acme/UserService.java", Role: "Service", Source: "FIND_SYMBOL"},
		{Workspace: "current", Symbol: "UserService.create", Path: "module-b/src/main/java/com/acme/UserService.java", Role: "Service", Source: "FIND_SYMBOL"},
	}}
	if err := validateEvidenceAtRoot153(t.TempDir(), analysis, EntrypointInventory{}); err != nil {
		t.Fatalf("same symbol in different module paths must coexist as distinct authority facts: %v", err)
	}
}

func Test162BareDuplicateEntrypointChainDoesNotConfirmBothModulePaths(t *testing.T) {
	inventory := EntrypointInventory{ExpectedEntrypoints: []ExpectedEntrypoint{
		{Symbol: "UserService.create", Path: "module-a/src/main/java/com/acme/UserService.java"},
		{Symbol: "UserService.create", Path: "module-b/src/main/java/com/acme/UserService.java"},
	}}
	proposal := ChangeAnalysis{CallChains: []CallChain{{EntryPoint: "UserService.create", Chain: []string{"UserService.create"}}}}
	err := VerifyEntrypointDispositions(inventory, proposal)
	if err == nil || !strings.Contains(err.Error(), "ENTRYPOINT_COMPLETENESS_INCOMPLETE") {
		t.Fatalf("ambiguous bare symbol chain must not confirm both module entrypoints, got %v", err)
	}
}
