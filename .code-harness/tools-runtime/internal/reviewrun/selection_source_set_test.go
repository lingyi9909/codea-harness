package reviewrun

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/reviewauthority"
)

func Test180SelectRejectsAddedSourceAfterMenu(t *testing.T) {
	root := copyControllerReviewFixture180(t)
	useRealAstGrep180(t, root)
	started, err := Start(root)
	if err != nil {
		t.Fatal(err)
	}
	opts, err := Prepare(context.Background(), root, started.RunID, Intent{Mode: "CURRENT_IMPLEMENTATION", Target: "OrderController"})
	if err != nil {
		t.Fatal(err)
	}
	added := filepath.Join(root, "src", "main", "java", "com", "example", "AddedController.java")
	if err := os.WriteFile(added, []byte("package com.example;\nclass AddedController {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := verifySelectionTurn180
	verifySelectionTurn180 = func(context.Context, string, reviewauthority.SelectionTurnRequest180) error { return nil }
	defer func() { verifySelectionTurn180 = old }()
	_, err = Select(context.Background(), root, SelectionRequest{RunID: started.RunID, OptionsHash: opts.Hash, IDs: []string{opts.Chains[0].ID}}, HostTurn{SessionID: "s", MessageID: "m"})
	if err == nil || !strings.Contains(err.Error(), "REVIEW_OPTIONS_STALE") {
		t.Fatalf("added source file did not invalidate menu: %v", err)
	}
}

func Test180PrepareRefreshesCompleteCacheAfterSourceSetChanges(t *testing.T) {
	root := copyControllerReviewFixture180(t)
	useRealAstGrep180(t, root)
	started, err := Start(root)
	if err != nil {
		t.Fatal(err)
	}
	first, err := Prepare(context.Background(), root, started.RunID, Intent{Mode: "CURRENT_IMPLEMENTATION", Target: "OrderController.create"})
	if err != nil {
		t.Fatal(err)
	}
	added := filepath.Join(root, "src", "main", "java", "com", "example", "AddedType.java")
	if err := os.WriteFile(added, []byte("package com.example;\nclass AddedType {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := Prepare(context.Background(), root, started.RunID, Intent{Mode: "CURRENT_IMPLEMENTATION", Target: "OrderController.create"})
	if err != nil {
		t.Fatal(err)
	}
	if !first.DiscoveryComplete || !second.DiscoveryComplete || first.Hash == second.Hash {
		t.Fatalf("source-set change reused stale complete options: first=%+v second=%+v", first, second)
	}
}