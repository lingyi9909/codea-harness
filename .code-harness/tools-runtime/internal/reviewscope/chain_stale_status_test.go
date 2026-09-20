package reviewscope_test

import (
	"os"
	"path/filepath"
	"testing"

	"codea-harness-tools/internal/chain"
	"codea-harness-tools/internal/reviewscope"
)

func TestResolveChainContextsExplicitStaleStatusNeverSilentlyReused(t *testing.T) {
	root := t.TempDir()
	persisted := acceptedOrderChain()
	persisted.Status = chain.StatusStale
	path := writeAcceptedChain(t, root, persisted)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	selection := task4TargetedSelection(t, task4Analysis, "OrderService.approve")

	result, err := reviewscope.ResolveChainContexts(root, selection, []byte(task4Analysis), reviewscope.ChainResolveOptions{RunID: "run-task4-explicit-stale"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != reviewscope.ChainResolutionStaleDecision || len(result.Stale) != 1 || len(result.Contexts) != 0 {
		t.Fatalf("explicit STALE Project State must require user decision even when current evidence matches: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(root, ".code-harness", "runs", "run-task4-explicit-stale", "analysis", "discovered-chains")); !os.IsNotExist(err) {
		t.Fatalf("explicit STALE chain must not rediscover before user decision, stat err=%v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("review lookup must not rewrite explicit STALE Project State")
	}
}
