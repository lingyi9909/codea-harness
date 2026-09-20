package report

import (
	"strings"
	"testing"
)

func TestAcceptedChainContextRendersChineseProvenance(t *testing.T) {
	req := sampleRequest()
	req.Result = ResultPassed
	req.Findings = nil
	req.ChainContext = &ChainContext{ID: "order-approve", Name: "订单审批", Source: "ACCEPTED", Status: "VALID"}
	md, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"| 业务链 | 订单审批 |",
		"| Chain ID | `order-approve` |",
		"| Chain 来源 | 项目已确认 |",
		"| Chain 状态 | 已确认 |",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("accepted chain provenance missing %q:\n%s", want, md)
		}
	}
	if strings.Contains(md, "尚未沉淀到项目 Chain") {
		t.Fatal("accepted chain must not show temporary warning")
	}
}

func TestTemporaryChainContextRendersWarning(t *testing.T) {
	req := sampleRequest()
	req.Result = ResultPassed
	req.Findings = nil
	req.ChainContext = &ChainContext{ID: "temporary-order-approve", Name: "订单审批", Source: "DISCOVERED", Status: "TEMPORARY"}
	md, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"| Chain 来源 | 本次临时发现 |",
		"| Chain 状态 | 临时 |",
		"⚠️ 本次评审使用临时发现的业务链，尚未沉淀到项目 Chain。",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("temporary chain provenance missing %q:\n%s", want, md)
		}
	}
}

func TestChainContextRejectsInvalidSourceStatusPair(t *testing.T) {
	req := sampleRequest()
	req.ChainContext = &ChainContext{ID: "order-approve", Name: "订单审批", Source: "ACCEPTED", Status: "TEMPORARY"}
	if err := Validate(req); err == nil {
		t.Fatal("accepted chain must not claim temporary status")
	}
	req.ChainContext = &ChainContext{ID: "order-approve", Name: "订单审批", Source: "DISCOVERED", Status: "VALID"}
	if err := Validate(req); err == nil {
		t.Fatal("discovered chain must not claim valid accepted status")
	}
}

func TestTargetedDisclaimerRemainsWithChainContext(t *testing.T) {
	req := sampleRequest()
	req.Mode = "TARGETED"
	req.Target = &ReviewTarget{Symbol: "OrderController.approve", Kind: "METHOD"}
	req.Scope.ScopedFiles = []string{"src/main/java/OrderController.java", "src/main/java/OrderServiceImpl.java"}
	req.Coverage.CallChains = []CallChain{{EntryPoint: "OrderController.approve", Chain: []string{"OrderController.approve", "OrderServiceImpl.approve"}}}
	req.Findings = nil
	req.Result = ResultPassed
	req.ChainContext = &ChainContext{ID: "order-approve", Name: "订单审批", Source: "ACCEPTED", Status: "VALID"}
	md, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "本结论只覆盖本次定向评审范围，不代表整个 Change Set 已完成评审。") {
		t.Fatalf("targeted disclaimer was lost:\n%s", md)
	}
}
