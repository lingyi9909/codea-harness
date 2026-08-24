package chain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func task3Chain() Chain {
	return Chain{
		Version: 1,
		ID:      "order-approve",
		Name:    "订单审批",
		Status:  StatusAccepted,
		EntryPoints: []EntryPoint{{
			Symbol: "OrderController.approve",
			Path:   "src/main/java/com/example/order/OrderController.java",
		}},
		Nodes: []Node{
			{Symbol: "OrderService.approve", Path: "src/main/java/com/example/order/OrderService.java", Role: "SERVICE"},
			{Symbol: "OrderServiceImpl.approve", Path: "src/main/java/com/example/order/OrderServiceImpl.java", Role: "SERVICE"},
			{Symbol: "OrderMapper.updateStatus", Path: "src/main/java/com/example/order/OrderMapper.java", Role: "MAPPER"},
		},
		Resources: []Resource{{
			Path:   "src/main/resources/mapper/OrderMapper.xml",
			Symbol: "OrderMapper.updateStatus",
			Role:   "MAPPER_XML",
		}},
		Boundaries: []Boundary{{
			Symbol: "PaymentRpcClient.notify",
			Path:   "src/main/java/com/example/client/PaymentRpcClient.java",
			Role:   "EXTERNAL",
		}},
		Notes: "V1、V2 共用核心审批逻辑。",
	}
}

func task3Evidence() ChangeAnalysisEvidence {
	return ChangeAnalysisEvidence{
		AffectedControllers: []AffectedControllerEvidence{{
			Controller: "OrderController",
			Endpoints:  []string{"OrderController.approve"},
			ImpactType: "DIRECT_CHANGE",
		}},
		CallChains: []CallChainEvidence{{
			EntryPoint: "OrderController.approve",
			Chain: []string{
				"OrderController.approve",
				"OrderService.approve",
				"OrderServiceImpl.approve",
				"OrderMapper.updateStatus",
			},
		}},
		SymbolLocations: []SymbolLocationEvidence{
			{Symbol: "OrderController.approve", Path: "src/main/java/com/example/order/OrderController.java", Role: "Controller", Source: "FIND_SYMBOL"},
			{Symbol: "OrderService.approve", Path: "src/main/java/com/example/order/OrderService.java", Role: "Service", Source: "FIND_SYMBOL"},
			{Symbol: "OrderServiceImpl.approve", Path: "src/main/java/com/example/order/OrderServiceImpl.java", Role: "Service", Source: "FIND_SYMBOL"},
			{Symbol: "OrderMapper.updateStatus", Path: "src/main/java/com/example/order/OrderMapper.java", Role: "Mapper", Source: "FIND_SYMBOL"},
			{Symbol: "PaymentRpcClient.notify", Path: "src/main/java/com/example/client/PaymentRpcClient.java", Role: "External", Source: "FIND_SYMBOL", From: "OrderMapper.updateStatus"},
		},
		ResourceRelations: []ResourceRelationEvidence{{
			Path:       "src/main/resources/mapper/OrderMapper.xml",
			Role:       "MapperXml",
			Resource:   "updateStatus",
			FromSymbol: "OrderMapper.updateStatus",
			FromKind:   "METHOD",
			Source:     "MAPPER_XML",
			Evidence:   "exact mapper statement relation",
		}},
		ExternalDependencies: []string{"PaymentRpcClient.notify"},
	}
}

func writeTask3Resource(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, "src", "main", "resources", "mapper", "OrderMapper.xml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("<mapper/>"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestValidateDistinguishesValidStaleAndInvalid(t *testing.T) {
	root := t.TempDir()
	writeTask3Resource(t, root)
	chain := task3Chain()
	evidence := EvidenceSnapshot(task3Evidence())

	if got := Validate(root, chain, evidence); got.Status != ValidationValid || len(got.Errors) != 0 {
		t.Fatalf("valid chain rejected: %+v", got)
	}

	stale := chain
	stale.Nodes = append([]Node(nil), chain.Nodes...)
	stale.Nodes[1].Path = "src/main/java/com/example/order/RenamedServiceImpl.java"
	if got := Validate(root, stale, evidence); got.Status != ValidationStale || len(got.Errors) == 0 {
		t.Fatalf("accepted code-fact mismatch must be STALE: %+v", got)
	}

	invalid := chain
	invalid.ID = "Order_Approve"
	if got := Validate(root, invalid, evidence); got.Status != ValidationInvalid || len(got.Errors) == 0 {
		t.Fatalf("malformed chain must be INVALID: %+v", got)
	}
}

