package report

import (
	"strings"
	"testing"
)

func TestTargetedReviewHeaderAndDisclaimer(t *testing.T) {
	req := sampleRequest()
	req.Mode = "TARGETED"
	req.Target = &ReviewTarget{Symbol: "OrderController.approve", Kind: "METHOD"}
	req.Scope.ScopedFiles = []string{"src/main/java/OrderController.java", "src/main/java/OrderServiceImpl.java"}
	req.Coverage.CallChains = []CallChain{{EntryPoint: "OrderController.approve", Chain: []string{"OrderController.approve", "OrderService.approve", "OrderServiceImpl.approve"}}}
	md, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"| 评审模式 | 🎯 定向评审 |",
		"| 评审目标 | `OrderController.approve` |",
		"| Change Set 文件 | 3 |",
		"| 本次 Scope 文件 | 2 |",
		"本结论只覆盖本次定向评审范围，不代表整个 Change Set 已完成评审。",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("missing %q in targeted report:\n%s", want, md)
		}
	}
}

func TestTargetedClassCanRenderMultipleMethodsWithoutFlattening(t *testing.T) {
	req := sampleRequest()
	req.Mode = "TARGETED"
	req.Target = &ReviewTarget{Symbol: "OrderController", Kind: "CLASS"}
	req.Scope.ScopedFiles = []string{"src/main/java/OrderController.java", "src/main/java/OrderServiceImpl.java"}
	req.Coverage.CallChains = []CallChain{
		{EntryPoint: "OrderController.approve", Chain: []string{"OrderController.approve", "OrderService.approve"}},
		{EntryPoint: "OrderController.cancel", Chain: []string{"OrderController.cancel", "OrderService.cancel"}},
	}
	md, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"| 评审目标 | `OrderController` |",
		"### 调用链 1",
		"`OrderController.approve`",
		"### 调用链 2",
		"`OrderController.cancel`",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("missing %q in class targeted report:\n%s", want, md)
		}
	}
}

func TestFullReviewDefaultsToFullModeAndUsesChangedFilesAsScope(t *testing.T) {
	req := sampleRequest()
	md, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"| 评审模式 | 📦 完整评审 |",
		"| Change Set 文件 | 3 |",
		"| 本次 Scope 文件 | 3 |",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("missing %q in full report:\n%s", want, md)
		}
	}
	if strings.Contains(md, "本结论只覆盖本次定向评审范围") {
		t.Fatal("FULL report must not contain targeted disclaimer")
	}
}

func TestTargetedRequiresTargetAndScopedFiles(t *testing.T) {
	req := sampleRequest()
	req.Mode = "TARGETED"
	if _, err := Render(req); err == nil {
		t.Fatal("TARGETED report without target/scopedFiles must be rejected")
	}
}

func TestTargetedPartialDoesNotImplyFullCoverage(t *testing.T) {
	req := sampleRequest()
	req.Mode = "TARGETED"
	req.Target = &ReviewTarget{Symbol: "OrderController.approve", Kind: "METHOD"}
	req.Scope.ScopedFiles = []string{"src/main/java/OrderController.java", "src/main/java/OrderServiceImpl.java"}
	req.Result = ResultManualActionRequired
	req.Coverage.Status = "PARTIAL"
	req.Findings = nil
	req.Coverage.MissingReviewedFiles = []string{"src/main/java/OrderServiceImpl.java"}
	md, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "⚠️ 需要人工处理") || !strings.Contains(md, "本结论只覆盖本次定向评审范围") {
		t.Fatalf("targeted partial report missing safety language:\n%s", md)
	}
	if strings.Contains(md, "本次评审通过") {
		t.Fatal("TARGETED PARTIAL must not claim passed")
	}
}
