package reviewrun

import (
    "context"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func addSecondController180(t *testing.T,root string) {
    t.Helper()
    p:=filepath.Join(root,"src","main","java","com","example","UserController.java")
    b:=`package com.example;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RestController;
@RestController
public class UserController {
    private final OrderService orderService;
    public UserController(OrderService orderService) {this.orderService=orderService;}
    @PostMapping("/users/a")
    public void syncUser(){orderService.create();}
    @PostMapping("/users/b")
    public void closeUser(){orderService.cancel();}
}
`
    if err:=os.WriteFile(p,[]byte(b),0600);err!=nil{t.Fatal(err)}
}

func TestT5FiveMethodControllerOneMenuExactChildren(t *testing.T) {
    methods:=[]Chain{}
    for _,name:=range []string{"one","two","three","four","five"} {
        methods=append(methods,Chain{
            Name:"OrderController."+name,Nodes:[]Node{
                {Path:"module-a/src/main/java/com/example/OrderController.java",Symbol:"OrderController."+name,Role:"CONTROLLER",Workspace:"current"},
                {Path:"module-a/src/main/java/com/example/OrderService.java",Symbol:"OrderService."+name,Role:"SERVICE",Workspace:"current"},
            },Unresolved:[]string{},
        })
    }
    choices:=groupControllerChains180(methods,"CHANGES")
    if len(choices)!=1||choices[0].ID!="C1"||len(choices[0].Methods)!=5||
       choices[0].Module!="module-a"||len(choices[0].Nodes)!=10 {
        t.Fatalf("five methods did not collapse exactly to one option: %+v",choices)
    }
    if len(expandSelectedMethodChains180(choices))!=5 {
        t.Fatal("group expansion lost methods")
    }
    if choices[0].ControllerPath!="module-a/src/main/java/com/example/OrderController.java"{
        t.Fatalf("Controller identity lost path: %+v",choices[0])
    }
    hash:=optionsHash180("/app",Intent{Mode:"CHANGES"},"sourceSha:gitSnapshot",Options{Chains:choices,DiscoveryComplete:true})
    choices[0].Methods[0].Nodes[1].Symbol="OrderService.changed"
    if hash==optionsHash180("/app",Intent{Mode:"CHANGES"},"sourceSha:gitSnapshot",Options{Chains:choices,DiscoveryComplete:true}){
        t.Fatal("optionsHash did not bind individual method-chain identity")
    }
}

func TestT5SingleControllerTwoMethodsAutoReadsBoth(t *testing.T){
    root:=copyControllerReviewFixture180(t)
    useRealAstGrep180(t,root)
    run,err:=Start(root);if err!=nil{t.Fatal(err)}
    opts,err:=Prepare(context.Background(),root,run.RunID,Intent{Mode:"CURRENT_IMPLEMENTATION",Target:"OrderController"})
    if err!=nil{t.Fatal(err)}
    if len(opts.Chains)!=1||opts.SelectionRequired||len(opts.Chains[0].Methods)!=2||!opts.DiscoveryComplete {
        t.Fatalf("single Controller must auto-select both methods: %+v",opts)
    }
    scope,err:=loadScope180(filepath.Dir(run.ReportPath),run.RunID)
    if err!=nil{t.Fatal(err)}
    if len(scope.Chains)!=2||len(scope.FindingReads)<4||scope.Coverage!="COMPLETE"{
        t.Fatalf("automatic grouped method scope incomplete: %+v",scope)
    }
}

func TestT5TwoControllersAutoReviewAllOrExactTarget(t *testing.T){
    for _,test:=range []struct{target string;controllerCount int;methods int}{
        {"",2,4},{"OrderController",1,2},{"UserController",1,2},
    } {
        t.Run("target="+test.target,func(t *testing.T){
            root:=copyControllerReviewFixture180(t)
            addSecondController180(t,root)
            useRealAstGrep180(t,root)
            run,err:=Start(root);if err!=nil{t.Fatal(err)}
            opts,err:=Prepare(context.Background(),root,run.RunID,
                Intent{Mode:"CURRENT_IMPLEMENTATION",Target:test.target})
            if err!=nil{t.Fatal(err)}
            if !opts.DiscoveryComplete||opts.SelectionRequired||
                len(opts.Chains)!=test.controllerCount {
                t.Fatalf("no manual menu; exact Controller target/all expected: %+v",opts)
            }
            _,state,err:=loadRun(root,run.RunID);if err!=nil{t.Fatal(err)}
            scope,err:=loadScope180(filepath.Dir(run.ReportPath),run.RunID)
            if err!=nil{t.Fatal(err)}
            if !state.ScopeReady||state.Coverage!="COMPLETE"||
                len(scope.Chains)!=test.methods||len(scope.SelectedIDs)!=test.controllerCount {
                t.Fatalf("scope is not automatically complete: state=%+v scope=%+v",state,scope)
            }
            for _,method:=range scope.Chains {
                if test.target!="" && !strings.HasPrefix(method.Name,test.target+"."){
                    t.Fatalf("unrelated Controller method leaked into targeted scope: %+v",method)
                }
            }
            // Manual select is no longer authorized even if a caller knows
            // a previous hash or can forge a Host identity.
            _,err=Select(context.Background(),root,SelectionRequest{
                RunID:run.RunID,OptionsHash:opts.Hash,IDs:[]string{"C1"}},
                HostTurn{SessionID:"forged",MessageID:"forged"})
            if err==nil || !strings.Contains(err.Error(),"REVIEW_SELECTION_NOT_REQUIRED"){
                t.Fatalf("manual selection unexpectedly accepted: %v",err)
            }
        })
    }
}

func TestT5ControllerGroupingPreservesUnresolvedAndMavenIdentity(t *testing.T){
    chains:=[]Chain{
        {Name:"OrderController.create",Nodes:[]Node{{Path:"module-a/src/main/java/x/OrderController.java",Symbol:"OrderController.create",Role:"CONTROLLER"}}},
        {Name:"OrderController.cancel",Nodes:[]Node{{Path:"module-a/src/main/java/x/OrderController.java",Symbol:"OrderController.cancel",Role:"CONTROLLER"}},Unresolved:[]string{"MAPPER_NOT_RESOLVED"}},
        {Name:"OrderController.create",Nodes:[]Node{{Path:"module-b/src/main/java/x/OrderController.java",Symbol:"OrderController.create",Role:"CONTROLLER"}}},
    }
    choices:=groupControllerChains180(chains,"CHANGES")
    if len(choices)!=2||choices[0].ControllerPath==choices[1].ControllerPath||
       !strings.Contains(strings.Join(choices[0].Unresolved,","),"MAPPER_NOT_RESOLVED"){
        t.Fatalf("unresolved child or module identity was lost: %+v",choices)
    }
}

func TestT5OneControllerMixedResolvedAndUnresolvedMethodsIsPartial(t *testing.T){
    root:=copyControllerReviewFixture180(t)
    useRealAstGrep180(t,root)
    replaceT5Fixture180(t,root,
        "src/main/java/com/example/OrderMapper.java",
        "void cancelOrder();","void cancelOrderRenamed();")
    run,err:=Start(root)
    if err!=nil{t.Fatal(err)}
    opts,err:=Prepare(context.Background(),root,run.RunID,Intent{Mode:"CURRENT_IMPLEMENTATION",Target:"OrderController"})
    if err!=nil{t.Fatal(err)}
    if opts.DiscoveryComplete||opts.SelectionRequired||len(opts.Chains)!=1||
       len(opts.Chains[0].Methods)!=2||len(opts.Chains[0].Unresolved)==0 {
        t.Fatalf("mixed complete/incomplete Controller was falsely COMPLETE: %+v",opts)
    }
    scope,err:=loadScope180(filepath.Dir(run.ReportPath),run.RunID)
    if err!=nil{t.Fatal(err)}
    if scope.Coverage!="PARTIAL"||len(scope.Chains)!=2 {
        t.Fatalf("mixed Controller lost PARTIAL method-level state: %+v",scope)
    }
    _,state,err:=loadRun(root,run.RunID)
    if err!=nil{t.Fatal(err)}
    if !state.ScopeReady||state.Coverage!="PARTIAL" {
        t.Fatalf("auto-selected partial Controller marked COMPLETE or not scoped: %+v",state)
    }
}

func TestT5TamperedControllerChildInvalidatesPersistedOptionsHash(t *testing.T) {
    root:=copyControllerReviewFixture180(t)
    addSecondController180(t,root)
    useRealAstGrep180(t,root)
    run,err:=Start(root)
    if err!=nil{t.Fatal(err)}
    opts,err:=Prepare(context.Background(),root,run.RunID,Intent{Mode:"CURRENT_IMPLEMENTATION"})
    if err!=nil{t.Fatal(err)}
    if len(opts.Chains)!=2 || opts.SelectionRequired {t.Fatalf("need Controller menu: %+v",opts)}
    path:=filepath.Join(filepath.Dir(run.ReportPath),"options.json")
    bytes,err:=os.ReadFile(path)
    if err!=nil{t.Fatal(err)}
    // Modify only a method-child symbol, not the Controller menu name or
    // saved hash, simulating subchain expansion/tampering after prepare.
    before:="OrderServiceImpl.create"
    after:="OrderServiceImpl.attackerAdded"
    if !strings.Contains(string(bytes),before){t.Fatalf("missing actual child in options: %s",bytes)}
    changed:=strings.Replace(string(bytes),before,after,1)
    if err:=os.WriteFile(path,[]byte(changed),0600);err!=nil{t.Fatal(err)}
    _,err=loadPreparedOptions180(filepath.Dir(run.ReportPath))
    if err==nil || !strings.Contains(err.Error(),"REVIEW_OPTIONS_STALE") {
        t.Fatalf("edited method subchain accepted despite immutable optionsHash: %v",err)
    }
}

func TestT5ChangesNoTargetReviewsEveryAffectedControllerWithoutSelection(t *testing.T){
    for _,tc:=range []struct{target string;controllers int;methods int}{
        {"",2,4},{"OrderController",1,2},{"UserController",1,2},
    }{
        t.Run("target="+tc.target,func(t *testing.T){
            root:=copyControllerReviewFixture180(t)
            addSecondController180(t,root)
            useRealAstGrep180(t,root)
            initControllerReviewGitBaseline180(t,root)
            replaceT5Fixture180(t,root,"src/main/java/com/example/OrderServiceImpl.java",
                "orderMapper.insertOrder();","orderMapper.insertOrder(); /* changed create */")
            replaceT5Fixture180(t,root,"src/main/java/com/example/OrderServiceImpl.java",
                "orderMapper.cancelOrder();","orderMapper.cancelOrder(); /* changed cancel */")
            run,err:=Start(root);if err!=nil{t.Fatal(err)}
            opts,err:=Prepare(context.Background(),root,run.RunID,
                Intent{Mode:"CHANGES",Target:tc.target})
            if err!=nil{t.Fatal(err)}
            if !opts.DiscoveryComplete||opts.SelectionRequired||len(opts.Chains)!=tc.controllers {
                t.Fatalf("target/all CHANGES Controller scope mismatch: %+v",opts)
            }
            scope,err:=loadScope180(filepath.Dir(run.ReportPath),run.RunID)
            if err!=nil{t.Fatal(err)}
            if scope.Coverage!="COMPLETE"||len(scope.Chains)!=tc.methods ||
                len(scope.SelectedIDs)!=tc.controllers {
                t.Fatalf("exact impacted Controller methods missing: %+v",scope)
            }
            for _,method:=range scope.Chains{
                if tc.target!="" && !strings.HasPrefix(method.Name,tc.target+"."){
                    t.Fatalf("unrelated Controller leaked into explicit target: %+v",method)
                }
            }
        })
    }
}
