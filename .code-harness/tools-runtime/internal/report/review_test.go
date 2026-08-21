package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleRequest() ReviewRequest {
	return ReviewRequest{
		RunID:          "review-20260821-001",
		HarnessVersion: "1.3.2",
		BaseRef:        "origin/develop",
		Head:           "abc123",
		Result:         ResultFailed,
		Scope: ReviewScope{ChangedFiles: []string{
			"src/main/java/OrderController.java",
			"src/main/java/OrderServiceImpl.java",
			"src/test/java/OrderServiceTest.java",
		}},
		Coverage: ReviewCoverage{
			ReviewedFiles: []string{
				"src/main/java/OrderController.java",
				"src/main/java/OrderService.java",
				"src/main/java/OrderServiceImpl.java",
				"src/main/java/OrderRepository.java",
				"src/test/java/OrderServiceTest.java",
			},
			CallChains: []CallChain{
				{EntryPoint: "OrderController.approve", Chain: []string{"OrderController.approve", "OrderService.approve", "OrderServiceImpl.approve", "OrderRepository.updateStatus"}},
				{EntryPoint: "OrderController.cancel", Chain: []string{"OrderController.cancel", "OrderService.cancel", "OrderServiceImpl.cancel"}},
			},
			ExternalDependencies: []string{"PaymentRpcClient"},
			Status:               "COMPLETE",
		},
		Findings: []Finding{
			{ID: "F-004", Category: "PRODUCTION_CODE", Severity: "LOW", File: "src/main/java/Z.java", Line: 9, Problem: "低风险问题", Evidence: "证据D", Impact: "影响D", Recommendation: "建议D", NeedsTest: false, IntroducedByChange: true, Confidence: 0.7},
			{ID: "F-003", Category: "PRODUCTION_CODE", Severity: "MEDIUM", File: "src/main/java/M.java", Line: 30, Problem: "中风险问题", Evidence: "证据C", Impact: "影响C", Recommendation: "建议C", NeedsTest: true, IntroducedByChange: true, Confidence: 0.8},
			{ID: "F-002", Category: "PRODUCTION_CODE", Severity: "CRITICAL", File: "src/main/java/B.java", Line: 20, Problem: "严重问题B", Evidence: "证据B", Impact: "影响B", Recommendation: "建议B", NeedsTest: true, IntroducedByChange: true, Confidence: 0.99},
			{ID: "F-001", Category: "PRODUCTION_CODE", Severity: "CRITICAL", File: "src/main/java/A.java", Line: 10, Problem: "严重问题A", Evidence: "证据A", Impact: "影响A", Recommendation: "建议A", NeedsTest: true, IntroducedByChange: true, Confidence: 0.95},
		},
	}
}

func TestR1PassedChineseReport(t *testing.T) {
	req := sampleRequest()
	req.Result = ResultPassed
	req.Findings = nil
	md, err := Render(req)
	if err != nil { t.Fatal(err) }
	for _, want := range []string{
		"# 📝 代码评审报告",
		"| 评审结果 | ✅ 通过 |",
		"## 📁 评审范围",
		"## 🔗 代码调用链",
		"## ✅ 评审覆盖",
		"## 📌 评审结论",
		"### ✅ 本次评审通过",
		"未发现需要处理的生产代码问题。",
	} {
		if !strings.Contains(md, want) { t.Fatalf("missing %q in markdown:\n%s", want, md) }
	}
}

func TestR2SeverityChineseColorAndDeterministicSort(t *testing.T) {
	md, err := Render(sampleRequest())
	if err != nil { t.Fatal(err) }
	for _, want := range []string{"🔴 严重", "🟠 高", "🟡 中", "🟢 低", "| 🔴 严重 | 2 |", "| 🟡 中 | 1 |", "| 🟢 低 | 1 |"} {
		if !strings.Contains(md, want) { t.Fatalf("missing %q in markdown:\n%s", want, md) }
	}
	idx1 := strings.Index(md, "F-001")
	idx2 := strings.Index(md, "F-002")
	idx3 := strings.Index(md, "F-003")
	idx4 := strings.Index(md, "F-004")
	if !(idx1 >= 0 && idx1 < idx2 && idx2 < idx3 && idx3 < idx4) {
		t.Fatalf("finding order is not severity/file/line/id deterministic: %d %d %d %d", idx1, idx2, idx3, idx4)
	}
}

