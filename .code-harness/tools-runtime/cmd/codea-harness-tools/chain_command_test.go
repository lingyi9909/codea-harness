package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func installChangeAnalysisSchema(t *testing.T) {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	source := filepath.Join(filepath.Dir(testFile), "..", "..", "..", "contracts", "change-analysis.schema.json")
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(".code-harness", "contracts", "change-analysis.schema.json"), string(data))
}

func validChainDiscoveryAnalysis() string {
	return `{
  "reviewScope":{"currentBranch":"develop","baseRef":"origin/develop","baseCommit":"a","mergeBase":"a","headCommit":"b","includeWorkingTree":true},
  "changedFiles":[
    {"path":"src/main/java/com/example/order/OrderServiceImpl.java","role":"Service","sources":["COMMITTED"]},
    {"path":"src/main/resources/mapper/OrderMapper.xml","role":"MapperXml","sources":["COMMITTED"]}
  ],
  "affectedControllers":[
    {"controller":"OrderController","endpoints":["OrderController.approve"],"impactType":"AFFECTED_BY_CALL_CHAIN","sourceSymbols":["OrderServiceImpl.approve"]}
  ],
  "callChains":[
    {"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderService.approve","OrderServiceImpl.approve","OrderMapper.updateStatus"]}
  ],
  "symbolLocations":[
    {"symbol":"OrderController.approve","path":"src/main/java/com/example/order/OrderController.java","role":"Controller","source":"FIND_SYMBOL"},
    {"symbol":"OrderService.approve","path":"src/main/java/com/example/order/OrderService.java","role":"Service","source":"FIND_SYMBOL"},
    {"symbol":"OrderServiceImpl.approve","path":"src/main/java/com/example/order/OrderServiceImpl.java","role":"Service","source":"FIND_IMPLEMENTATIONS","from":"OrderService.approve"},
    {"symbol":"OrderMapper.updateStatus","path":"src/main/java/com/example/order/OrderMapper.java","role":"Mapper","source":"FIND_SYMBOL"}
  ],
  "resourceRelations":[
    {"path":"src/main/resources/mapper/OrderMapper.xml","role":"MapperXml","resource":"OrderMapper.xml#updateStatus","fromSymbol":"OrderMapper.updateStatus","fromKind":"METHOD","source":"MAPPER_STATEMENT","evidence":"statement id updateStatus matches OrderMapper.updateStatus"}
  ],
  "externalDependencies":[],
  "riskAreas":[],
  "reviewCoverage":{"status":"COMPLETE","reviewedFiles":[
    {"path":"src/main/java/com/example/order/OrderServiceImpl.java","role":"Service","reason":"CHANGED"},
    {"path":"src/main/resources/mapper/OrderMapper.xml","role":"MapperXml","reason":"CHANGED"},
    {"path":"src/main/java/com/example/order/OrderController.java","role":"Controller","reason":"CALL_CHAIN"},
    {"path":"src/main/java/com/example/order/OrderService.java","role":"Service","reason":"CALL_CHAIN"},
    {"path":"src/main/java/com/example/order/OrderMapper.java","role":"Mapper","reason":"CALL_CHAIN"}
  ],"unresolvedSymbols":[]}
}`
}

func TestChainDiscoverUsesControlledRunRequestAndWritesOnlyRunState(t *testing.T) {
	withTempProject(t)
	installChangeAnalysisSchema(t)
	runID := "run-task2"
	analysisPath := filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json")
	writeFile(t, analysisPath, validChainDiscoveryAnalysis())
	requestPath := writeQueryRequest(t, runID, "chain-discover.json", `{"runId":"run-task2","target":"OrderController.approve","changeAnalysisPath":".code-harness/runs/run-task2/analysis/change-analysis.json"}`)

	if err := run([]string{"chain", "discover", "--input", requestPath}); err != nil {
		t.Fatal(err)
	}
	discovered := filepath.Join(".code-harness", "runs", runID, "analysis", "discovered-chains")
	entries, err := os.ReadDir(discovered)
	if err != nil || len(entries) != 1 {
		t.Fatalf("discovered run artifact missing: entries=%v err=%v", entries, err)
	}
	if _, err := os.Stat(filepath.Join(".code-harness", "chains")); !os.IsNotExist(err) {
		t.Fatalf("Task 2 discovery must not create Project State chains/**, err=%v", err)
	}
}

func TestChainDiscoverRejectsOutsideInputAndRunIDMismatch(t *testing.T) {
	withTempProject(t)
	writeFile(t, "request.json", `{}`)
	if err := run([]string{"chain", "discover", "--input", "request.json"}); err == nil || !strings.Contains(err.Error(), "runs/<runId>/requests") {
		t.Fatalf("outside request path must reject before discovery, err=%v", err)
	}

	path := writeQueryRequest(t, "run-a", "chain-discover.json", `{"runId":"run-b","changeAnalysisPath":".code-harness/runs/run-b/analysis/change-analysis.json"}`)
	if err := run([]string{"chain", "discover", "--input", path}); err == nil || !strings.Contains(err.Error(), "RUN_ID_PATH_MISMATCH") {
		t.Fatalf("body/path runId mismatch must reject, err=%v", err)
	}
}

func TestChainDiscoverRejectsRawTargetOrOutputCLIArguments(t *testing.T) {
	if err := run([]string{"chain", "discover", "OrderController.approve"}); err == nil {
		t.Fatal("chain discover must not accept positional raw target")
	}
	if err := run([]string{"chain", "discover", "--output", "elsewhere"}); err == nil {
		t.Fatal("chain discover must not expose arbitrary output path")
	}
}
