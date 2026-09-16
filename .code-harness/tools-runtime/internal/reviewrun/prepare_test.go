package reviewrun

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func Test180PrepareTwoEndpointsRequiresSelection(t *testing.T) {
	root := copyControllerReviewFixture180(t)
	useRealAstGrep180(t, root)
	started, err := Start(root)
	if err != nil {
		t.Fatal(err)
	}
	began := time.Now()
	got, err := Prepare(context.Background(), root, started.RunID, Intent{Mode: "CURRENT_IMPLEMENTATION", Target: "OrderController"})
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(began) > 15*time.Second {
		t.Fatalf("prepare exceeded fixture budget: %s", time.Since(began))
	}
	if !got.DiscoveryComplete || !got.SelectionRequired || len(got.Chains) != 2 {
		t.Fatalf("unexpected options: %+v", got)
	}
	if got.Chains[0].ID == got.Chains[1].ID {
		t.Fatalf("distinct endpoints were merged: %+v", got.Chains)
	}
	if got.Hash == "" {
		t.Fatal("missing optionsHash")
	}
	shared := false
	for _, a := range got.Chains[0].Nodes {
		for _, b := range got.Chains[1].Nodes {
			if a.Role == "SERVICE" && b.Role == "SERVICE" && a.Path == b.Path {
				shared = true
			}
		}
	}
	if !shared {
		t.Fatalf("fixture must prove two entries share one service: %+v", got.Chains)
	}
	runDir, _, err := loadRun(root, started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := loadPreparedOptions180(runDir)
	if err != nil {
		t.Fatal(err)
	}
	if stored.NavigationProcesses < 3 || stored.NavigationProcesses > 4 {
		t.Fatalf("expected bounded batch navigation, got %d ast-grep processes", stored.NavigationProcesses)
	}
}

func Test180PrepareSingleCompleteAutoSelects(t *testing.T) {
	root := copyControllerReviewFixture180(t)
	useRealAstGrep180(t, root)
	started, _ := Start(root)
	got, err := Prepare(context.Background(), root, started.RunID, Intent{Mode: "CURRENT_IMPLEMENTATION", Target: "OrderController.create"})
	if err != nil {
		t.Fatal(err)
	}
	if !got.DiscoveryComplete || got.SelectionRequired || len(got.Chains) != 1 {
		t.Fatalf("single complete chain did not auto-select: %+v", got)
	}
	_, state, err := loadRun(root, started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !state.ScopeReady || state.Coverage != "COMPLETE" || len(state.SelectedIDs) != 1 || state.SelectedIDs[0] != got.Chains[0].ID {
		t.Fatalf("auto-selected scope not persisted: %+v", state)
	}
}

func Test180UnknownImplementationPreventsAutoSingle(t *testing.T) {
	root := copyControllerReviewFixture180(t)
	useRealAstGrep180(t, root)
	if err := os.Remove(filepath.Join(root, "src", "main", "java", "com", "example", "OrderServiceImpl.java")); err != nil {
		t.Fatal(err)
	}
	started, _ := Start(root)
	got, err := Prepare(context.Background(), root, started.RunID, Intent{Mode: "CURRENT_IMPLEMENTATION", Target: "OrderController.create"})
	if err != nil {
		t.Fatal(err)
	}
	if got.DiscoveryComplete || got.SelectionRequired {
		t.Fatalf("unknown implementation must not manufacture AUTO_SINGLE: %+v", got)
	}
	if len(got.Gaps) == 0 {
		t.Fatalf("missing implementation gap: %+v", got)
	}
	_, state, _ := loadRun(root, started.RunID)
	if state.ScopeReady {
		t.Fatalf("incomplete discovery marked scope ready: %+v", state)
	}
}

func Test180PrepareNoEntrypointStaysIncomplete(t *testing.T) {
	root := copyControllerReviewFixture180(t)
	useRealAstGrep180(t, root)
	started, err := Start(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Prepare(context.Background(), root, started.RunID, Intent{Mode: "CURRENT_IMPLEMENTATION", Target: "MissingController"})
	if err != nil {
		t.Fatal(err)
	}
	if got.DiscoveryComplete || len(got.Chains) != 0 {
		t.Fatalf("zero discovered entrypoints must not create complete scope: %+v", got)
	}
	if !strings.Contains(strings.Join(got.Gaps, "\n"), "ENTRYPOINT_NOT_FOUND") {
		t.Fatalf("missing entrypoint gap: %+v", got)
	}
	_, state, err := loadRun(root, started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if state.ScopeReady || state.Coverage != "PARTIAL" {
		t.Fatalf("zero entrypoints created ready scope: %+v", state)
	}
}

func Test180PrepareMissingAstGrepFailsClosed(t *testing.T) {
	root := copyControllerReviewFixture180(t)
	t.Setenv("CODEA_AST_GREP_TEST_PATH", "")
	t.Setenv("PATH", t.TempDir())
	started, err := Start(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Prepare(context.Background(), root, started.RunID, Intent{Mode: "CURRENT_IMPLEMENTATION", Target: "OrderController.create"})
	if err != nil {
		t.Fatal(err)
	}
	if got.DiscoveryComplete || len(got.Gaps) == 0 || !strings.Contains(strings.Join(got.Gaps, "\n"), "AST_GREP_UNAVAILABLE") {
		t.Fatalf("missing ast-grep must remain incomplete with dependency gap: %+v", got)
	}
	_, state, err := loadRun(root, started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if state.ScopeReady || state.Coverage != "PARTIAL" {
		t.Fatalf("missing ast-grep created a ready scope: %+v", state)
	}
}

func Test180NodesRejectLegacyParallelRefs(t *testing.T) {
	root := copyControllerReviewFixture180(t)
	useRealAstGrep180(t, root)
	started, _ := Start(root)
	got, err := Prepare(context.Background(), root, started.RunID, Intent{Mode: "CURRENT_IMPLEMENTATION", Target: "OrderController"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".code-harness", "runs", started.RunID, "options.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	mutated := strings.Replace(string(data), `"nodes":`, `"chainRefs":[],"nodes":`, 1)
	if mutated == string(data) {
		t.Fatal("fixture could not inject legacy chainRefs")
	}
	if err := os.WriteFile(path, []byte(mutated), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Select(context.Background(), root, SelectionRequest{RunID: started.RunID, OptionsHash: got.Hash, IDs: []string{got.Chains[0].ID}}, HostTurn{})
	if err == nil || !strings.Contains(err.Error(), "REVIEW_OPTIONS_STALE") {
		t.Fatalf("legacy parallel refs must fail closed, err=%v", err)
	}
}

func useRealAstGrep180(t *testing.T, root string) {
	t.Helper()
	if p := os.Getenv("CODEA_AST_GREP_TEST_PATH"); p != "" {
		return
	}
	if runtime.GOOS == "windows" {
		p := filepath.Clean(filepath.Join("..", "..", "..", "bin", "ast-grep.exe"))
		if _, err := os.Stat(p); err == nil {
			t.Setenv("CODEA_AST_GREP_TEST_PATH", p)
			return
		}
	}
	t.Skip("real ast-grep unavailable outside Windows runtime regression")
}