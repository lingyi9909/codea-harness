package chain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test152DiscoverPreservesWorkspaceIdentityInMixedChain(t *testing.T) {
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
			Chain: []string{
				"XxxController.submit",
				"XxxService.submit",
				"XxxServiceImpl.submit",
				"AbstractTemplate.execute",
				"XxxServiceImpl.doExecute",
				"XxxMapper.updateStatus",
			},
		}},
		SymbolLocations: []SymbolLocationEvidence{
			{Workspace: "current", Symbol: "XxxController.submit", Path: "src/main/java/com/company/XxxController.java", Role: "Controller", Source: "FIND_SYMBOL"},
			{Workspace: "current", Symbol: "XxxService.submit", Path: "src/main/java/com/company/XxxService.java", Role: "Service", Source: "FIND_SYMBOL"},
			{Workspace: "current", Symbol: "XxxServiceImpl.submit", Path: "src/main/java/com/company/XxxServiceImpl.java", Role: "Service", Source: "FIND_IMPLEMENTATIONS"},
			{Workspace: "company-framework", Symbol: "AbstractTemplate.execute", Path: "src/main/java/com/company/AbstractTemplate.java", Role: "Service", Source: "WORKSPACE_INHERITANCE", From: "XxxServiceImpl.submit"},
			{Workspace: "current", Symbol: "XxxServiceImpl.doExecute", Path: "src/main/java/com/company/XxxServiceImpl.java", Role: "Service", Source: "WORKSPACE_INHERITANCE", From: "AbstractTemplate.execute"},
			{Workspace: "current", Symbol: "XxxMapper.updateStatus", Path: "src/main/java/com/company/XxxMapper.java", Role: "Mapper", Source: "FIND_SYMBOL", From: "XxxServiceImpl.doExecute"},
		},
		ReviewCoverage: ReviewCoverageEvidence{UnresolvedSymbols: []UnresolvedSymbolEvidence{}},
	}

	result, err := Discover(root, DiscoverInput{RunID: "run-152-workspace", Target: "XxxController", ChangeAnalysis: analysis})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != DiscoveryComplete || len(result.Chains) != 1 {
		t.Fatalf("expected one COMPLETE mixed workspace chain, got %#v", result)
	}
	chain := result.Chains[0]
	want := []string{"current", "current", "company-framework", "current", "current"}
	if len(chain.Nodes) != len(want) {
		t.Fatalf("unexpected node count: %#v", chain.Nodes)
	}
	for i, workspace := range want {
		if chain.Nodes[i].Workspace != workspace {
			t.Fatalf("node[%d] workspace=%q want %q: %#v", i, chain.Nodes[i].Workspace, workspace, chain.Nodes[i])
		}
	}
	data, err := os.ReadFile(filepath.Join(root, ".code-harness", "runs", "run-152-workspace", "analysis", "discovered-chains", chain.ID+".yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "workspace: company-framework") || !strings.Contains(text, "workspace: current") {
		t.Fatalf("DISCOVERED YAML must preserve both workspace identities:\n%s", text)
	}
}

func Test152WorkspaceParticipatesInDeterministicCoreIdentity(t *testing.T) {
	current := Chain{Version: 1, Name: "Foo", Status: StatusDiscovered, Nodes: []Node{{Workspace: "current", Symbol: "Foo.execute", Path: "src/main/java/Foo.java", Role: "SERVICE"}}}
	dependency := Chain{Version: 1, Name: "Foo", Status: StatusDiscovered, Nodes: []Node{{Workspace: "company-framework", Symbol: "Foo.execute", Path: "src/main/java/Foo.java", Role: "SERVICE"}}}
	current.ID = discoveredID(current)
	dependency.ID = discoveredID(dependency)
	if current.ID == dependency.ID {
		t.Fatalf("workspace must participate in deterministic chain ID: %s", current.ID)
	}
	if got := Canonicalize([]Chain{current, dependency}); len(got) != 2 {
		t.Fatalf("different workspaces must not canonicalize together: %#v", got)
	}
}

func Test152LegacyChainWithoutWorkspaceLoadsAsCurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.yaml")
	legacy := `version: 1
id: ` + strings.Repeat("a", 64) + `
name: legacy
status: ACCEPTED
entryPoints:
  - symbol: XxxController.submit
    path: src/main/java/XxxController.java
nodes:
  - symbol: XxxServiceImpl.submit
    path: src/main/java/XxxServiceImpl.java
    role: SERVICE
`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Nodes) != 1 || loaded.Nodes[0].Workspace != "current" {
		t.Fatalf("legacy chain node must default to workspace=current: %#v", loaded.Nodes)
	}
}
