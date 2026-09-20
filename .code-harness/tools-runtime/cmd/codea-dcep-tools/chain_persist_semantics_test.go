package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChainPersistValidatesDiscoveredBeforePromotingAccepted(t *testing.T) {
	withTempProject(t)
	installChangeAnalysisSchema(t)
	runID := "run-persist-invalid"
	analysisPath := filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json")
	writeFile(t, analysisPath, validChainDiscoveryAnalysis())
	prepareCommittedCertifiedAnalysisFixture153(t, runID, analysisPath)

	candidatePath := filepath.Join(".code-harness", "runs", runID, "analysis", "discovered-chains", "order-approve.yaml")
	writeFile(t, candidatePath, `version: 1
id: order-approve
name: 订单审批
status: DISCOVERED
entryPoints:
  - symbol: OrderController.approve
    path: src/main/java/com/example/order/OrderController.java
nodes:
  - symbol: OrderService.approve
    path: src/main/java/com/example/order/OrderService.java
    role: SERVICE
  - symbol: MissingService.approve
    path: src/main/java/com/example/order/MissingService.java
    role: SERVICE
`)
	certifyTask3Candidate(t, analysisPath, candidatePath, "DISCOVERED")
	requestPath := writeQueryRequest(t, runID, "chain-seal-invalid.json", `{"runId":"run-persist-invalid","candidatePath":".code-harness/runs/run-persist-invalid/analysis/discovered-chains/order-approve.yaml"}`)
	err := run([]string{"chain", "seal-persist", "--input", requestPath})
	if err == nil || !strings.Contains(err.Error(), "CHAIN_CANDIDATE_VALIDATION_FAILED") {
		t.Fatalf("invalid discovered candidate must be rejected before write-plan authority exists, err=%v", err)
	}
	if _, statErr := os.Stat(filepath.Join(".code-harness", "chains", "order-approve.yaml")); !os.IsNotExist(statErr) {
		t.Fatalf("invalid candidate must produce zero Project State writes, stat err=%v", statErr)
	}
}
