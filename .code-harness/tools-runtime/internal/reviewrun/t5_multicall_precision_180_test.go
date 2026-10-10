package reviewrun

import (
    "context"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func replaceT5Fixture180(t *testing.T,root,rel,old,new string) {
    t.Helper()
    path:=filepath.Join(root,filepath.FromSlash(rel))
    b,err:=os.ReadFile(path)
    if err!=nil {t.Fatal(err)}
    if !strings.Contains(string(b),old) {t.Fatalf("fixture missing %q in %s",old,rel)}
    if err:=os.WriteFile(path,[]byte(strings.Replace(string(b),old,new,1)),0600);err!=nil {t.Fatal(err)}
}

func TestT5OneControllerMethodCanCallMultipleServiceMethods(t *testing.T) {
    root:=copyControllerReviewFixture180(t)
    useRealAstGrep180(t,root)
    replaceT5Fixture180(t,root,"src/main/java/com/example/OrderController.java",
        "orderService.create();","orderService.create();\n        orderService.cancel();")
    start,err:=Start(root)
    if err!=nil {t.Fatal(err)}
    opts,err:=Prepare(context.Background(),root,start.RunID,Intent{Mode:"CURRENT_IMPLEMENTATION",Target:"OrderController.create"})
    if err!=nil {t.Fatal(err)}
    if !opts.DiscoveryComplete||len(opts.Chains)!=1||opts.SelectionRequired {
        t.Fatalf("multiple genuine calls lost coverage: %+v",opts)
    }
    chain:=opts.Chains[0]
    want:=map[string]bool{"OrderServiceImpl.create":false,"OrderServiceImpl.cancel":false,"OrderMapper.insertOrder":false,"OrderMapper.cancelOrder":false}
    for _,node:=range chain.Nodes {if _,found:=want[node.Symbol];found {want[node.Symbol]=true}}
    for symbol,found:=range want {if !found {t.Fatalf("missing multi-call downstream node %s: %+v",symbol,chain)}}
}

func TestT5OnlyChangedServiceMethodAffectsMatchingController(t *testing.T) {
    root:=copyControllerReviewFixture180(t)
    useRealAstGrep180(t,root)
    initControllerReviewGitBaseline180(t,root)
    replaceT5Fixture180(t,root,"src/main/java/com/example/OrderServiceImpl.java",
        "orderMapper.insertOrder();","orderMapper.insertOrder(); // changed create method")
    start,err:=Start(root)
    if err!=nil {t.Fatal(err)}
    opts,err:=Prepare(context.Background(),root,start.RunID,Intent{Mode:"CHANGES"})
    if err!=nil {t.Fatal(err)}
    if !opts.DiscoveryComplete||len(opts.Chains)!=1||opts.Chains[0].Name!="OrderController.create" {
        t.Fatalf("sibling service method falsely affected: %+v",opts)
    }
}

func TestT5OnlyChangedSQLStatementAffectsMatchingController(t *testing.T) {
    root:=copyControllerReviewFixture180(t)
    useRealAstGrep180(t,root)
    initControllerReviewGitBaseline180(t,root)
    replaceT5Fixture180(t,root,"src/main/resources/mapper/OrderMapper.xml",
        "VALUES (1)","VALUES (7)")
    start,err:=Start(root)
    if err!=nil {t.Fatal(err)}
    opts,err:=Prepare(context.Background(),root,start.RunID,Intent{Mode:"CHANGES"})
    if err!=nil {t.Fatal(err)}
    if !opts.DiscoveryComplete||len(opts.Chains)!=1||opts.Chains[0].Name!="OrderController.create" {
        t.Fatalf("shared XML file contaminated unrelated SQL chain: %+v",opts)
    }
}

func TestT5UnmappedClassLevelChangeDoesNotClaimComplete(t *testing.T) {
    root:=copyControllerReviewFixture180(t)
    useRealAstGrep180(t,root)
    initControllerReviewGitBaseline180(t,root)
    path:=filepath.Join(root,"src/main/java/com/example/OrderController.java")
    f,err:=os.OpenFile(path,os.O_WRONLY|os.O_APPEND,0)
    if err!=nil {t.Fatal(err)}
    _,err=f.WriteString("\n// change outside any method\n")
    closeErr:=f.Close()
    if err!=nil {t.Fatal(err)}
    if closeErr!=nil {t.Fatal(closeErr)}
    start,err:=Start(root)
    if err!=nil {t.Fatal(err)}
    opts,err:=Prepare(context.Background(),root,start.RunID,Intent{Mode:"CHANGES"})
    if err!=nil {t.Fatal(err)}
    if opts.DiscoveryComplete||len(opts.Chains)!=0||!strings.Contains(strings.Join(opts.Gaps,"\n"),"CHANGE_IMPACT_UNRESOLVED") {
        t.Fatalf("unmapped class-level edit manufactured complete CHANGES: %+v",opts)
    }
    _,state,err:=loadRun(root,start.RunID)
    if err!=nil {t.Fatal(err)}
    if state.ScopeReady {t.Fatalf("unmapped edit authorized scope: %+v",state)}
}
