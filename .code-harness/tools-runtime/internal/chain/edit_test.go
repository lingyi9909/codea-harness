package chain

import (
	"reflect"
	"strings"
	"testing"
)

func task153EditExistingChain() Chain {
	return Chain{
		Version: 1,
		ID:      "order-submit",
		Name:    "订单提交",
		Status:  StatusAccepted,
		EntryPoints: []EntryPoint{{
			Workspace: CurrentWorkspace,
			Symbol:    "OrderController.submit",
			Path:      "src/main/java/com/example/OrderController.java",
		}},
		Nodes: []Node{
			{Workspace: CurrentWorkspace, Symbol: "ServiceA.step", Path: "src/main/java/com/example/ServiceA.java", Role: "SERVICE"},
			{Workspace: CurrentWorkspace, Symbol: "ServiceB.step", Path: "src/main/java/com/example/ServiceB.java", Role: "SERVICE"},
		},
		Notes: "old notes",
	}
}

func task153EditEvidence() ChangeAnalysisEvidence {
	return ChangeAnalysisEvidence{
		AffectedControllers: []AffectedControllerEvidence{{
			Controller: "OrderController",
			Endpoints:  []string{"OrderController.submit"},
		}},
		SymbolLocations: []SymbolLocationEvidence{
			{Workspace: CurrentWorkspace, Symbol: "OrderController.submit", Path: "src/main/java/com/example/OrderController.java", Role: "Controller", Source: "FIND_SYMBOL"},
			{Workspace: CurrentWorkspace, Symbol: "ServiceA.step", Path: "src/main/java/com/example/ServiceA.java", Role: "Service", Source: "FIND_SYMBOL"},
			{Workspace: CurrentWorkspace, Symbol: "ServiceB.step", Path: "src/main/java/com/example/ServiceB.java", Role: "Service", Source: "FIND_SYMBOL"},
			{Workspace: CurrentWorkspace, Symbol: "ServiceX.step", Path: "src/main/java/com/example/ServiceX.java", Role: "Service", Source: "FIND_SYMBOL"},
			{Workspace: CurrentWorkspace, Symbol: "AuditService.record", Path: "src/main/java/com/example/AuditService.java", Role: "Service", Source: "FIND_SYMBOL"},
			{Workspace: CurrentWorkspace, Symbol: "LooseService.check", Path: "src/main/java/com/example/LooseService.java", Role: "Service", Source: "FIND_SYMBOL"},
			{Workspace: "dep-lib", Symbol: "DepService.call", Path: "src/main/java/com/dep/DepService.java", Role: "Service", Source: "WORKSPACE_DEPENDENCY"},
		},
		CallChains: []CallChainEvidence{
			{EntryPoint: "OrderController.submit", Chain: []string{"OrderController.submit", "ServiceA.step", "ServiceB.step"}},
			{EntryPoint: "OrderController.submit", Chain: []string{"OrderController.submit", "ServiceX.step", "ServiceB.step"}},
			{EntryPoint: "OrderController.submit", Chain: []string{"OrderController.submit", "ServiceA.step", "AuditService.record", "ServiceB.step"}},
			{EntryPoint: "OrderController.submit", Chain: []string{"OrderController.submit", "ServiceA.step"}},
			{EntryPoint: "OrderController.submit", Chain: []string{"OrderController.submit", "ServiceB.step", "ServiceA.step"}},
		},
	}
}

func task153NodeSymbols(nodes []Node) []string {
	out := make([]string, len(nodes))
	for i := range nodes {
		out[i] = nodes[i].Symbol
	}
	return out
}

