package reviewrun

import (
    "context"
    "os"
    "path/filepath"
    "strings"
    "testing"

    "codea-harness-tools/internal/reviewauthority"
)

// A real user can select a chain after another process changes the Git
// baseline without changing any Java bytes. The old menu must be rejected.
func Test180SelectRejectsChangedGitBaselineAfterMenu(t *testing.T) {
    root := copyControllerReviewFixture180(t)
    useRealAstGrep180(t, root)
    initControllerReviewGitBaseline180(t, root)
    changed := "src/main/java/com/example/OrderServiceImpl.java"
    replaceT5Fixture180(t,root,changed,"orderMapper.insertOrder();","orderMapper.insertOrder(); // changed create for menu")
    replaceT5Fixture180(t,root,changed,"orderMapper.cancelOrder();","orderMapper.cancelOrder(); // changed cancel for menu")
    started, err := Start(root)
    if err != nil { t.Fatal(err) }
    options, err := Prepare(context.Background(), root, started.RunID, Intent{Mode:"CHANGES"})
    if err != nil || !options.SelectionRequired || len(options.Chains)<2 {
        t.Fatalf("expected multiple CHANGES chains: %+v %v", options, err)
    }
    // The next Git commit changes diff identity, not on-disk source bytes.
    gitReview180(t, root, "add", changed)
    gitReview180(t, root, "commit", "-m", "code changed after menu")
    old := verifySelectionTurn180
    verifySelectionTurn180 = func(context.Context,string,reviewauthority.SelectionTurnRequest180) error { return nil }
    defer func(){ verifySelectionTurn180 = old }()
    _, err = Select(context.Background(), root, SelectionRequest{RunID:started.RunID,OptionsHash:options.Hash,IDs:[]string{options.Chains[0].ID}},HostTurn{SessionID:"s",MessageID:"m"})
    if err == nil || !strings.Contains(err.Error(),"REVIEW_OPTIONS_STALE") {
        t.Fatalf("stale Git menu accepted: %v",err)
    }
    if _, err := os.Stat(filepath.Join(root,".code-harness","runs",started.RunID,"scope.json")); !os.IsNotExist(err) {
        t.Fatalf("stale Git selection generated scope: %v",err)
    }
}

func Test180FinishRejectsChangedGitBaselineWithoutIntroducedFindings(t *testing.T) {
    root := copyControllerReviewFixture180(t)
    useRealAstGrep180(t, root)
    initControllerReviewGitBaseline180(t, root)
    changed := "src/main/java/com/example/OrderController.java"
    replaceT5Fixture180(t,root,changed,"orderService.create();","orderService.create(); // review target changed")
    started, err := Start(root)
    if err != nil { t.Fatal(err) }
    options, err := Prepare(context.Background(),root,started.RunID,Intent{Mode:"CHANGES",Target:"OrderController.create"})
    if err != nil || !options.DiscoveryComplete || options.SelectionRequired || len(options.Chains)!=1 {
        t.Fatalf("expected complete target: %+v %v",options,err)
    }
    runDir:=filepath.Join(root,".code-harness","runs",started.RunID)
    scope, err := loadScope180(runDir,started.RunID)
    if err != nil { t.Fatal(err) }
    gitReview180(t,root,"add",changed)
    gitReview180(t,root,"commit","-m","baseline changed before finish")
    _, err = Finish(context.Background(),root,FinishRequest{RunID:started.RunID,Reads:scope.Reads,Findings:[]Finding{},PendingRisks:[]string{},Gaps:[]string{}})
    if err == nil || !strings.Contains(err.Error(),"REVIEW_FINISH_CHANGESET_STALE") {
        t.Fatalf("finish accepted a stale Git diff: %v",err)
    }
    status, err := Status(root,started.RunID)
    if err != nil || status.Execution != "INCOMPLETE" {
        t.Fatalf("stale finish changed durable report status: %+v %v",status,err)
    }
}
