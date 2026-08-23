package chain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/schema"
)

const approvedChainYAML = `version: 1

id: order-approve
name: 订单审批
status: ACCEPTED

entryPoints:
  - symbol: OrderControllerV1.approve
    path: src/main/java/com/example/order/OrderControllerV1.java
  - symbol: OrderControllerV2.approve
    path: src/main/java/com/example/order/OrderControllerV2.java

nodes:
  - symbol: OrderService.approve
    path: src/main/java/com/example/order/OrderService.java
    role: SERVICE
  - symbol: OrderServiceImpl.approve
    path: src/main/java/com/example/order/OrderServiceImpl.java
    role: SERVICE
  - symbol: OrderMapper.updateStatus
    path: src/main/java/com/example/order/OrderMapper.java
    role: MAPPER

resources:
  - path: src/main/resources/mapper/OrderMapper.xml
    symbol: OrderMapper.updateStatus
    role: MAPPER_XML

boundaries:
  - symbol: PaymentRpcClient.notify
    path: src/main/java/com/example/client/PaymentRpcClient.java
    role: EXTERNAL

notes: |
  V1、V2 接口共用同一套订单审批核心逻辑。
`

func contractBytes(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestChainSchemaAcceptsApprovedDesignSample(t *testing.T) {
	if err := schema.ValidateYAML(contractBytes(t, "chain.schema.json"), []byte(approvedChainYAML)); err != nil {
		t.Fatalf("approved Chain sample rejected: %v", err)
	}
}

func TestChainSchemaRejectsInvalidContract(t *testing.T) {
	schemaBytes := contractBytes(t, "chain.schema.json")
	cases := map[string]string{
		"version": strings.Replace(approvedChainYAML, "version: 1", "version: 2", 1),
		"id": strings.Replace(approvedChainYAML, "id: order-approve", "id: Order_Approve", 1),
		"name": strings.Replace(approvedChainYAML, "name: 订单审批", `name: ""`, 1),
		"status": strings.Replace(approvedChainYAML, "status: ACCEPTED", "status: PUBLISHED", 1),
		"entryPoints": strings.Replace(approvedChainYAML, "entryPoints:\n  - symbol: OrderControllerV1.approve\n    path: src/main/java/com/example/order/OrderControllerV1.java\n  - symbol: OrderControllerV2.approve\n    path: src/main/java/com/example/order/OrderControllerV2.java", "entryPoints: []", 1),
		"node role": strings.Replace(approvedChainYAML, "role: SERVICE", "role: CONTROLLER", 1),
		"resource role": strings.Replace(approvedChainYAML, "role: MAPPER_XML", "role: XML", 1),
		"boundary role": strings.Replace(approvedChainYAML, "role: EXTERNAL", "role: INTERNAL", 1),
		"additional property": approvedChainYAML + "unexpected: true\n",
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if err := schema.ValidateYAML(schemaBytes, []byte(input)); err == nil {
				t.Fatalf("invalid Chain accepted:\n%s", input)
			}
		})
	}
}

func TestChainValidationResultSchemaIsStrict(t *testing.T) {
	schemaBytes := contractBytes(t, "chain-validation-result.schema.json")
	valid := []byte(`{"chainId":"order-approve","status":"VALID","errors":[],"warnings":[]}`)
	if err := schema.ValidateJSON(schemaBytes, valid); err != nil {
		t.Fatalf("valid result rejected: %v", err)
	}
	for _, invalid := range [][]byte{
		[]byte(`{"chainId":"order-approve","status":"UNKNOWN","errors":[],"warnings":[]}`),
		[]byte(`{"chainId":"order-approve","status":"VALID","errors":[],"warnings":[],"extra":true}`),
	} {
		if err := schema.ValidateJSON(schemaBytes, invalid); err == nil {
			t.Fatalf("invalid validation result accepted: %s", invalid)
		}
	}
}

func TestValidateIDAndChainPathPreventTraversal(t *testing.T) {
	if err := ValidateID("order-approve"); err != nil {
		t.Fatalf("valid id rejected: %v", err)
	}
	for _, id := range []string{"", "Order-approve", "order_approve", "-order", "../order", `order\\approve`, strings.Repeat("a", 65)} {
		if err := ValidateID(id); err == nil {
			t.Fatalf("invalid id accepted: %q", id)
		}
	}

	root := t.TempDir()
	got, err := ChainPath(root, "order-approve")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, ".code-harness", "chains", "order-approve.yaml")
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("ChainPath=%q want %q", got, want)
	}
	again, err := ChainPath(root, "order-approve")
	if err != nil || filepath.Clean(again) != filepath.Clean(got) {
		t.Fatalf("same id must resolve to one deterministic path: %q %v", again, err)
	}
	if _, err := ChainPath(root, "../escape"); err == nil {
		t.Fatal("traversal id must be rejected before path construction")
	}
}

func TestMarshalYAMLIsDeterministicAndCarriesEditingHeader(t *testing.T) {
	c := Chain{
		Version: 1,
		ID: "order-approve",
		Name: "订单审批",
		Status: StatusAccepted,
		EntryPoints: []EntryPoint{{Symbol: "OrderController.approve", Path: "src/main/java/OrderController.java"}},
		Nodes: []Node{{Symbol: "OrderService.approve", Path: "src/main/java/OrderService.java", Role: "SERVICE"}},
	}
	first, err := MarshalYAML(c)
	if err != nil {
		t.Fatal(err)
	}
	second, err := MarshalYAML(c)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("MarshalYAML must be deterministic")
	}
	for _, line := range []string{
		"# Codea Harness Business Chain",
		"# 这是项目长期业务 Chain，可直接编辑。",
		"# 修改后请执行：harness chain validate <id>",
		"# 代码结构变化后请执行：harness chain refresh <id>",
		"# symbol/path/call relation 必须真实存在，Runtime 会重新校验。",
		"# 本文件属于 Project State，Harness 升级不会覆盖。",
	} {
		if !strings.Contains(string(first), line) {
			t.Fatalf("generated YAML missing header %q:\n%s", line, first)
		}
	}
}

func TestLoadUsesStrictFieldsAndRoundTripsModel(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "order-approve.yaml")
	if err := os.WriteFile(path, []byte(approvedChainYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "order-approve" || got.Status != StatusAccepted || len(got.EntryPoints) != 2 || len(got.Nodes) != 3 {
		t.Fatalf("unexpected Chain model: %+v", got)
	}

	bad := strings.Replace(approvedChainYAML, "name: 订单审批", "name: 订单审批\nunknownField: true", 1)
	if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("Load must reject unknown YAML fields")
	}
}

func TestChainTemplateContainsEditingContractAndValidSample(t *testing.T) {
	templateBytes, err := os.ReadFile(filepath.Join("..", "..", "..", "templates", "chain.template.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		"# Codea Harness Business Chain",
		"harness chain validate <id>",
		"harness chain refresh <id>",
		"Project State",
	} {
		if !strings.Contains(string(templateBytes), line) {
			t.Fatalf("template missing %q", line)
		}
	}
	if err := schema.ValidateYAML(contractBytes(t, "chain.schema.json"), templateBytes); err != nil {
		t.Fatalf("template must itself be a valid Chain: %v", err)
	}
}
