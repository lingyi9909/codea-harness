package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/chain"
)

func setupTask153AcceptedChainForEdit(t *testing.T, runID string) (string, string) {
	t.Helper()
	candidatePath, chainID := setupTask153AuthorityDiscovery(t, runID)
	planID := sealTask153Candidate(t, runID, candidatePath)
	if err := persistTask153Plan(t, runID, planID); err != nil {
		t.Fatalf("persist discovery candidate: %v", err)
	}
	projectPath, err := chain.ChainPath(".", chainID)
	if err != nil { t.Fatal(err) }
	if _, err := os.Stat(projectPath); err != nil { t.Fatalf("accepted Project State Chain missing: %v", err) }
	return projectPath, chainID
}

func Test153EditCommandCreatesCertifiedCandidateWithoutProjectStateWrite(t *testing.T) {
	runID := "run-153-edit-command"
	projectPath, chainID := setupTask153AcceptedChainForEdit(t, runID)
	before, err := os.ReadFile(projectPath)
	if err != nil { t.Fatal(err) }

	requestPath := writeQueryRequest(t, runID, "chain-edit.json", `{
  "runId":"`+runID+`",
  "chainId":"`+chainID+`",
  "changeAnalysisPath":".code-harness/runs/`+runID+`/analysis/change-analysis.json",
  "operations":[{"type":"RENAME_CHAIN","name":"订单审批新名称"}]
}`)
	if err := run([]string{"chain", "edit", "--input", requestPath}); err != nil {
		t.Fatalf("chain edit: %v", err)
	}

	after, err := os.ReadFile(projectPath)
	if err != nil { t.Fatal(err) }
	if string(after) != string(before) {
		t.Fatal("chain edit must not directly modify Project State")
	}

	candidatePath := filepath.Join(".code-harness", "runs", runID, "analysis", "edit-candidates", chainID+".yaml")
	candidate, err := chain.Load(candidatePath)
	if err != nil { t.Fatalf("load edit candidate: %v", err) }
	if candidate.Name != "订单审批新名称" || candidate.Status != chain.StatusAccepted || candidate.ID != chainID {
		t.Fatalf("unexpected edit candidate: %+v", candidate)
	}
	certBytes, err := os.ReadFile(candidatePath + ".cert.json")
	if err != nil { t.Fatalf("edit candidate certificate missing: %v", err) }
	var cert map[string]any
	if err := json.Unmarshal(certBytes, &cert); err != nil { t.Fatal(err) }
	if cert["kind"] != "EDIT" || cert["runId"] != runID || cert["chainId"] != chainID {
		t.Fatalf("unexpected edit candidate cert: %#v", cert)
	}
}

func Test153EditCommandRejectsNonRequestPath(t *testing.T) {
	withTempProject(t)
	badPath := filepath.Join("tmp", "chain-edit.json")
	writeFile(t, badPath, `{"runId":"run-153-edit-path","chainId":"fake","changeAnalysisPath":".code-harness/runs/run-153-edit-path/analysis/change-analysis.json","operations":[{"type":"UPDATE_NOTES","notes":"x"}]}`)
	if err := run([]string{"chain", "edit", "--input", badPath}); err == nil || !strings.Contains(err.Error(), "requests") {
		t.Fatalf("chain edit must accept only same-run requests path, got %v", err)
	}
}
