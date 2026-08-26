package chain

import (
	"strings"
	"testing"
)

const task153DependencyWorkspace = "company-framework"
const task153DependencySymbol = "AbstractTemplate.execute"
const task153DependencyPath = "company-framework/src/main/java/com/company/framework/AbstractTemplate.java"

func task153WorkspaceEditEvidence() ChangeAnalysisEvidence {
	evidence := task153EditEvidence()
	evidence.SymbolLocations = append(evidence.SymbolLocations, SymbolLocationEvidence{
		Workspace: task153DependencyWorkspace,
		Symbol:    task153DependencySymbol,
		Path:      task153DependencyPath,
		Role:      "Service",
		Source:    "WORKSPACE_DEPENDENCY",
	})
	evidence.CallChains = append(evidence.CallChains,
		CallChainEvidence{
			EntryPoint: "OrderController.submit",
			Chain:      []string{"OrderController.submit", "ServiceA.step", task153DependencySymbol, "ServiceB.step"},
		},
		CallChainEvidence{
			EntryPoint: "OrderController.submit",
			Chain:      []string{"OrderController.submit", task153DependencySymbol, "ServiceB.step"},
		},
	)
	return evidence
}

func Test153EditAddsVerifiedWorkspaceDependencyNode(t *testing.T) {
	evidence := task153WorkspaceEditEvidence()
	candidate, err := applyEditOperations153(
		task153EditExistingChain(),
		[]EditOperation{{Type: "ADD_NODE", Symbol: task153DependencySymbol, After: "ServiceA.step"}},
		evidence,
	)
	if err != nil {
		t.Fatalf("verified Workspace Dependency ADD_NODE must be accepted: %v", err)
	}
	if len(candidate.Nodes) != 3 {
		t.Fatalf("nodes=%d want=3", len(candidate.Nodes))
	}
	added := candidate.Nodes[1]
	if added.Workspace != task153DependencyWorkspace || added.Symbol != task153DependencySymbol || added.Path != task153DependencyPath || added.Role != "SERVICE" {
		t.Fatalf("dependency node identity was not preserved: %+v", added)
	}
	if err := verifyEditedChainFacts153(t.TempDir(), candidate, evidence); err != nil {
		t.Fatalf("verified dependency ADD_NODE result must pass ordered Chain validation: %v", err)
	}
}

func Test153EditReplacesWithVerifiedWorkspaceDependencyNode(t *testing.T) {
	evidence := task153WorkspaceEditEvidence()
	candidate, err := applyEditOperations153(
		task153EditExistingChain(),
		[]EditOperation{{Type: "REPLACE_NODE", From: "ServiceA.step", To: task153DependencySymbol}},
		evidence,
	)
	if err != nil {
		t.Fatalf("verified Workspace Dependency REPLACE_NODE must be accepted: %v", err)
	}
	replaced := candidate.Nodes[0]
	if replaced.Workspace != task153DependencyWorkspace || replaced.Symbol != task153DependencySymbol || replaced.Path != task153DependencyPath || replaced.Role != "SERVICE" {
		t.Fatalf("replacement dependency identity was not preserved: %+v", replaced)
	}
	if err := verifyEditedChainFacts153(t.TempDir(), candidate, evidence); err != nil {
		t.Fatalf("verified dependency REPLACE_NODE result must pass ordered Chain validation: %v", err)
	}
}

func Test153EditRejectsWorkspaceDependencyImpersonatedAsCurrent(t *testing.T) {
	evidence := task153WorkspaceEditEvidence()
	candidate := task153EditExistingChain()
	candidate.Nodes = []Node{
		{Workspace: CurrentWorkspace, Symbol: "ServiceA.step", Path: "src/main/java/com/example/ServiceA.java", Role: "SERVICE"},
		{Workspace: CurrentWorkspace, Symbol: task153DependencySymbol, Path: task153DependencyPath, Role: "SERVICE"},
		{Workspace: CurrentWorkspace, Symbol: "ServiceB.step", Path: "src/main/java/com/example/ServiceB.java", Role: "SERVICE"},
	}
	if err := verifyEditedChainFacts153(t.TempDir(), candidate, evidence); err == nil || !strings.Contains(err.Error(), "CHAIN_EDIT_FACT_NOT_VERIFIED") {
		t.Fatalf("dependency node presented as current must fail closed, got %v", err)
	}
}

func Test153EditRejectsSameSymbolAcrossWorkspacesAsAmbiguous(t *testing.T) {
	evidence := task153WorkspaceEditEvidence()
	evidence.SymbolLocations = append(evidence.SymbolLocations, SymbolLocationEvidence{
		Workspace: CurrentWorkspace,
		Symbol:    task153DependencySymbol,
		Path:      "src/main/java/com/example/AbstractTemplate.java",
		Role:      "Service",
		Source:    "FIND_SYMBOL",
	})
	_, err := applyEditOperations153(
		task153EditExistingChain(),
		[]EditOperation{{Type: "ADD_NODE", Symbol: task153DependencySymbol, After: "ServiceA.step"}},
		evidence,
	)
	if err == nil || !strings.Contains(err.Error(), "CHAIN_EDIT_FACT_NOT_VERIFIED") {
		t.Fatalf("same exact symbol across workspaces must be rejected as ambiguous, got %v", err)
	}
}
