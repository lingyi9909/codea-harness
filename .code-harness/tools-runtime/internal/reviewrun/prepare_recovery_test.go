package reviewrun

import (
	"context"
	"os"
	"strings"
	"testing"
)

func Test180PrepareRetriesIncompleteAfterAstGrepRestored(t *testing.T) {
	root := copyControllerReviewFixture180(t)
	useRealAstGrep180(t, root)
	realAst := os.Getenv("CODEA_AST_GREP_TEST_PATH")
	if realAst == "" {
		t.Skip("real ast-grep unavailable")
	}
	originalPath := os.Getenv("PATH")
	t.Setenv("CODEA_AST_GREP_TEST_PATH", "")
	t.Setenv("PATH", t.TempDir())
	started, err := Start(root)
	if err != nil {
		t.Fatal(err)
	}
	first, err := Prepare(context.Background(), root, started.RunID, Intent{Mode: "CURRENT_IMPLEMENTATION", Target: "OrderController.create"})
	if err != nil {
		t.Fatal(err)
	}
	if first.DiscoveryComplete || !strings.Contains(strings.Join(first.Gaps, "\n"), "AST_GREP_UNAVAILABLE") {
		t.Fatalf("first prepare must record dependency gap: %+v", first)
	}

	t.Setenv("CODEA_AST_GREP_TEST_PATH", realAst)
	t.Setenv("PATH", originalPath)
	second, err := Prepare(context.Background(), root, started.RunID, Intent{Mode: "CURRENT_IMPLEMENTATION", Target: "OrderController.create"})
	if err != nil {
		t.Fatal(err)
	}
	if !second.DiscoveryComplete || second.SelectionRequired || len(second.Chains) != 1 {
		t.Fatalf("same run did not recover after ast-grep was restored: %+v", second)
	}
}