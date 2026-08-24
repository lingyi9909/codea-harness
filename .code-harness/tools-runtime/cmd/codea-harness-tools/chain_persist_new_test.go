package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChainPersistNewDiscoveredCandidateAfterValidation(t *testing.T) {
	withTempProject(t)
	installChangeAnalysisSchema(t)
	writeFile(t, filepath.Join("src", "main", "resources", "mapper", "OrderMapper.xml"), "<mapper/>")

	runID := "run-new-chain"
	analysisPath := filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json")
	writeFile(t, analysisPath, task3AnalysisJSON(false))
	candidatePath := filepath.Join(".code-harness", "runs", runID, "analysis", "discovered-chains", "order-approve.yaml")
	writeFile(t, candidatePath, strings.Replace(task3AcceptedYAML, "status: ACCEPTED", "status: DISCOVERED", 1))
	requestPath := writeQueryRequest(t, runID, "chain-persist.json", `{"runId":"run-new-chain","candidatePath":".code-harness/runs/run-new-chain/analysis/discovered-chains/order-approve.yaml","changeAnalysisPath":".code-harness/runs/run-new-chain/analysis/change-analysis.json"}`)

	if err := run([]string{"chain", "persist", "--input", requestPath}); err != nil {
		t.Fatalf("new verified discovered Chain must persist after explicit confirmation flow: %v", err)
	}
	persisted := filepath.Join(".code-harness", "chains", "order-approve.yaml")
	data, err := os.ReadFile(persisted)
	if err != nil {
		t.Fatalf("accepted Project State missing: %v", err)
	}
	if !strings.Contains(string(data), "status: ACCEPTED") {
		t.Fatalf("persisted Chain must be ACCEPTED:\n%s", data)
	}
}
