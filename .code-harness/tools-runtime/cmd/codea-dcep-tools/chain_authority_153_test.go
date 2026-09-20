package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/chain"
)

func setupTask153AuthorityDiscovery(t *testing.T, runID string) (string, string) {
	t.Helper()
	withTempProject(t)
	installChangeAnalysisSchema(t)
	analysisPath := filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json")
	writeFile(t, analysisPath, validChainDiscoveryAnalysis())
	prepareCommittedCertifiedAnalysisFixture153(t, runID, analysisPath)
	requestPath := writeQueryRequest(t, runID, "chain-discover.json", `{"runId":"`+runID+`","target":"OrderController.approve","changeAnalysisPath":".code-harness/runs/`+runID+`/analysis/change-analysis.json"}`)
	if err := run([]string{"chain", "discover", "--input", requestPath}); err != nil { t.Fatalf("discover: %v", err) }
	dir := filepath.Join(".code-harness", "runs", runID, "analysis", "discovered-chains")
	entries, err := os.ReadDir(dir)
	if err != nil { t.Fatal(err) }
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".yaml") {
			id := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			return filepath.ToSlash(filepath.Join(dir, entry.Name())), id
		}
	}
	t.Fatal("Runtime discovery produced no YAML candidate")
	return "", ""
}

func sealTask153Candidate(t *testing.T, runID, candidatePath string) string {
	t.Helper()
	input := writeQueryRequest(t, runID, "chain-seal-persist.json", `{"runId":"`+runID+`","candidatePath":"`+candidatePath+`"}`)
	if err := run([]string{"chain", "seal-persist", "--input", input}); err != nil { t.Fatalf("seal-persist: %v", err) }
	dir := filepath.Join(".code-harness", "runs", runID, "analysis", "chain-write-plans")
	entries, err := os.ReadDir(dir)
	if err != nil { t.Fatal(err) }
	if len(entries) != 1 || filepath.Ext(entries[0].Name()) != ".json" { t.Fatalf("expected exactly one sealed write plan, got %v", entries) }
	return strings.TrimSuffix(entries[0].Name(), ".json")
}

func persistTask153Plan(t *testing.T, runID, planID string) error {
	t.Helper()
	input := writeQueryRequest(t, runID, "chain-persist.json", `{"runId":"`+runID+`","planId":"`+planID+`"}`)
	return run([]string{"chain", "persist", "--input", input})
}

func Test153WritePlanRejectsCandidateMutationAfterSealWithZeroProjectStateWrites(t *testing.T) {
	runID := "run-153-seal-candidate"
	candidatePath, chainID := setupTask153AuthorityDiscovery(t, runID)
	planID := sealTask153Candidate(t, runID, candidatePath)
	f, err := os.OpenFile(filepath.FromSlash(candidatePath), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil { t.Fatal(err) }
	if _, err := f.WriteString("# agent changed approved bytes\n"); err != nil { _ = f.Close(); t.Fatal(err) }
	if err := f.Close(); err != nil { t.Fatal(err) }

	err = persistTask153Plan(t, runID, planID)
	if err == nil || !strings.Contains(err.Error(), "CHAIN_CANDIDATE_HASH_MISMATCH") { t.Fatalf("sealed candidate mutation must fail closed, got %v", err) }
	projectPath, _ := chain.ChainPath(".", chainID)
	if _, err := os.Stat(projectPath); !os.IsNotExist(err) { t.Fatalf("candidate mutation must produce 0 Project State writes, stat=%v", err) }
}

func Test153WritePlanRejectsCertifiedAnalysisMutationAfterSealWithZeroProjectStateWrites(t *testing.T) {
	runID := "run-153-seal-analysis"
	candidatePath, chainID := setupTask153AuthorityDiscovery(t, runID)
	planID := sealTask153Candidate(t, runID, candidatePath)
	analysisPath := filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json")
	f, err := os.OpenFile(analysisPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil { t.Fatal(err) }
	if _, err := f.WriteString(" \n"); err != nil { _ = f.Close(); t.Fatal(err) }
	if err := f.Close(); err != nil { t.Fatal(err) }

	err = persistTask153Plan(t, runID, planID)
	if err == nil || !strings.Contains(err.Error(), "CHANGED_ANALYSIS_HASH_MISMATCH") {
		t.Fatalf("sealed plan must reject authoritative analysis mutation, got %v", err)
	}
	projectPath, _ := chain.ChainPath(".", chainID)
	if _, err := os.Stat(projectPath); !os.IsNotExist(err) { t.Fatalf("analysis mutation must produce 0 Project State writes, stat=%v", err) }
}

func Test153WritePlanRejectsExistingProjectStateMutationAfterSealWithZeroWrites(t *testing.T) {
	runID := "run-153-seal-existing"
	candidatePath, chainID := setupTask153AuthorityDiscovery(t, runID)
	candidate, err := chain.Load(filepath.FromSlash(candidatePath))
	if err != nil { t.Fatal(err) }
	candidate.Status = chain.StatusAccepted
	if err := chain.SaveAccepted(".", candidate, ""); err != nil { t.Fatalf("seed existing Project State: %v", err) }
	planID := sealTask153Candidate(t, runID, candidatePath)
	projectPath, _ := chain.ChainPath(".", chainID)
	before, err := os.ReadFile(projectPath)
	if err != nil { t.Fatal(err) }
	mutated := append(append([]byte(nil), before...), []byte("# concurrent Project State mutation\n")...)
	if err := os.WriteFile(projectPath, mutated, 0o644); err != nil { t.Fatal(err) }

	err = persistTask153Plan(t, runID, planID)
	if err == nil || !strings.Contains(err.Error(), "CHAIN_EXPECTED_HASH_MISMATCH") { t.Fatalf("existing Project State mutation must fail closed, got %v", err) }
	after, err := os.ReadFile(projectPath)
	if err != nil { t.Fatal(err) }
	if string(after) != string(mutated) { t.Fatal("failed persist must not overwrite concurrently changed Project State") }
}

func Test153PersistRejectsArbitraryCandidatePathRequest(t *testing.T) {
	withTempProject(t)
	runID := "run-153-plan-only"
	input := writeQueryRequest(t, runID, "chain-persist.json", `{"runId":"`+runID+`","candidatePath":".code-harness/runs/`+runID+`/analysis/discovered-chains/fake.yaml","changeAnalysisPath":".code-harness/runs/`+runID+`/analysis/change-analysis.json"}`)
	if err := run([]string{"chain", "persist", "--input", input}); err == nil || !strings.Contains(err.Error(), "planId") {
		t.Fatalf("final persist must accept only runId + planId authority, got %v", err)
	}
}
