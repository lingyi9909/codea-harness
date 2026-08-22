package report

import (
	"strings"
	"testing"
)

func Test14FullFirstScreenSummary(t *testing.T) {
	req := sampleRequest()
	md, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# 🔍 代码评审报告",
		"| 评审结果 | ❌ 未通过 |",
		"| 评审模式 | 📦 完整评审 |",
		"| 问题数量 | 4 |",
		"| 下一步 | 优先处理阻断问题；可使用 `harness fix finding:F-001` |",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("FULL first-screen summary missing %q:\n%s", want, md)
		}
	}
}

func Test14TargetedFirstScreenAndDisclaimer(t *testing.T) {
	req := sampleRequest()
	req.Mode = "TARGETED"
	req.Target = &ReviewTarget{Symbol: "OrderController.approve", Kind: "METHOD"}
	req.Scope.ScopedFiles = []string{
		"src/main/java/OrderController.java",
		"src/main/java/OrderService.java",
		"src/main/java/OrderServiceImpl.java",
		"src/main/java/OrderRepository.java",
	}
	for i := range req.Findings {
		req.Findings[i].File = "src/main/java/OrderServiceImpl.java"
	}
	md, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# 🔍 代码评审报告",
		"| 评审模式 | 🎯 定向评审 |",
		"| 评审目标 | `OrderController.approve` |",
		"本结论只覆盖本次定向评审范围，不代表整个 Change Set 已完成评审。",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("TARGETED UX missing %q:\n%s", want, md)
		}
	}
}

func Test14PartialFirstScreenAndExactNextStep(t *testing.T) {
	req := sampleRequest()
	req.Result = ResultManualActionRequired
	req.Coverage.Status = "PARTIAL"
	req.Findings = nil
	req.Coverage.Unresolved = []string{"OrderService.approve - 原因：未找到实现"}
	req.Coverage.MissingReviewedFiles = []string{"src/main/java/OrderServiceImpl.java"}
	md, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# 🔍 代码评审报告",
		"| 评审结果 | ⚠️ 需要人工处理 |",
		"| 下一步 | 处理未解析项/缺失评审文件后重新评审 |",
		"## ➡️ 下一步",
		"请先处理未解析项 `OrderService.approve - 原因：未找到实现`，并补充评审文件 `src/main/java/OrderServiceImpl.java`。",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("PARTIAL UX missing %q:\n%s", want, md)
		}
	}
}

func Test14FixedUIIsChinese(t *testing.T) {
	req := sampleRequest()
	md, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"Review Scope",
		"Review Coverage",
		"Summary",
		"Evidence",
		"Manual Action Required",
	} {
		if strings.Contains(md, forbidden) {
			t.Fatalf("fixed UI contains English %q:\n%s", forbidden, md)
		}
	}
}

func Test14CallChainRolePresentation(t *testing.T) {
	req := sampleRequest()
	req.Coverage.CallChains = []CallChain{{
		EntryPoint: "OrderController.approve",
		Chain: []string{
			"OrderController.approve",
			"OrderService.approve",
			"OrderServiceImpl.approve",
			"OrderMapper.updateStatus",
			"OrderMapperXml.updateStatus",
			"CustomNode.execute",
		},
	}}
	md, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"🌐 接口入口｜`OrderController.approve`",
		"⚙️ 业务接口｜`OrderService.approve`",
		"🧠 业务实现｜`OrderServiceImpl.approve`",
		"🗄 数据访问｜`OrderMapper.updateStatus`",
		"📄 Mapper XML｜`OrderMapperXml.updateStatus`",
		"🔹 代码节点｜`CustomNode.execute`",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("call-chain role presentation missing %q:\n%s", want, md)
		}
	}
}

func Test14FindingBlockStandard(t *testing.T) {
	req := sampleRequest()
	req.Findings = []Finding{{
		ID: "F-001", Category: "PRODUCTION_CODE", Severity: "HIGH",
		File: "src/main/java/OrderServiceImpl.java", Line: 128,
		Problem: "订单状态变更缺少当前状态校验。",
		Evidence: "approve() 直接更新状态。",
		Impact: "可能产生非法状态迁移。",
		Recommendation: "更新前校验当前状态。",
		NeedsTest: true, IntroducedByChange: true, Confidence: 0.95,
	}}
	md, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"### 🟠 F-001｜高",
		"📍 **位置**",
		"❗ **问题**",
		"🔎 **证据**",
		"💥 **影响**",
		"🛠 **修复建议**",
		"🧪 **是否需要测试**",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("finding block missing %q:\n%s", want, md)
		}
	}
}

func Test14NextStepByResult(t *testing.T) {
	failed := sampleRequest()
	failedMD, err := Render(failed)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(failedMD, "下一步：优先处理阻断问题；可使用 `harness fix finding:F-001`") {
		t.Fatalf("FAILED next step missing:\n%s", failedMD)
	}

	passed := sampleRequest()
	passed.Result = ResultPassed
	passed.Findings = nil
	passedMD, err := Render(passed)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(passedMD, "下一步：无需处理阻断问题。") {
		t.Fatalf("PASSED next step missing:\n%s", passedMD)
	}
	if !strings.Contains(passedMD, "| 下一步 | 无需处理阻断问题 |") {
		t.Fatalf("PASSED first-screen next action missing:\n%s", passedMD)
	}
}

func Test14RenderRemainsByteForByteDeterministic(t *testing.T) {
	req := sampleRequest()
	first, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("1.4 review renderer must be byte-for-byte deterministic")
	}
}