func Test153EditSupportsAllSemanticOperations(t *testing.T) {
	tests := []struct {
		name      string
		ops       []EditOperation
		wantNodes []string
		wantName  string
		wantNotes string
	}{
		{
			name: "replace node",
			ops: []EditOperation{{Type: "REPLACE_NODE", From: "ServiceA.step", To: "ServiceX.step"}},
			wantNodes: []string{"ServiceX.step", "ServiceB.step"}, wantName: "订单提交", wantNotes: "old notes",
		},
		{
			name: "add node",
			ops: []EditOperation{{Type: "ADD_NODE", Symbol: "AuditService.record", After: "ServiceA.step"}},
			wantNodes: []string{"ServiceA.step", "AuditService.record", "ServiceB.step"}, wantName: "订单提交", wantNotes: "old notes",
		},
		{
			name: "remove node",
			ops: []EditOperation{{Type: "REMOVE_NODE", Symbol: "ServiceB.step"}},
			wantNodes: []string{"ServiceA.step"}, wantName: "订单提交", wantNotes: "old notes",
		},
		{
			name: "reorder nodes",
			ops: []EditOperation{{Type: "REORDER_NODE", Order: []string{"ServiceB.step", "ServiceA.step"}}},
			wantNodes: []string{"ServiceB.step", "ServiceA.step"}, wantName: "订单提交", wantNotes: "old notes",
		},
		{
			name: "rename chain",
			ops: []EditOperation{{Type: "RENAME_CHAIN", Name: "订单提交主链"}},
			wantNodes: []string{"ServiceA.step", "ServiceB.step"}, wantName: "订单提交主链", wantNotes: "old notes",
		},
		{
			name: "update notes",
			ops: []EditOperation{{Type: "UPDATE_NOTES", Notes: "业务侧确认的说明"}},
			wantNodes: []string{"ServiceA.step", "ServiceB.step"}, wantName: "订单提交", wantNotes: "业务侧确认的说明",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := task153EditExistingChain()
			candidate, err := applyEditOperations153(original, tt.ops, task153EditEvidence())
			if err != nil {
				t.Fatalf("apply edit: %v", err)
			}
			if candidate.Version != original.Version || candidate.ID != original.ID || candidate.Status != original.Status || !reflect.DeepEqual(candidate.EntryPoints, original.EntryPoints) {
				t.Fatalf("edit must preserve version/id/status/entryPoints: original=%+v candidate=%+v", original, candidate)
			}
			if got := task153NodeSymbols(candidate.Nodes); !reflect.DeepEqual(got, tt.wantNodes) {
				t.Fatalf("nodes=%v want=%v", got, tt.wantNodes)
			}
			if candidate.Name != tt.wantName || candidate.Notes != tt.wantNotes {
				t.Fatalf("metadata name=%q notes=%q", candidate.Name, candidate.Notes)
			}
			if err := verifyEditedChainFacts153(t.TempDir(), candidate, task153EditEvidence()); err != nil {
				t.Fatalf("verified resulting chain must pass: %v", err)
			}
		})
	}
}

func Test153EditRejectsUnverifiedCodeFacts(t *testing.T) {
	tests := []struct {
		name string
		ops  []EditOperation
	}{
		{name: "invented replacement", ops: []EditOperation{{Type: "REPLACE_NODE", From: "ServiceA.step", To: "InventedService.process"}}},
		{name: "existing symbol without verified edge", ops: []EditOperation{{Type: "ADD_NODE", Symbol: "LooseService.check", After: "ServiceA.step"}}},
		{name: "unverified reorder", ops: []EditOperation{{Type: "REORDER_NODE", Order: []string{"ServiceA.step", "ServiceB.step", "LooseService.check"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate, err := applyEditOperations153(task153EditExistingChain(), tt.ops, task153EditEvidence())
			if err == nil {
				err = verifyEditedChainFacts153(t.TempDir(), candidate, task153EditEvidence())
			}
			if err == nil || !strings.Contains(err.Error(), "CHAIN_EDIT_FACT_NOT_VERIFIED") {
				t.Fatalf("unverified code fact must fail closed, got %v candidate=%+v", err, candidate)
			}
		})
	}
}

func Test153EditRejectsDependencyNodePresentedAsCurrentWorkspace(t *testing.T) {
	candidate := task153EditExistingChain()
	candidate.Nodes = []Node{
		{Workspace: CurrentWorkspace, Symbol: "ServiceA.step", Path: "src/main/java/com/example/ServiceA.java", Role: "SERVICE"},
		{Workspace: CurrentWorkspace, Symbol: "DepService.call", Path: "src/main/java/com/dep/DepService.java", Role: "SERVICE"},
		{Workspace: CurrentWorkspace, Symbol: "ServiceB.step", Path: "src/main/java/com/example/ServiceB.java", Role: "SERVICE"},
	}
	if err := verifyEditedChainFacts153(t.TempDir(), candidate, task153EditEvidence()); err == nil || !strings.Contains(err.Error(), "CHAIN_EDIT_FACT_NOT_VERIFIED") {
		t.Fatalf("dependency evidence must not authorize a current-workspace node, got %v", err)
	}
}
