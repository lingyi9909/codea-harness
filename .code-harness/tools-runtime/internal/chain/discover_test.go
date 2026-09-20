package chain

import (
	"os"
	"path/filepath"
	"testing"
)

func task2Evidence() ChangeAnalysisEvidence {
	return ChangeAnalysisEvidence{
		ChangedFiles: []ChangedFileEvidence{
			{Path: "src/main/java/com/example/order/OrderServiceImpl.java", Role: "Service"},
		},
		AffectedControllers: []AffectedControllerEvidence{
			{Controller: "OrderControllerV1", Endpoints: []string{"OrderControllerV1.approve"}, ImpactType: "AFFECTED_BY_CALL_CHAIN", SourceSymbols: []string{"OrderServiceImpl.approve"}},
			{Controller: "OrderControllerV2", Endpoints: []string{"OrderControllerV2.approve"}, ImpactType: "AFFECTED_BY_CALL_CHAIN", SourceSymbols: []string{"OrderServiceImpl.approve"}},
			{Controller: "UserController", Endpoints: []string{"UserController.disable"}, ImpactType: "AFFECTED_BY_CALL_CHAIN", SourceSymbols: []string{"UserService.disable"}},
		},
		CallChains: []CallChainEvidence{
			{EntryPoint: "OrderControllerV1.approve", Chain: []string{"OrderControllerV1.approve", "OrderService.approve", "OrderServiceImpl.approve", "OrderMapper.updateStatus"}},
			{EntryPoint: "OrderControllerV2.approve", Chain: []string{"OrderControllerV2.approve", "OrderService.approve", "OrderServiceImpl.approve", "OrderMapper.updateStatus"}},
			{EntryPoint: "UserController.disable", Chain: []string{"UserController.disable", "UserService.disable"}},
		},
		SymbolLocations: []SymbolLocationEvidence{
			{Symbol: "OrderControllerV1.approve", Path: "src/main/java/com/example/order/OrderControllerV1.java", Role: "Controller", Source: "FIND_SYMBOL"},
			{Symbol: "OrderControllerV2.approve", Path: "src/main/java/com/example/order/OrderControllerV2.java", Role: "Controller", Source: "FIND_SYMBOL"},
			{Symbol: "OrderService.approve", Path: "src/main/java/com/example/order/OrderService.java", Role: "Service", Source: "FIND_SYMBOL"},
			{Symbol: "OrderServiceImpl.approve", Path: "src/main/java/com/example/order/OrderServiceImpl.java", Role: "Service", Source: "FIND_IMPLEMENTATIONS", From: "OrderService.approve"},
			{Symbol: "OrderMapper.updateStatus", Path: "src/main/java/com/example/order/OrderMapper.java", Role: "Mapper", Source: "FIND_SYMBOL"},
			{Symbol: "UserController.disable", Path: "src/main/java/com/example/user/UserController.java", Role: "Controller", Source: "FIND_SYMBOL"},
			{Symbol: "UserService.disable", Path: "src/main/java/com/example/user/UserService.java", Role: "Service", Source: "FIND_SYMBOL"},
		},
		ResourceRelations: []ResourceRelationEvidence{
			{Path: "src/main/resources/mapper/OrderMapper.xml", Role: "MapperXml", Resource: "OrderMapper.xml#updateStatus", FromSymbol: "OrderMapper.updateStatus", FromKind: "METHOD", Source: "MAPPER_STATEMENT", Evidence: "statement namespace/id"},
		},
		ReviewCoverage: ReviewCoverageEvidence{},
	}
}

func TestDiscoverProductionControllerMethodAndExactCorePath(t *testing.T) {
	result, err := Discover(t.TempDir(), DiscoverInput{RunID: "run-task2", Target: "OrderControllerV1.approve", ChangeAnalysis: task2Evidence()})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != DiscoveryComplete || len(result.Chains) != 1 {
		t.Fatalf("result=%+v", result)
	}
	c := result.Chains[0]
	if c.Status != StatusDiscovered || len(c.EntryPoints) != 1 || c.EntryPoints[0].Symbol != "OrderControllerV1.approve" {
		t.Fatalf("unexpected entry: %+v", c)
	}
	if len(c.Nodes) != 3 || c.Nodes[0].Symbol != "OrderService.approve" || c.Nodes[0].Role != "SERVICE" || c.Nodes[1].Symbol != "OrderServiceImpl.approve" || c.Nodes[2].Role != "MAPPER" {
		t.Fatalf("verified node order/roles lost: %+v", c.Nodes)
	}
	if len(c.Resources) != 1 || c.Resources[0].Path != "src/main/resources/mapper/OrderMapper.xml" || c.Resources[0].Role != "MAPPER_XML" {
		t.Fatalf("verified Mapper.xml relation missing: %+v", c.Resources)
	}
}

