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

func task153EditCandidatePath(runID, chainID string) string {
	return filepath.Join(".code-harness", "runs", runID, "analysis", "edit-candidates", chainID+".yaml")
}

func runTask153RenameEdit(t *testing.T, runID, chainID, name string) string {
	t.Helper()
	requestPath := writeQueryRequest(t, runID, "chain-edit.json", `{
  "runId":"`+runID+`",
  "chainId":"`+chainID+`",
  "changeAnalysisPath":".code-harness/runs/`+runID+`/analysis/change-analysis.json",
  "operations":[{"type":"RENAME_CHAIN","name":"`+name+`"}]
}`)
	if err := run([]string{"chain", "edit", "--input", requestPath}); err != nil {
		t.Fatalf("chain edit: %v", err)
	}
	return task153EditCandidatePath(runID, chainID)
}

func sealTask153NewCandidate(t *testing.T, runID, candidatePath string) string {
	t.Helper()
	dir := filepath.Join(".code-harness", "runs", runID, "analysis", "chain-write-plans")
	before := map[string]bool{}
	if entries, err := os.ReadDir(dir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" { before[entry.Name()] = true }
		}
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	input := writeQueryRequest(t, runID, "chain-edit-seal.json", `{"runId":"`+runID+`","candidatePath":"`+filepath.ToSlash(candidatePath)+`"}`)
	if err := run([]string{"chain", "seal-persist", "--input", input}); err != nil { t.Fatalf("seal edit candidate: %v", err) }
	entries, err := os.ReadDir(dir)
	if err != nil { t.Fatal(err) }
	var added []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" || before[entry.Name()] { continue }
		added = append(added, entry.Name())
	}
	if len(added) != 1 { t.Fatalf("expected exactly one new write plan, got %v", added) }
	return strings.TrimSuffix(added[0], ".json")
}

func Test153EditCommandCreatesCertifiedCandidateWithoutProjectStateWrite(t *testing.T) {
	runID := "run-153-edit-command"
	projectPath, chainID := setupTask153AcceptedChainForEdit(t, runID)
	before, err := os.ReadFile(projectPath)
	if err != nil { t.Fatal(err) }

	candidatePath := runTask153RenameEdit(t, runID, chainID, "订单审批新名称")
	after, err := os.ReadFile(projectPath)
	if err != nil { t.Fatal(err) }
	if string(after) != string(before) {
		t.Fatal("chain edit must not directly modify Project State")
	}

	candidate, err := chain.Load(candidatePath)
	if err != nil { t.Fatalf("load edit candidate: %v", err) }
	if candidate.Name != "订单审批新名称" || candidate.Status != chain.StatusAccepted || candidate.ID != chainID {
		t.Fatalf("unexpected edit candidate: %+v", candidate)
	}
	certPath := strings.TrimSuffix(candidatePath, filepath.Ext(candidatePath)) + ".cert.json"
	certBytes, err := os.ReadFile(certPath)
	if err != nil { t.Fatalf("edit candidate certificate missing: %v", err) }
	var cert map[string]any
	if err := json.Unmarshal(certBytes, &cert); err != nil { t.Fatal(err) }
	if cert["kind"] != "EDIT" || cert["runId"] != runID || cert["chainId"] != chainID {
		t.Fatalf("unexpected edit candidate cert: %#v", cert)
	}
}

func Test153EditCandidatePersistsOnlyThroughWritePlan(t *testing.T) {
	runID := "run-153-edit-persist"
	projectPath, chainID := setupTask153AcceptedChainForEdit(t, runID)
	candidatePath := runTask153RenameEdit(t, runID, chainID, "订单审批持久化名称")
	planID := sealTask153NewCandidate(t, runID, candidatePath)
	if err := persistTask153Plan(t, runID, planID); err != nil { t.Fatalf("persist edit plan: %v", err) }
	accepted, err := chain.Load(projectPath)
	if err != nil { t.Fatal(err) }
	if accepted.Name != "订单审批持久化名称" || accepted.Status != chain.StatusAccepted {
		t.Fatalf("persisted edit not applied through write plan: %+v", accepted)
	}
}

