package main

import (
	"testing"

	"codea-harness-tools/internal/chain"
)

func Test163Task2AffectedPersistRegression(t *testing.T) {
	runID := "run-163-affected-persist"
	candidatePath, chainID := setupTask153AuthorityDiscovery(t, runID)
	planID := sealTask153Candidate(t, runID, candidatePath)
	if err := persistTask153Plan(t, runID, planID); err != nil {
		t.Fatalf("AFFECTED persistence regression: %v", err)
	}
	path, err := chain.ChainPath(".", chainID)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := chain.Load(path)
	if err != nil {
		t.Fatalf("load AFFECTED persisted chain: %v", err)
	}
	if persisted.ID != chainID || persisted.Status != chain.StatusAccepted {
		t.Fatalf("AFFECTED persisted chain=%+v", persisted)
	}
	t.Log("AFFECTED_PERSIST_REGRESSION PASS")
}
