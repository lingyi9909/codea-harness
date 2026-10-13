package reviewrun

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func Test180ReprepareInvalidatesOldSelectedScope(t *testing.T) {
	root := copyControllerReviewFixture180(t)
    addSecondController180(t,root)
	useRealAstGrep180(t, root)
	started, err := Start(root)
	if err != nil {
		t.Fatal(err)
	}
	first, err := Prepare(context.Background(), root, started.RunID, Intent{Mode: "CURRENT_IMPLEMENTATION"})
	if err != nil {
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
	second, err := Prepare(context.Background(), root, started.RunID, Intent{Mode: "CURRENT_IMPLEMENTATION"})
	if err != nil {
		t.Fatal(err)
	}
	if second.Hash == first.Hash || second.SelectionRequired {
		t.Fatalf("reprepare did not create fresh multi-chain options: first=%+v second=%+v", first, second)
	}
	scope,err:=loadScope180(filepath.Dir(started.ReportPath),started.RunID)
    if err!=nil{t.Fatal(err)}
    if scope.OptionsHash!=second.Hash || len(scope.SelectedIDs)!=2 || len(scope.Chains)!=4 {
        t.Fatalf("reprepare did not atomically re-authorize current all-Controller scope: %+v",scope)
    }
	_, state, err := loadRun(root, started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !state.ScopeReady || len(state.SelectedIDs)!=2 || state.Coverage!="COMPLETE" {
        t.Fatalf("reprepare lost new source-bound auto scope: %+v",state)
    }
}