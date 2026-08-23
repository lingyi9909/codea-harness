package chain

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverKeepsDistinctBranchesFromSameControllerMethod(t *testing.T) {
	analysis := task2Evidence()
	analysis.AffectedControllers = analysis.AffectedControllers[:1]
	analysis.CallChains = []CallChainEvidence{
		{EntryPoint: "OrderControllerV1.approve", Chain: []string{"OrderControllerV1.approve", "OrderService.approve", "OrderServiceImpl.approve", "OrderMapper.updateStatus"}},
		{EntryPoint: "OrderControllerV1.approve", Chain: []string{"OrderControllerV1.approve", "OrderService.approve", "RiskService.check", "OrderMapper.updateStatus"}},
	}
	analysis.SymbolLocations = append(analysis.SymbolLocations, SymbolLocationEvidence{Symbol: "RiskService.check", Path: "src/main/java/com/example/order/RiskService.java", Role: "Service", Source: "FIND_SYMBOL"})

	root := t.TempDir()
	result, err := Discover(root, DiscoverInput{RunID: "run-branches", Target: "OrderControllerV1.approve", ChangeAnalysis: analysis})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != DiscoveryComplete || len(result.Chains) != 2 {
		t.Fatalf("distinct confirmed branches must survive: %+v", result)
	}
	if result.Chains[0].ID == result.Chains[1].ID {
		t.Fatalf("distinct branches need distinct deterministic artifact ids: %+v", result.Chains)
	}
	entries, err := os.ReadDir(filepath.Join(root, ".code-harness", "runs", "run-branches", "analysis", "discovered-chains"))
	if err != nil || len(entries) != 2 {
		t.Fatalf("one branch overwrote another: entries=%v err=%v", entries, err)
	}
}

func TestServiceTargetDoesNotSelectSiblingEndpointFromControllerLevelSourceSymbols(t *testing.T) {
	analysis := task2Evidence()
	analysis.AffectedControllers = []AffectedControllerEvidence{{
		Controller:    "OrderControllerV1",
		Endpoints:     []string{"OrderControllerV1.approve", "OrderControllerV1.cancel"},
		ImpactType:    "AFFECTED_BY_CALL_CHAIN",
		SourceSymbols: []string{"OrderServiceImpl.approve", "CancelService.cancel"},
	}}
	analysis.CallChains = []CallChainEvidence{
		{EntryPoint: "OrderControllerV1.approve", Chain: []string{"OrderControllerV1.approve", "OrderService.approve", "OrderServiceImpl.approve", "OrderMapper.updateStatus"}},
		{EntryPoint: "OrderControllerV1.cancel", Chain: []string{"OrderControllerV1.cancel", "CancelService.cancel"}},
	}
	analysis.SymbolLocations = append(analysis.SymbolLocations,
		SymbolLocationEvidence{Symbol: "OrderControllerV1.cancel", Path: "src/main/java/com/example/order/OrderControllerV1.java", Role: "Controller", Source: "FIND_SYMBOL"},
		SymbolLocationEvidence{Symbol: "CancelService.cancel", Path: "src/main/java/com/example/order/CancelService.java", Role: "Service", Source: "FIND_SYMBOL"},
	)

	result, err := Discover(t.TempDir(), DiscoverInput{RunID: "run-service", Target: "OrderServiceImpl", ChangeAnalysis: analysis})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Chains) != 1 || len(result.Chains[0].EntryPoints) != 1 || result.Chains[0].EntryPoints[0].Symbol != "OrderControllerV1.approve" {
		t.Fatalf("service target leaked sibling controller endpoint: %+v", result.Chains)
	}
}

func TestDiscoverRejectsControllerClassAsPersistedEntryPoint(t *testing.T) {
	analysis := task2Evidence()
	analysis.AffectedControllers = []AffectedControllerEvidence{{Controller: "OrderControllerV1", Endpoints: []string{"OrderControllerV1"}, ImpactType: "DIRECT_CHANGE", SourceSymbols: []string{"OrderControllerV1"}}}
	analysis.CallChains = []CallChainEvidence{{EntryPoint: "OrderControllerV1", Chain: []string{"OrderControllerV1", "OrderService.approve"}}}
	analysis.SymbolLocations = append(analysis.SymbolLocations, SymbolLocationEvidence{Symbol: "OrderControllerV1", Path: "src/main/java/com/example/order/OrderControllerV1.java", Role: "Controller", Source: "FIND_SYMBOL"})

	result, err := Discover(t.TempDir(), DiscoverInput{RunID: "run-class-entry", ChangeAnalysis: analysis})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != DiscoveryPartial || len(result.Chains) != 0 {
		t.Fatalf("Controller class must never be persisted as EntryPoint: %+v", result)
	}
}

func TestDiscoverIncludesVerifiedClassLevelYamlRelationForMethodNode(t *testing.T) {
	analysis := task2Evidence()
	analysis.ResourceRelations = append(analysis.ResourceRelations, ResourceRelationEvidence{
		Path:       "src/main/resources/application.yml",
		Role:       "YamlConfig",
		Resource:   "order.timeout-ms",
		FromSymbol: "OrderService",
		FromKind:   "CLASS",
		Source:     "CONFIG_REFERENCE",
		Evidence:   "OrderService consumes order.timeout-ms",
	})
	result, err := Discover(t.TempDir(), DiscoverInput{RunID: "run-yml", Target: "OrderControllerV1.approve", ChangeAnalysis: analysis})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Chains) != 1 || len(result.Chains[0].Resources) != 2 {
		t.Fatalf("verified CLASS YML relation missing from discovered chain: %+v", result.Chains)
	}
	found := false
	for _, resource := range result.Chains[0].Resources {
		if resource.Path == "src/main/resources/application.yml" && resource.Role == "YAML_CONFIG" {
			found = true
		}
	}
	if !found {
		t.Fatalf("YAML_CONFIG resource missing: %+v", result.Chains[0].Resources)
	}
}