func TestR3MultipleCallChainsAreNotFlattened(t *testing.T) {
	md, err := Render(sampleRequest())
	if err != nil { t.Fatal(err) }
	for _, want := range []string{"### 调用链 1", "`OrderController.approve`\n↓\n`OrderService.approve`", "### 调用链 2", "`OrderController.cancel`\n↓\n`OrderService.cancel`"} {
		if !strings.Contains(md, want) { t.Fatalf("missing %q in markdown:\n%s", want, md) }
	}
}

func TestR4NoCallChainHasChineseEmptyState(t *testing.T) {
	req := sampleRequest()
	req.Coverage.CallChains = nil
	md, err := Render(req)
	if err != nil { t.Fatal(err) }
	if !strings.Contains(md, "未发现需要展开的项目内部调用链。") { t.Fatal(md) }
}

func TestR5TestReviewScopeRulesAreLocked(t *testing.T) {
	path := filepath.Join("..", "..", "..", "agents", "reviewer.md")
	data, err := os.ReadFile(path)
	if err != nil { t.Fatal(err) }
	text := string(data)
	for _, want := range []string{
		"测试代码默认不得产生普通 Code Review Finding",
		"TEST_VALIDITY",
		"删除有效测试",
		"删除或明显弱化关键断言",
		"Mock 内部业务 Bean",
		"false-positive",
	} {
		if !strings.Contains(text, want) { t.Fatalf("reviewer rule missing %q", want) }
	}
}

func TestR6PartialChineseManualActionRequired(t *testing.T) {
	req := sampleRequest()
	req.Result = ResultManualActionRequired
	req.Coverage.Status = "PARTIAL"
	req.Findings = nil
	req.Coverage.Unresolved = []string{"OrderService.approve - 原因：未找到实现"}
	req.Coverage.MissingReviewedFiles = []string{"src/main/java/OrderServiceImpl.java"}
	md, err := Render(req)
	if err != nil { t.Fatal(err) }
	for _, want := range []string{"## ⚠️ 评审未完整完成", "当前评审需要人工处理，不能判定为通过。", "OrderService.approve", "尚未评审文件", "⚠️ 需要人工处理"} {
		if !strings.Contains(md, want) { t.Fatalf("missing %q in markdown:\n%s", want, md) }
	}
	if strings.Contains(md, "✅ 本次评审通过") { t.Fatal("PARTIAL report must not claim passed") }
}

func TestR7FixedUIHasNoEnglishLabels(t *testing.T) {
	md, err := Render(sampleRequest())
	if err != nil { t.Fatal(err) }
	for _, forbidden := range []string{"Review Scope", "Review Coverage", "Review Findings", "Problem", "Evidence", "Impact", "Recommendation", "Summary", "Needs Test"} {
		if strings.Contains(md, forbidden) { t.Fatalf("fixed UI still contains %q:\n%s", forbidden, md) }
	}
}

func TestR8SameInputIsByteForByteDeterministic(t *testing.T) {
	req := sampleRequest()
	a, err := Render(req)
	if err != nil { t.Fatal(err) }
	b, err := Render(req)
	if err != nil { t.Fatal(err) }
	if a != b { t.Fatal("same input produced different markdown") }
}

func TestFindingCategoryRequired(t *testing.T) {
	req := sampleRequest()
	req.Findings[0].Category = ""
	if err := Validate(req); err == nil { t.Fatal("expected missing finding category rejection") }
}

func TestRequestTransportMustMatchRunAndIsDeletedAfterSuccess(t *testing.T) {
	root := t.TempDir()
	old, err := os.Getwd()
	if err != nil { t.Fatal(err) }
	if err := os.Chdir(root); err != nil { t.Fatal(err) }
	defer func() { _ = os.Chdir(old) }()

	req := sampleRequest()
	data, err := json.Marshal(req)
	if err != nil { t.Fatal(err) }
	input := filepath.Join(".code-harness", "runs", req.RunID, "requests", "review-report.json")
	if err := os.MkdirAll(filepath.Dir(input), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(input, data, 0o600); err != nil { t.Fatal(err) }
	path, err := WriteRequestFile(".", input)
	if err != nil { t.Fatal(err) }
	if _, err := os.Stat(input); !os.IsNotExist(err) { t.Fatalf("transport should be deleted, stat err=%v", err) }
	if _, err := os.Stat(path); err != nil { t.Fatal(err) }
}

func TestRequestTransportRejectsUnknownFields(t *testing.T) {
	req := sampleRequest()
	data, err := json.Marshal(req)
	if err != nil { t.Fatal(err) }
	data = append(data[:len(data)-1], []byte(`,"unexpected":true}`)...)
	if _, err := DecodeReviewRequest(data); err == nil { t.Fatal("expected unknown field rejection") }
}
