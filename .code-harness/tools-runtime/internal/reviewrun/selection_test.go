package reviewrun

import (
    "context"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func Test180NoTargetAutomatesAllControllersWithoutHumanSelection(t *testing.T) {
    root:=copyControllerReviewFixture180(t)
    addSecondController180(t,root)
    useRealAstGrep180(t,root)
    run,err:=Start(root);if err!=nil{t.Fatal(err)}
    opts,err:=Prepare(context.Background(),root,run.RunID,Intent{Mode:"CURRENT_IMPLEMENTATION"})
    if err!=nil{t.Fatal(err)}
    if opts.SelectionRequired || len(opts.Chains)!=2 {t.Fatalf("should auto-include both Controllers: %+v",opts)}
    scope,err:=loadScope180(filepath.Dir(run.ReportPath),run.RunID)
    if err!=nil{t.Fatal(err)}
    if len(scope.SelectedIDs)!=2||len(scope.Chains)!=4 {
        t.Fatalf("auto group lost Controller method chains: %+v",scope)
    }
}

func Test180LegacySelectAlwaysRejectsEvenWithValidOrSyntheticUserID(t *testing.T) {
    root:=copyControllerReviewFixture180(t)
    addSecondController180(t,root)
    useRealAstGrep180(t,root)
    run,err:=Start(root);if err!=nil{t.Fatal(err)}
    opts,err:=Prepare(context.Background(),root,run.RunID,Intent{Mode:"CURRENT_IMPLEMENTATION"})
    if err!=nil{t.Fatal(err)}
    for _,ids:=range [][]string{{"C1"},{"C2"},{"C1","C2"},{"C999"},{"C1","C1"}}{
        _,err=Select(context.Background(),root,SelectionRequest{RunID:run.RunID,OptionsHash:opts.Hash,IDs:ids},
            HostTurn{SessionID:"synthetic",MessageID:"synthetic"})
        if err==nil || !strings.Contains(err.Error(),"REVIEW_SELECTION_NOT_REQUIRED"){
            t.Fatalf("manual selection %v unexpectedly enabled: %v",ids,err)
        }
    }
}

func Test180AutomaticScopeRejectsChangedSourceBeforeFinish(t *testing.T) {
    root:=copyControllerReviewFixture180(t)
    addSecondController180(t,root)
    useRealAstGrep180(t,root)
    run,err:=Start(root);if err!=nil{t.Fatal(err)}
    _,err=Prepare(context.Background(),root,run.RunID,Intent{Mode:"CURRENT_IMPLEMENTATION"})
    if err!=nil{t.Fatal(err)}
    scope,err:=loadScope180(filepath.Dir(run.ReportPath),run.RunID)
    if err!=nil{t.Fatal(err)}
    path:=filepath.Join(root,"src","main","java","com","example","OrderServiceImpl.java")
    f,err:=os.OpenFile(path,os.O_APPEND|os.O_WRONLY,0)
    if err!=nil{t.Fatal(err)}
    _,err=f.WriteString("\n// changed after auto-scoping\n")
    if err!=nil{t.Fatal(err)}
    if err=f.Close();err!=nil{t.Fatal(err)}
    _,err=Finish(context.Background(),root,FinishRequest{RunID:run.RunID,Reads:scope.Reads})
    if err==nil || !strings.Contains(err.Error(),"STALE"){
        t.Fatalf("finish accepted stale authorized source after prepare: %v",err)
    }
}