func TestDiscoverExcludesTestAndNonProductionControllerEvidence(t *testing.T) {
	analysis := task2Evidence()
	analysis.AffectedControllers = append(analysis.AffectedControllers,
		AffectedControllerEvidence{Controller: "TestOrderController", Endpoints: []string{"TestOrderController.approve"}, ImpactType: "DIRECT_CHANGE", SourceSymbols: []string{"TestOrderController.approve"}},
		AffectedControllerEvidence{Controller: "DemoController", Endpoints: []string{"DemoController.approve"}, ImpactType: "DIRECT_CHANGE", SourceSymbols: []string{"DemoController.approve"}},
	)
	analysis.CallChains = append(analysis.CallChains,
		CallChainEvidence{EntryPoint: "TestOrderController.approve", Chain: []string{"TestOrderController.approve", "OrderService.approve"}},
		CallChainEvidence{EntryPoint: "DemoController.approve", Chain: []string{"DemoController.approve", "OrderService.approve"}},
	)
	analysis.SymbolLocations = append(analysis.SymbolLocations,
		SymbolLocationEvidence{Symbol: "TestOrderController.approve", Path: "src/test/java/com/example/TestOrderController.java", Role: "Controller", Source: "FIND_SYMBOL"},
		SymbolLocationEvidence{Symbol: "DemoController.approve", Path: "src/main/java/com/example/demo/DemoController.java", Role: "Controller", Source: "FIND_SYMBOL"},
	)

	result, err := Discover(t.TempDir(), DiscoverInput{RunID: "run-task2", ChangeAnalysis: analysis})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range result.Chains {
		for _, ep := range c.EntryPoints {
			if ep.Symbol == "TestOrderController.approve" || ep.Symbol == "DemoController.approve" {
				t.Fatalf("test/demo source became persisted entry: %+v", ep)
			}
		}
	}
}

func TestDiscoverIsTargetAndChangeBounded(t *testing.T) {
	analysis := task2Evidence()
	result, err := Discover(t.TempDir(), DiscoverInput{RunID: "run-task2", Target: "OrderControllerV1.approve", ChangeAnalysis: analysis})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Chains) != 1 || result.Chains[0].EntryPoints[0].Symbol != "OrderControllerV1.approve" {
		t.Fatalf("target discovery leaked unrelated chains: %+v", result.Chains)
	}

	result, err = Discover(t.TempDir(), DiscoverInput{RunID: "run-task2", Target: "OrderServiceImpl", ChangeAnalysis: analysis})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Chains) != 1 || len(result.Chains[0].EntryPoints) != 2 {
		t.Fatalf("service target must resolve upward and exact-core canonicalize V1/V2: %+v", result.Chains)
	}

	analysis.AffectedControllers = analysis.AffectedControllers[:2]
	result, err = Discover(t.TempDir(), DiscoverInput{RunID: "run-task2", ChangeAnalysis: analysis})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range result.Chains {
		for _, ep := range c.EntryPoints {
			if ep.Symbol == "UserController.disable" {
				t.Fatal("no-target discovery must only use Change Set affected controllers")
			}
		}
	}
}

