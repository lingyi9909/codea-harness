package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const task170StaleProjectChainYAML = `version: 1
id: order-approve
name: 手工维护的订单审批链
status: ACCEPTED
entryPoints:
  - symbol: OrderController.approve
    path: src/main/java/com/example/order/OrderController.java
nodes:
  - symbol: LegacyOrderService.approve
    path: src/main/java/com/example/order/LegacyOrderService.java
    role: SERVICE
notes: 手工备注不能被 review 覆盖
`

func task170CurrentScopeRequest() string {
	return `{
  "mode":"TARGETED",
  "target":{"symbol":"OrderController.approve","kind":"METHOD"},
  "selectedCallChains":[{
    "entryPoint":"OrderController.approve",
    "chain":["OrderController.approve","OrderService.approve","OrderServiceImpl.approve","OrderMapper.updateStatus"]
  }],
  "scopedFiles":[
    "src/main/java/com/example/order/OrderController.java",
    "src/main/java/com/example/order/OrderService.java",
    "src/main/java/com/example/order/OrderServiceImpl.java",
    "src/main/java/com/example/order/OrderMapper.java",
    "src/main/resources/mapper/OrderMapper.xml"
  ]
}`
}

func setupTask170AutoChainCommand(t *testing.T, runID string) string {
	t.Helper()
	withTempProject(t)
	installChangeAnalysisSchema(t)
	installReviewScopeSchema(t)
	writeFile(t, filepath.Join("src", "main", "resources", "mapper", "OrderMapper.xml"), "<mapper/>")
	analysisPath := filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json")
	writeFile(t, analysisPath, validChainDiscoveryAnalysis())
	prepareCommittedCertifiedAnalysisFixture153(t, runID, analysisPath)
	chainPath := filepath.Join(".code-harness", "chains", "order-approve.yaml")
	writeFile(t, chainPath, task170StaleProjectChainYAML)
	return chainPath
}

func Test170AutoChainCommandUsesRuntimeOwnedTemporaryPolicy(t *testing.T) {
	const runID = "run-t6-command-auto"
	chainPath := setupTask170AutoChainCommand(t, runID)
	before, err := os.ReadFile(chainPath)
	if err != nil {
		t.Fatal(err)
	}
	request := `{
  "runId":"` + runID + `",
  "changeAnalysisPath":".code-harness/runs/` + runID + `/analysis/change-analysis.json",
  "reviewScope":` + task170CurrentScopeRequest() + `
}`
	requestPath := writeQueryRequest(t, runID, "chain-review-context.json", request)
	if err := run([]string{"chain", "review-context", "--input", requestPath}); err != nil {
		t.Fatalf("normal review must auto-rediscover stale chain without maintenance prompt: %v", err)
	}
	after, err := os.ReadFile(chainPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("review-context must never mutate persisted chain Project State")
	}
	entries, err := os.ReadDir(filepath.Join(".code-harness", "runs", runID, "analysis", "discovered-chains"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("auto review should publish only one current temporary chain + cert: %v", entries)
	}
}

func Test170AutoChainRequestCannotOverrideRuntimeStrategy(t *testing.T) {
	withTempProject(t)
	const runID = "run-t6-command-policy"
	request := `{
  "runId":"` + runID + `",
  "changeAnalysisPath":".code-harness/runs/` + runID + `/analysis/change-analysis.json",
  "reviewScope":{"mode":"FULL","selectedCallChains":[],"scopedFiles":[]},
  "allowTemporaryForStale":false
}`
	requestPath := writeQueryRequest(t, runID, "chain-review-context.json", request)
	err := run([]string{"chain", "review-context", "--input", requestPath})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "unknown field") {
		t.Fatalf("Agent must not be able to override AUTO_TEMPORARY policy, err=%v", err)
	}
}
