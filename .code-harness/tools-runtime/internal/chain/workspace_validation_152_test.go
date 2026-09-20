package chain

import "testing"

func Test152ChainValidationRequiresMatchingWorkspaceEvidence(t *testing.T) {
	root := t.TempDir()
	analysis := ChangeAnalysisEvidence{
		AffectedControllers: []AffectedControllerEvidence{{
			Controller: "XxxController",
			Endpoints: []string{"XxxController.submit"},
			ImpactType: "DIRECT_CHANGE",
			SourceSymbols: []string{"XxxServiceImpl.submit"},
		}},
		CallChains: []CallChainEvidence{{
			EntryPoint: "XxxController.submit",
			Chain: []string{"XxxController.submit", "XxxServiceImpl.submit", "AbstractTemplate.execute"},
		}},
		SymbolLocations: []SymbolLocationEvidence{
			{Workspace: CurrentWorkspace, Symbol: "XxxController.submit", Path: "src/main/java/XxxController.java", Role: "Controller", Source: "FIND_SYMBOL"},
			{Workspace: CurrentWorkspace, Symbol: "XxxServiceImpl.submit", Path: "src/main/java/XxxServiceImpl.java", Role: "Service", Source: "FIND_SYMBOL"},
			{Workspace: "company-framework", Symbol: "AbstractTemplate.execute", Path: "src/main/java/AbstractTemplate.java", Role: "Service", Source: "WORKSPACE_INHERITANCE", From: "XxxServiceImpl.submit"},
		},
	}
	chain := Chain{
		Version: 1,
		ID:      "chain-152-validation",
		Name:    "XxxController.submit",
		Status:  StatusDiscovered,
		EntryPoints: []EntryPoint{{Workspace: CurrentWorkspace, Symbol: "XxxController.submit", Path: "src/main/java/XxxController.java"}},
		Nodes: []Node{
			{Workspace: CurrentWorkspace, Symbol: "XxxServiceImpl.submit", Path: "src/main/java/XxxServiceImpl.java", Role: "SERVICE"},
			{Workspace: "company-framework", Symbol: "AbstractTemplate.execute", Path: "src/main/java/AbstractTemplate.java", Role: "SERVICE"},
		},
	}

	valid := Validate(root, chain, EvidenceSnapshot(analysis))
	if valid.Status != ValidationValid {
		t.Fatalf("matching workspace evidence must validate: %#v", valid)
	}

	wrong := chain
	wrong.Nodes = append([]Node(nil), chain.Nodes...)
	wrong.Nodes[1].Workspace = CurrentWorkspace
	invalid := Validate(root, wrong, EvidenceSnapshot(analysis))
	if invalid.Status != ValidationInvalid {
		t.Fatalf("workspace mismatch must invalidate chain evidence: %#v", invalid)
	}
}
