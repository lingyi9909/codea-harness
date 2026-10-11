package reviewrun

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

// A genuine CHANGES regression: five independently routed Controller
// endpoints call a single changed Service method. The sixth sibling method
// is unchanged and MUST NOT enter this review merely because the Controller
// is grouped as one human choice.
func TestT5ActualFiveImpactedControllerMethodsAutoSelectWithoutSibling(t *testing.T) {
    root:=copyControllerReviewFixture180(t)
    useRealAstGrep180(t,root)
    controller:="src/main/java/com/example/OrderController.java"
    path:=filepath.Join(root,filepath.FromSlash(controller))
    raw,err:=os.ReadFile(path)
    if err!=nil {t.Fatal(err)}
    source:=string(raw)
    at:=strings.LastIndex(source,"}")
    if at<0 {t.Fatal("Controller closing brace not found")}
    added:=""
    for i:=2;i<=5;i++ {
        added+=fmt.Sprintf("    @PostMapping(\"/orders/m%d\")\n    public void m%d() { orderService.create(); }\n",i,i)
    }
    // Preserve the original source's line endings for precise Git impact.
    if strings.Contains(source,"\r\n") {
        added=strings.ReplaceAll(added,"\n","\r\n")
    }
    source=source[:at]+added+source[at:]
    if err:=os.WriteFile(path,[]byte(source),0600);err!=nil{t.Fatal(err)}
    initControllerReviewGitBaseline180(t,root)
    replaceT5Fixture180(t,root,"src/main/java/com/example/OrderServiceImpl.java",
        "orderMapper.insertOrder();","orderMapper.insertOrder(); // only create changed")
    run,err:=Start(root)
    if err!=nil{t.Fatal(err)}
    opts,err:=Prepare(context.Background(),root,run.RunID,Intent{Mode:"CHANGES"})
    if err!=nil{t.Fatal(err)}
    if len(opts.Chains)!=1||opts.SelectionRequired||!opts.DiscoveryComplete||
       len(opts.Chains[0].Methods)!=5{
        t.Fatalf("five genuine changed method chains must produce one Controller option: %+v",opts)
    }
    names:=map[string]bool{}
    for _,m:=range opts.Chains[0].Methods {
        names[m.Name]=true
        if strings.HasSuffix(m.Name,".cancel") {t.Fatalf("unaffected method added to CHANGES: %+v",m)}
    }
    for _,method:=range []string{"create","m2","m3","m4","m5"} {
        if !names["OrderController."+method] {t.Fatalf("affected method %s lost: %+v",method,names)}
    }
    scope,err:=loadScope180(filepath.Dir(run.ReportPath),run.RunID)
    if err!=nil{t.Fatal(err)}
    if scope.Coverage!="COMPLETE"||len(scope.Chains)!=5||len(scope.SelectedIDs)!=1||
       scope.SelectedIDs[0]!="C1" {
        t.Fatalf("auto-selected five-method scope incomplete: %+v",scope)
    }
    for _,m:=range scope.Chains {
        if strings.HasSuffix(m.Name,".cancel") {
            t.Fatalf("unaffected sibling method leaked into scope: %+v",m)
        }
    }
}
