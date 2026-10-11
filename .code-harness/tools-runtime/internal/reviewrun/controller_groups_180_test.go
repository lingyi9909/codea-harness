package reviewrun

import (
    "context"
    "os"
    "path/filepath"
    "strings"
    "testing"
    "codea-harness-tools/internal/reviewauthority"
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

func TestT5TwoControllersEachTwoMethodsSelectExactController(t *testing.T){
    root:=copyControllerReviewFixture180(t)
    addSecondController180(t,root)
    useRealAstGrep180(t,root)
    run,err:=Start(root);if err!=nil{t.Fatal(err)}
    opts,err:=Prepare(context.Background(),root,run.RunID,Intent{Mode:"CURRENT_IMPLEMENTATION"})
    if err!=nil{t.Fatal(err)}
    if len(opts.Chains)!=2||!opts.SelectionRequired||!opts.DiscoveryComplete {
        t.Fatalf("need exactly two Controller menu options: %+v",opts)
    }
    if !strings.HasPrefix(opts.Chains[0].Name,"OrderController")||
       !strings.HasPrefix(opts.Chains[1].Name,"UserController") {
        t.Fatalf("Controller menu order wrong: %+v",opts.Chains)
    }
    for _,c:=range opts.Chains{if len(c.Methods)!=2{t.Fatalf("Controller lost affected child methods: %+v",c)}}
    old:=verifySelectionTurn180
    verifySelectionTurn180=func(context.Context,string,reviewauthority.SelectionTurnRequest180)error{return nil}
    defer func(){verifySelectionTurn180=old}()
    out,err:=Select(context.Background(),root,SelectionRequest{
        RunID:run.RunID,OptionsHash:opts.Hash,IDs:[]string{"C1"}},
        HostTurn{SessionID:"unit-host",MessageID:"unit-user"})
    if err!=nil{t.Fatal(err)}
    if out.Coverage!="COMPLETE"{t.Fatalf("selected Controller coverage: %+v",out)}
    scope,err:=loadScope180(filepath.Dir(run.ReportPath),run.RunID)
    if err!=nil{t.Fatal(err)}
    if len(scope.Chains)!=2||len(scope.SelectedIDs)!=1||scope.SelectedIDs[0]!="C1"{
        t.Fatalf("selection failed to expand only Controller methods: %+v",scope)
    }
    for _,m:=range scope.Chains{
        if !strings.HasPrefix(m.Name,"OrderController.") {
            t.Fatalf("unselected Controller method leaked into review: %+v",m)
        }
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
