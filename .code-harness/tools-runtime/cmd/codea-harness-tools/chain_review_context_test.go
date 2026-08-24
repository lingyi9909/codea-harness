package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTask4ReviewContextProject(t *testing.T) string {
	t.Helper()
	withTempProject(t)
	installChangeAnalysisSchema(t)
	installReviewScopeSchema(t)
	writeFile(t, filepath.Join("src", "main", "resources", "mapper", "OrderMapper.xml"), "<mapper/>")
	writeFile(t, filepath.Join(".code-harness", "chains", "order-approve.yaml"), task3AcceptedYAML)
	runID := "run-task4-review"
	analysisPath := filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json")
	writeFile(t, analysisPath, task3AnalysisJSON(false))
	return analysisPath
}

func installReviewScopeSchema(t *testing.T) {
	t.Helper()
	_, testFile, _, ok := runtimeCallerForTest()
	if !ok {
		t.Fatal("locate test source")
	}
	source := filepath.Join(filepath.Dir(testFile), "..", "..", "..", "contracts", "review-scope.schema.json")
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(".code-harness", "contracts", "review-scope.schema.json"), string(data))
}

// Kept behind a small helper so this test does not duplicate runtime.Caller setup logic elsewhere.
func runtimeCallerForTest() (uintptr, string, int, bool) {
	return runtime.Caller(0)
}

func TestChainReviewContextUsesVerifiedTargetedScopeAndReusesAccepted(t *testing.T) {
	analysisPath := setupTask4ReviewContextProject(t)
	request := `{
	  "runId":"run-task4-review",
	  "changeAnalysisPath":".code-harness/runs/run-task4-review/analysis/change-analysis.json",
	  "reviewScope":{
	    "mode":"TARGETED",
	    "target":{"symbol":"OrderController.approve","kind":"METHOD"},
	    "selectedCallChains":[{"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderService.approve","OrderServiceImpl.approve","OrderMapper.updateStatus"]}],
	    "scopedFiles":[
	      "src/main/java/com/example/order/OrderController.java",
	      "src/main/java/com/example/order/OrderService.java",
	      "src/main/java/com/example/order/OrderServiceImpl.java",
	      "src/main/java/com/example/order/OrderMapper.java"
	    ]
	  }
	}`
	requestPath := writeQueryRequest(t, "run-task4-review", "chain-review-context.json", request)
	if err := run([]string{"chain", "review-context", "--input", requestPath}); err != nil {
		t.Fatalf("accepted review chain context rejected: %v", err)
	}
	if _, err := os.Stat(filepath.Join(".code-harness", "runs", "run-task4-review", "analysis", "discovered-chains")); !os.IsNotExist(err) {
		t.Fatalf("accepted reuse must not lazy discover, stat err=%v", err)
	}
	if _, err := os.Stat(analysisPath); err != nil {
		t.Fatal(err)
	}
}

func TestChainReviewContextRejectsOutsideRequestAndCrossRunAnalysis(t *testing.T) {
	withTempProject(t)
	writeFile(t, "request.json", `{}`)
	if err := run([]string{"chain", "review-context", "--input", "request.json"}); err == nil || !strings.Contains(err.Error(), "runs/<runId>/requests") {
		t.Fatalf("outside review-context request must reject, err=%v", err)
	}
	requestPath := writeQueryRequest(t, "run-a", "chain-review-context.json", `{
	  "runId":"run-a",
	  "changeAnalysisPath":".code-harness/runs/run-b/analysis/change-analysis.json",
	  "reviewScope":{"mode":"FULL","selectedCallChains":[],"scopedFiles":[]}
	}`)
	if err := run([]string{"chain", "review-context", "--input", requestPath}); err == nil || !strings.Contains(err.Error(), "same run") {
		t.Fatalf("cross-run ChangeAnalysis must reject, err=%v", err)
	}
}

func TestChainReviewContextRejectsUnverifiedScopeBeforeResolution(t *testing.T) {
	setupTask4ReviewContextProject(t)
	request := `{
	  "runId":"run-task4-review",
	  "changeAnalysisPath":".code-harness/runs/run-task4-review/analysis/change-analysis.json",
	  "reviewScope":{
	    "mode":"TARGETED",
	    "target":{"symbol":"OrderController.approve","kind":"METHOD"},
	    "selectedCallChains":[{"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderService.approve","OrderServiceImpl.approve","OrderMapper.updateStatus"]}],
	    "scopedFiles":["src/main/java/com/example/order/OrderController.java"]
	  }
	}`
	requestPath := writeQueryRequest(t, "run-task4-review", "chain-review-context-bad-scope.json", request)
	if err := run([]string{"chain", "review-context", "--input", requestPath}); err == nil {
		t.Fatal("review-context must reject scope that omits verified selected chain files")
	}
}
