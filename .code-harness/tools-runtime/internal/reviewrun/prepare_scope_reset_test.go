package reviewrun

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codea-harness-tools/internal/reviewauthority"
)

func Test180ReprepareInvalidatesOldSelectedScope(t *testing.T) {
	root := copyControllerReviewFixture180(t)
	useRealAstGrep180(t, root)
	started, err := Start(root)
	if err != nil {
		t.Fatal(err)
	}
	first, err := Prepare(context.Background(), root, started.RunID, Intent{Mode: "CURRENT_IMPLEMENTATION", Target: "OrderController"})
	if err != nil {
		t.Fatal(err)
	}
	oldVerify := verifySelectionTurn180
	verifySelectionTurn180 = func(context.Context, string, reviewauthority.SelectionTurnRequest180) error { return nil }
	defer func() { verifySelectionTurn180 = oldVerify }()
	if _, err := Select(context.Background(), root, SelectionRequest{RunID: started.RunID, OptionsHash: first.Hash, IDs: []string{first.Chains[0].ID}}, HostTurn{SessionID: "s", MessageID: "m"}); err != nil {
		t.Fatal(err)
	}
	scopePath := filepath.Join(root, ".code-harness", "runs", started.RunID, "scope.json")
	if _, err := os.Stat(scopePath); err != nil {
		t.Fatal(err)
	}
	service := filepath.Join(root, "src", "main", "java", "com", "example", "OrderServiceImpl.java")
	f, err := os.OpenFile(service, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("\n// reprepare source change\n"); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := Prepare(context.Background(), root, started.RunID, Intent{Mode: "CURRENT_IMPLEMENTATION", Target: "OrderController"})
	if err != nil {
		t.Fatal(err)
	}
	if second.Hash == first.Hash || !second.SelectionRequired {
		t.Fatalf("reprepare did not create fresh multi-chain options: first=%+v second=%+v", first, second)
	}
	if _, err := os.Stat(scopePath); !os.IsNotExist(err) {
		t.Fatalf("stale scope survived reprepare: %v", err)
	}
	_, state, err := loadRun(root, started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if state.ScopeReady || len(state.SelectedIDs) != 0 {
		t.Fatalf("reprepare retained stale selected state: %+v", state)
	}
}