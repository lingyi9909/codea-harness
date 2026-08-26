package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test153ChainLoaderRejectsUncertifiedAnalysis(t *testing.T) {
	analysisPath := setupTask3CommandProject(t, false)
	_, _, err := loadVerifiedChainAnalysis(analysisPath)
	if err == nil || !strings.Contains(err.Error(), "CERTIFICATE_READ_FAILED") {
		t.Fatalf("Chain shared loader must require Certified ChangeAnalysis, got %v", err)
	}
}

func Test153ChainDiscoverRejectsUncertifiedAnalysis(t *testing.T) {
	withTempProject(t)
	installChangeAnalysisSchema(t)
	runID := "run-153-uncertified-discover"
	analysisPath := filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json")
	writeFile(t, analysisPath, validChainDiscoveryAnalysis())
	requestPath := writeQueryRequest(t, runID, "chain-discover.json", `{"runId":"run-153-uncertified-discover","target":"OrderController.approve","changeAnalysisPath":".code-harness/runs/run-153-uncertified-discover/analysis/change-analysis.json"}`)
	if err := run([]string{"chain", "discover", "--input", requestPath}); err == nil || !strings.Contains(err.Error(), "CERTIFICATE_READ_FAILED") {
		t.Fatalf("chain discover must fail closed on uncertified analysis, got %v", err)
	}
}

func Test153ChainRefreshRejectsUncertifiedAnalysis(t *testing.T) {
	setupTask3CommandProject(t, true)
	discovered := strings.Replace(task3AcceptedYAML, "status: ACCEPTED", "status: DISCOVERED", 1)
	discoveredPath := filepath.Join(".code-harness", "runs", "run-task3", "analysis", "discovered-chains", "new.yaml")
	writeFile(t, discoveredPath, discovered)
	requestPath := writeQueryRequest(t, "run-task3", "chain-refresh-uncertified.json", `{"runId":"run-task3","id":"order-approve","discoveredPath":".code-harness/runs/run-task3/analysis/discovered-chains/new.yaml"}`)
	if err := run([]string{"chain", "refresh", "--input", requestPath}); err == nil || !strings.Contains(err.Error(), "CERTIFICATE_READ_FAILED") {
		t.Fatalf("chain refresh must require same-run Certified ChangeAnalysis, got %v", err)
	}
	candidate := filepath.Join(".code-harness", "runs", "run-task3", "analysis", "refresh-candidates", "order-approve.yaml")
	if _, err := os.Stat(candidate); !os.IsNotExist(err) {
		t.Fatalf("uncertified refresh must publish no candidate, stat=%v", err)
	}
}

func Test153ChainReviewContextRejectsUncertifiedAnalysis(t *testing.T) {
	setupTask4ReviewContextProject(t)
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
	      "src/main/java/com/example/order/OrderMapper.java",
	      "src/main/resources/mapper/OrderMapper.xml"
	    ]
	  }
	}`
	requestPath := writeQueryRequest(t, "run-task4-review", "chain-review-context-uncertified.json", request)
	if err := run([]string{"chain", "review-context", "--input", requestPath}); err == nil || !strings.Contains(err.Error(), "CERTIFICATE_READ_FAILED") {
		t.Fatalf("review-context must fail closed on uncertified analysis, got %v", err)
	}
}
