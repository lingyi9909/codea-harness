package reviewrun

import (
    "context"
    "os"
    "path/filepath"
    "reflect"
    "strings"
    "testing"

    "codea-harness-tools/internal/reviewauthority"
)

func Test180SelectRejectsStaleMenu(t *testing.T) {
    root := copyControllerReviewFixture180(t)
    useRealAstGrep180(t, root)
    started, _ := Start(root)
    opts, err := Prepare(context.Background(), root, started.RunID, Intent{Mode:"CURRENT_IMPLEMENTATION", Target:"OrderController"})
    if err != nil { t.Fatal(err) }
    service := filepath.Join(root,"src","main","java","com","example","OrderServiceImpl.java")
    f, err := os.OpenFile(service, os.O_APPEND|os.O_WRONLY, 0); if err != nil { t.Fatal(err) }
    _, _ = f.WriteString("\n// changed after menu\n"); _ = f.Close()
    old := verifySelectionTurn180
    verifySelectionTurn180 = func(context.Context,string,reviewauthority.SelectionTurnRequest180) error { return nil }
    defer func(){ verifySelectionTurn180 = old }()
    _, err = Select(context.Background(), root, SelectionRequest{RunID:started.RunID, OptionsHash:opts.Hash, IDs:[]string{opts.Chains[0].ID}}, HostTurn{SessionID:"s",MessageID:"m"})
    if err == nil || !strings.Contains(err.Error(), "REVIEW_OPTIONS_STALE") { t.Fatalf("stale source/menu accepted: %v", err) }
}

func Test180SelectRechecksSourceAfterTurnBeforePersist(t *testing.T) {
    root := copyControllerReviewFixture180(t)
    useRealAstGrep180(t, root)
    started, _ := Start(root)
    opts, err := Prepare(context.Background(), root, started.RunID, Intent{Mode:"CURRENT_IMPLEMENTATION", Target:"OrderController"})
    if err != nil { t.Fatal(err) }
    service := filepath.Join(root,"src","main","java","com","example","OrderServiceImpl.java")
    old := verifySelectionTurn180
    verifySelectionTurn180 = func(context.Context,string,reviewauthority.SelectionTurnRequest180) error {
        f, err := os.OpenFile(service, os.O_APPEND|os.O_WRONLY, 0)
        if err != nil { return err }
        _, err = f.WriteString("\n// changed after verified user turn\n")
        closeErr := f.Close()
        if err != nil { return err }
        return closeErr
    }
    defer func(){ verifySelectionTurn180 = old }()
    _, err = Select(context.Background(), root, SelectionRequest{RunID:started.RunID, OptionsHash:opts.Hash, IDs:[]string{opts.Chains[0].ID}}, HostTurn{SessionID:"s",MessageID:"m"})
    if err == nil || !strings.Contains(err.Error(), "REVIEW_OPTIONS_STALE") { t.Fatalf("source changed after turn verification must be rejected: %v", err) }
    if _, statErr := os.Stat(filepath.Join(root,".code-harness","runs",started.RunID,"scope.json")); !os.IsNotExist(statErr) { t.Fatalf("stale selection wrote scope.json: %v", statErr) }
}

func Test180SelectedSubsetIsExact(t *testing.T) {
    root := copyControllerReviewFixture180(t)
    useRealAstGrep180(t, root)
    started, _ := Start(root)
    opts, err := Prepare(context.Background(), root, started.RunID, Intent{Mode:"CURRENT_IMPLEMENTATION", Target:"OrderController"})
    if err != nil { t.Fatal(err) }
    old := verifySelectionTurn180
    verifySelectionTurn180 = func(context.Context,string,reviewauthority.SelectionTurnRequest180) error { return nil }
    defer func(){ verifySelectionTurn180 = old }()
    picked := opts.Chains[0].ID
    got, err := Select(context.Background(), root, SelectionRequest{RunID:started.RunID, OptionsHash:opts.Hash, IDs:[]string{picked}}, HostTurn{SessionID:"opaque-session",MessageID:"opaque-user"})
    if err != nil { t.Fatal(err) }
    if got.Execution != "INCOMPLETE" || got.Coverage != "COMPLETE" { t.Fatalf("selection outcome: %+v", got) }
    _, state, err := loadRun(root, started.RunID); if err != nil { t.Fatal(err) }
    if !state.ScopeReady || !reflect.DeepEqual(state.SelectedIDs, []string{picked}) { t.Fatalf("selected scope not exact: %+v", state) }
    scopeBytes, err := os.ReadFile(filepath.Join(root,".code-harness","runs",started.RunID,"scope.json")); if err != nil { t.Fatal(err) }
    if strings.Contains(string(scopeBytes), opts.Chains[1].ID) { t.Fatalf("unselected chain leaked into scope:\n%s", scopeBytes) }
}

func Test180SelectUnknownAndDuplicateIDsReject(t *testing.T) {
    root := copyControllerReviewFixture180(t)
    useRealAstGrep180(t, root)
    started, _ := Start(root)
    opts, err := Prepare(context.Background(), root, started.RunID, Intent{Mode:"CURRENT_IMPLEMENTATION", Target:"OrderController"}); if err != nil { t.Fatal(err) }
    old := verifySelectionTurn180
    verifySelectionTurn180 = func(context.Context,string,reviewauthority.SelectionTurnRequest180) error { return nil }
    defer func(){ verifySelectionTurn180 = old }()
    for _, ids := range [][]string{{"C999"},{opts.Chains[0].ID,opts.Chains[0].ID}} {
        if _, err := Select(context.Background(), root, SelectionRequest{RunID:started.RunID,OptionsHash:opts.Hash,IDs:ids}, HostTurn{SessionID:"s",MessageID:"m"}); err == nil { t.Fatalf("invalid ids accepted: %v", ids) }
    }
}