func TestValidateRejectsEntryNodeResourceBoundaryAndOrderRegressions(t *testing.T) {
	root := t.TempDir()
	writeTask3Resource(t, root)
	base := task3Chain()
	evidence := EvidenceSnapshot(task3Evidence())

	cases := map[string]func(Chain) Chain{
		"entry missing": func(c Chain) Chain { c.EntryPoints[0].Symbol = "MissingController.approve"; return c },
		"entry wrong path": func(c Chain) Chain { c.EntryPoints[0].Path = "src/main/java/wrong/OrderController.java"; return c },
		"entry test source": func(c Chain) Chain { c.EntryPoints[0].Path = "src/test/java/OrderController.java"; return c },
		"node missing": func(c Chain) Chain { c.Nodes[1].Symbol = "MissingService.approve"; return c },
		"node wrong path": func(c Chain) Chain { c.Nodes[1].Path = "src/main/java/wrong/OrderServiceImpl.java"; return c },
		"node order invalid": func(c Chain) Chain { c.Nodes[0], c.Nodes[1] = c.Nodes[1], c.Nodes[0]; return c },
		"resource role mismatch": func(c Chain) Chain { c.Resources[0].Role = "YAML_CONFIG"; return c },
		"resource relation absent": func(c Chain) Chain { c.Resources[0].Symbol = "OrderService.approve"; return c },
		"boundary missing": func(c Chain) Chain { c.Boundaries[0].Symbol = "MissingClient.notify"; return c },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c := task3Chain()
			c = mutate(c)
			got := Validate(root, c, evidence)
			if got.Status != ValidationStale || len(got.Errors) == 0 {
				t.Fatalf("accepted fact regression must be STALE: %+v", got)
			}
		})
	}

	if err := os.Remove(filepath.Join(root, base.Resources[0].Path)); err != nil {
		t.Fatal(err)
	}
	if got := Validate(root, base, evidence); got.Status != ValidationStale {
		t.Fatalf("missing resource must be STALE: %+v", got)
	}
}

func TestValidateNotesDoNotAffectCodeFacts(t *testing.T) {
	root := t.TempDir()
	writeTask3Resource(t, root)
	left := task3Chain()
	right := task3Chain()
	right.Notes = "完全不同的人工说明，不得覆盖机器事实。"
	evidence := EvidenceSnapshot(task3Evidence())
	leftResult := Validate(root, left, evidence)
	rightResult := Validate(root, right, evidence)
	if leftResult.Status != rightResult.Status || strings.Join(leftResult.Errors, "\n") != strings.Join(rightResult.Errors, "\n") {
		t.Fatalf("notes changed validation outcome: left=%+v right=%+v", leftResult, rightResult)
	}
}

func TestSaveAcceptedRequiresExplicitHashAndAtomicReplacement(t *testing.T) {
	root := t.TempDir()
	c := task3Chain()
	if err := SaveAccepted(root, c, ""); err != nil {
		t.Fatalf("first accepted save failed: %v", err)
	}
	path, _ := ChainPath(root, c.ID)
	firstHash, err := FileHash(path)
	if err != nil {
		t.Fatal(err)
	}

	updated := c
	updated.Notes = "用户确认后的新说明"
	if err := SaveAccepted(root, updated, ""); err == nil {
		t.Fatal("existing accepted chain must not be silently overwritten")
	}
	if err := SaveAccepted(root, updated, "deadbeef"); err == nil {
		t.Fatal("stale expected hash must be rejected")
	}
	if err := SaveAccepted(root, updated, firstHash); err != nil {
		t.Fatalf("explicit matching hash must allow atomic replacement: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Notes != updated.Notes || loaded.Status != StatusAccepted {
		t.Fatalf("replacement not persisted: %+v", loaded)
	}
}

func TestRefreshIsDiffFirstAndPreservesStableIdentity(t *testing.T) {
	root := t.TempDir()
	existing := task3Chain()
	if err := SaveAccepted(root, existing, ""); err != nil {
		t.Fatal(err)
	}
	discovered := existing
	discovered.ID = "temporary-discovered-id"
	discovered.Name = "OrderController.approve"
	discovered.Status = StatusDiscovered
	discovered.Nodes = append([]Node(nil), existing.Nodes...)
	discovered.Nodes = append(discovered.Nodes[:2], append([]Node{{Symbol: "RiskService.check", Path: "src/main/java/com/example/order/RiskService.java", Role: "SERVICE"}}, discovered.Nodes[2:]...)...)

	result := Refresh(root, existing, discovered)
	if len(result.Errors) != 0 || !result.Changed || result.ExistingHash == "" {
		t.Fatalf("refresh diff not produced: %+v", result)
	}
	if result.Candidate.ID != existing.ID || result.Candidate.Name != existing.Name || result.Candidate.Notes != existing.Notes || result.Candidate.Status != StatusAccepted {
		t.Fatalf("refresh must preserve stable user identity fields: %+v", result.Candidate)
	}
	if len(result.Added) == 0 || !strings.Contains(strings.Join(result.Added, "\n"), "RiskService.check") {
		t.Fatalf("refresh must expose deterministic added facts: %+v", result)
	}
	path, _ := ChainPath(root, existing.ID)
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Nodes) != len(existing.Nodes) {
		t.Fatal("refresh must not overwrite Project State before explicit confirmation")
	}
}

func TestListAndRenderChineseAreStable(t *testing.T) {
	root := t.TempDir()
	accepted := task3Chain()
	if err := SaveAccepted(root, accepted, ""); err != nil {
		t.Fatal(err)
	}
	stale := task3Chain()
	stale.ID = "refund-apply"
	stale.Name = "退款申请"
	stale.Status = StatusStale
	if err := SaveAccepted(root, stale, ""); err != nil {
		t.Fatal(err)
	}
	items, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != "order-approve" || items[1].ID != "refund-apply" {
		t.Fatalf("unexpected deterministic list: %+v", items)
	}
	if !strings.Contains(items[0].StatusLabel, "已确认") || !strings.Contains(items[1].StatusLabel, "已过期") {
		t.Fatalf("list status labels must be Chinese: %+v", items)
	}
	show := RenderChinese(accepted)
	for _, want := range []string{"订单审批", "状态：✅ 已确认", "入口：", "OrderController.approve", "核心链路：", "OrderServiceImpl.approve", "说明："} {
		if !strings.Contains(show, want) {
			t.Fatalf("show output missing %q:\n%s", want, show)
		}
	}
}
