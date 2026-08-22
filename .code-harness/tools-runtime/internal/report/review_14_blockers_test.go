package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test14CallChainRoleDoesNotGuessFromClassSuffix(t *testing.T) {
	req := sampleRequest()
	req.Coverage.CallChains = []CallChain{{
		EntryPoint: "MisleadingController.execute",
		Chain: []string{
			"MisleadingController.execute",
			"ActuallyService.execute",
			"ImplController.execute",
			"PlainService.execute",
			"OrderMapper.updateStatus",
			"OrderMapper.xml#updateStatus",
			"CustomNode.execute",
		},
	}}

	decoded := decodeReviewWithRoleEvidence(t, req,
		[]map[string]any{
			{"symbol": "MisleadingController.execute", "role": "Other", "source": "FIND_SYMBOL"},
			{"symbol": "ActuallyService.execute", "role": "Controller", "source": "FIND_REFERENCES"},
			{"symbol": "ImplController.execute", "role": "Service", "source": "FIND_IMPLEMENTATIONS"},
			{"symbol": "PlainService.execute", "role": "Service", "source": "FIND_SYMBOL"},
			{"symbol": "OrderMapper.updateStatus", "role": "Mapper", "source": "FIND_SYMBOL"},
		},
		[]map[string]any{
			{"resource": "OrderMapper.xml#updateStatus", "role": "MapperXml", "source": "MAPPER_STATEMENT"},
		},
	)

	md, err := Render(decoded)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"🔹 代码节点｜`MisleadingController.execute`",
		"🌐 接口入口｜`ActuallyService.execute`",
		"🧠 业务实现｜`ImplController.execute`",
		"⚙️ 业务服务｜`PlainService.execute`",
		"🗄 数据访问｜`OrderMapper.updateStatus`",
		"📄 Mapper XML｜`OrderMapper.xml#updateStatus`",
		"🔹 代码节点｜`CustomNode.execute`",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("role-evidence rendering missing %q:\n%s", want, md)
		}
	}
	if strings.Contains(md, "🌐 接口入口｜`MisleadingController.execute`") {
		t.Fatalf("renderer guessed Controller role from class suffix:\n%s", md)
	}
}

func Test14CallChainWithoutRoleEvidenceDefaultsToCodeNode(t *testing.T) {
	req := sampleRequest()
	req.Coverage.CallChains = []CallChain{{
		EntryPoint: "OrderController.approve",
		Chain:      []string{"OrderController.approve", "OrderService.approve"},
	}}
	req.Coverage.SymbolRoleEvidence = nil
	req.Coverage.ResourceRoleEvidence = nil
	md, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"🔹 代码节点｜`OrderController.approve`",
		"🔹 代码节点｜`OrderService.approve`",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("missing safe fallback %q:\n%s", want, md)
		}
	}
}

func Test14ReviewMarkdownRuntimeContractTextIsChinese(t *testing.T) {
	req := sampleRequest()
	req.Result = ResultManualActionRequired
	req.Coverage.Status = "PARTIAL"
	req.Findings = nil
	req.Coverage.RuntimeErrors = []string{"invalid role evidence"}
	md, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(md, "Runtime Contract") || strings.Contains(md, "Runtime 校验") {
		t.Fatalf("review.md leaked English runtime contract wording:\n%s", md)
	}
	if !strings.Contains(md, "运行时契约校验错误") {
		t.Fatalf("review.md missing Chinese runtime contract wording:\n%s", md)
	}
}

func Test14OrchestratorUserSummaryDoesNotLeakMachineEnums(t *testing.T) {
	path := filepath.Join("..", "..", "..", "agents", "orchestrator.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	start := strings.Index(text, "## 统一结果")
	if start < 0 {
		t.Fatal("orchestrator missing unified user summary section")
	}
	section := text[start+len("## 统一结果"):]
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}
	for _, forbidden := range []string{"PASSED", "FAILED", "WAITING_APPROVAL", "MANUAL_ACTION_REQUIRED", "TEST_VALIDITY"} {
		if strings.Contains(section, forbidden) {
			t.Fatalf("user-visible unified summary leaked machine enum %q:\n%s", forbidden, section)
		}
	}
	for _, want := range []string{"✅ 通过", "❌ 未通过", "⏳ 等待批准", "⚠️ 需要人工处理", "测试有效性问题"} {
		if !strings.Contains(section, want) {
			t.Fatalf("user-visible unified summary missing %q:\n%s", want, section)
		}
	}
}

func withStandardRoleEvidence(req ReviewRequest) ReviewRequest {
	req.Coverage.SymbolRoleEvidence = []SymbolRoleEvidence{
		{Symbol: "OrderController.approve", Role: "Controller", Source: "FIND_SYMBOL"},
		{Symbol: "OrderService.approve", Role: "Service", Source: "FIND_SYMBOL"},
		{Symbol: "OrderServiceImpl.approve", Role: "Service", Source: "FIND_IMPLEMENTATIONS"},
		{Symbol: "OrderRepository.updateStatus", Role: "Repository", Source: "FIND_SYMBOL"},
		{Symbol: "OrderController.cancel", Role: "Controller", Source: "FIND_SYMBOL"},
		{Symbol: "OrderService.cancel", Role: "Service", Source: "FIND_SYMBOL"},
		{Symbol: "OrderServiceImpl.cancel", Role: "Service", Source: "FIND_IMPLEMENTATIONS"},
	}
	return req
}

func decodeReviewWithRoleEvidence(t *testing.T, req ReviewRequest, symbols, resources []map[string]any) ReviewRequest {
	t.Helper()
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	coverage, ok := raw["reviewCoverage"].(map[string]any)
	if !ok {
		t.Fatal("reviewCoverage transport missing")
	}
	coverage["symbolRoleEvidence"] = symbols
	coverage["resourceRoleEvidence"] = resources
	data, err = json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeReviewRequest(data)
	if err != nil {
		t.Fatalf("role evidence transport rejected: %v", err)
	}
	return decoded
}