func Test153EditCandidateMutationAfterSealFailsWithZeroProjectStateWrites(t *testing.T) {
	runID := "run-153-edit-seal-tamper"
	projectPath, chainID := setupTask153AcceptedChainForEdit(t, runID)
	before, err := os.ReadFile(projectPath)
	if err != nil { t.Fatal(err) }
	candidatePath := runTask153RenameEdit(t, runID, chainID, "订单审批待保存名称")
	planID := sealTask153NewCandidate(t, runID, candidatePath)
	f, err := os.OpenFile(candidatePath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil { t.Fatal(err) }
	if _, err := f.WriteString("# agent changed edit candidate after seal\n"); err != nil { _ = f.Close(); t.Fatal(err) }
	if err := f.Close(); err != nil { t.Fatal(err) }
	if err := persistTask153Plan(t, runID, planID); err == nil || !strings.Contains(err.Error(), "CHAIN_CANDIDATE_HASH_MISMATCH") {
		t.Fatalf("mutated EDIT candidate must fail closed, got %v", err)
	}
	after, err := os.ReadFile(projectPath)
	if err != nil { t.Fatal(err) }
	if string(after) != string(before) { t.Fatal("failed edit persist must produce 0 Project State writes") }
}

func Test153EditCommandRejectsUnverifiedCodeFactWithoutCandidateWrite(t *testing.T) {
	runID := "run-153-edit-unverified"
	projectPath, chainID := setupTask153AcceptedChainForEdit(t, runID)
	before, err := os.ReadFile(projectPath)
	if err != nil { t.Fatal(err) }
	requestPath := writeQueryRequest(t, runID, "chain-edit-unverified.json", `{
  "runId":"`+runID+`",
  "chainId":"`+chainID+`",
  "changeAnalysisPath":".code-harness/runs/`+runID+`/analysis/change-analysis.json",
  "operations":[{"type":"REPLACE_NODE","from":"OrderService.approve","to":"InventedService.process"}]
}`)
	if err := run([]string{"chain", "edit", "--input", requestPath}); err == nil || !strings.Contains(err.Error(), "CHAIN_EDIT_FACT_NOT_VERIFIED") {
		t.Fatalf("unverified edit fact must fail closed, got %v", err)
	}
	candidatePath := task153EditCandidatePath(runID, chainID)
	if _, err := os.Stat(candidatePath); !os.IsNotExist(err) { t.Fatalf("unverified edit must not write candidate, stat=%v", err) }
	after, err := os.ReadFile(projectPath)
	if err != nil { t.Fatal(err) }
	if string(after) != string(before) { t.Fatal("unverified edit must not modify Project State") }
}

func Test153EditRequestSchemaRejectsEntryPointMutation(t *testing.T) {
	runID := "run-153-edit-entrypoint-forbidden"
	projectPath, chainID := setupTask153AcceptedChainForEdit(t, runID)
	before, err := os.ReadFile(projectPath)
	if err != nil { t.Fatal(err) }
	requestPath := writeQueryRequest(t, runID, "chain-edit-entrypoint.json", `{
  "runId":"`+runID+`",
  "chainId":"`+chainID+`",
  "changeAnalysisPath":".code-harness/runs/`+runID+`/analysis/change-analysis.json",
  "operations":[{"type":"ADD_ENTRYPOINT","symbol":"OtherController.submit"}]
}`)
	if err := run([]string{"chain", "edit", "--input", requestPath}); err == nil || !strings.Contains(err.Error(), "CHAIN_EDIT_REQUEST_INVALID") {
		t.Fatalf("entryPoint edit must be rejected by Task 5 contract, got %v", err)
	}
	if _, err := os.Stat(task153EditCandidatePath(runID, chainID)); !os.IsNotExist(err) { t.Fatalf("forbidden edit must not write candidate, stat=%v", err) }
	after, err := os.ReadFile(projectPath)
	if err != nil { t.Fatal(err) }
	if string(after) != string(before) { t.Fatal("forbidden edit must not modify Project State") }
}

func Test153EditCommandRejectsNonRequestPath(t *testing.T) {
	withTempProject(t)
	badPath := filepath.Join("tmp", "chain-edit.json")
	writeFile(t, badPath, `{"runId":"run-153-edit-path","chainId":"fake","changeAnalysisPath":".code-harness/runs/run-153-edit-path/analysis/change-analysis.json","operations":[{"type":"UPDATE_NOTES","notes":"x"}]}`)
	if err := run([]string{"chain", "edit", "--input", badPath}); err == nil || !strings.Contains(err.Error(), "requests") {
		t.Fatalf("chain edit must accept only same-run requests path, got %v", err)
	}
}