func TestDiscoverAmbiguousEntryAndInternalUnresolvedArePartial(t *testing.T) {
	analysis := task2Evidence()
	analysis.SymbolLocations = append(analysis.SymbolLocations, SymbolLocationEvidence{Symbol: "OrderControllerV1.approve", Path: "modules/legacy/src/main/java/com/example/order/OrderControllerV1.java", Role: "Controller", Source: "FIND_SYMBOL"})
	result, err := Discover(t.TempDir(), DiscoverInput{RunID: "run-task2", Target: "OrderControllerV1.approve", ChangeAnalysis: analysis})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != DiscoveryPartial || len(result.Unresolved) == 0 || len(result.Chains) != 0 {
		t.Fatalf("ambiguous entry must be PARTIAL and not persisted as a chain: %+v", result)
	}

	analysis = task2Evidence()
	analysis.ReviewCoverage.UnresolvedSymbols = []UnresolvedSymbolEvidence{{Symbol: "OrderRiskService.check", From: "OrderServiceImpl.approve", Reason: "REFERENCE_NOT_FOUND"}}
	result, err = Discover(t.TempDir(), DiscoverInput{RunID: "run-task2", Target: "OrderControllerV1.approve", ChangeAnalysis: analysis})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != DiscoveryPartial || len(result.Unresolved) != 1 || len(result.Chains) != 1 || result.Chains[0].Status != StatusDiscovered {
		t.Fatalf("internal unresolved must remain PARTIAL/DISCOVERED: %+v", result)
	}
}

func TestCanonicalizeMergesOnlyExactVerifiedCoreFacts(t *testing.T) {
	baseNodes := []Node{
		{Symbol: "OrderService.approve", Path: "src/main/java/OrderService.java", Role: "SERVICE"},
		{Symbol: "OrderServiceImpl.approve", Path: "src/main/java/OrderServiceImpl.java", Role: "SERVICE"},
		{Symbol: "OrderMapper.updateStatus", Path: "src/main/java/OrderMapper.java", Role: "MAPPER"},
	}
	v1 := Chain{Version: 1, ID: "v1", Name: "v1", Status: StatusDiscovered, EntryPoints: []EntryPoint{{Symbol: "OrderControllerV1.approve", Path: "src/main/java/OrderControllerV1.java"}}, Nodes: append([]Node(nil), baseNodes...)}
	v2 := Chain{Version: 1, ID: "v2", Name: "v2", Status: StatusDiscovered, EntryPoints: []EntryPoint{{Symbol: "OrderControllerV2.approve", Path: "src/main/java/OrderControllerV2.java"}}, Nodes: append([]Node(nil), baseNodes...)}
	merged := Canonicalize([]Chain{v2, v1})
	if len(merged) != 1 || len(merged[0].EntryPoints) != 2 {
		t.Fatalf("exact core path must merge: %+v", merged)
	}

	different := v2
	different.Nodes = []Node{
		{Symbol: "OrderV2Service.approve", Path: "src/main/java/OrderV2Service.java", Role: "SERVICE"},
		{Symbol: "RiskService.check", Path: "src/main/java/RiskService.java", Role: "SERVICE"},
		{Symbol: "OrderMapper.updateStatus", Path: "src/main/java/OrderMapper.java", Role: "MAPPER"},
	}
	if got := Canonicalize([]Chain{v1, different}); len(got) != 2 {
		t.Fatalf("different verified core path must not merge: %+v", got)
	}

	sameNamesDifferentPaths := v2
	sameNamesDifferentPaths.Nodes = append([]Node(nil), baseNodes...)
	sameNamesDifferentPaths.Nodes[1].Path = "modules/v2/src/main/java/OrderServiceImpl.java"
	if got := Canonicalize([]Chain{v1, sameNamesDifferentPaths}); len(got) != 2 {
		t.Fatalf("same class names with different exact paths must not merge: %+v", got)
	}
}

func TestDiscoverPersistsOnlyRunScopedDiscoveredArtifacts(t *testing.T) {
	root := t.TempDir()
	result, err := Discover(root, DiscoverInput{RunID: "run-task2", Target: "OrderControllerV1.approve", ChangeAnalysis: task2Evidence()})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Chains) != 1 {
		t.Fatalf("result=%+v", result)
	}
	dir := filepath.Join(root, ".code-harness", "runs", "run-task2", "analysis", "discovered-chains")
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("run-scoped discovery artifact missing: entries=%v err=%v", entries, err)
	}
	got, err := Load(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusDiscovered {
		t.Fatalf("persisted discovery must remain DISCOVERED: %+v", got)
	}
	projectState := filepath.Join(root, ".code-harness", "chains")
	if entries, err := os.ReadDir(projectState); err == nil && len(entries) != 0 {
		t.Fatalf("discovery must never write Project State: %v", entries)
	} else if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}
