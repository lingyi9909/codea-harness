package reviewrun

import (
 "context"
 "os"
 "path/filepath"
 "strings"
 "testing"
)

// Distinct Maven modules may contain identically named Controllers.
// Impact selection is keyed by entrypoint identity (path + symbol), never
// by the display symbol alone.
func Test180ChangesSameNamedControllersInDifferentModulesDoNotCrossContaminate(t *testing.T) {
 root := copyControllerReviewFixture180(t)
 useRealAstGrep180(t, root)
 src := filepath.Join(root, "src")
 moduleA := filepath.Join(root, "module-a", "src")
 if err := os.MkdirAll(filepath.Dir(moduleA), 0755); err != nil { t.Fatal(err) }
 if err := os.Rename(src, moduleA); err != nil { t.Fatal(err) }
 err := filepath.WalkDir(moduleA, func(p string, d os.DirEntry, walkErr error) error {
  if walkErr != nil { return walkErr }
  rel, err := filepath.Rel(moduleA, p)
  if err != nil { return err }
  target := filepath.Join(root, "module-b", "src", rel)
  if d.IsDir() { return os.MkdirAll(target, 0755) }
  data, err := os.ReadFile(p)
  if err != nil { return err }
  return os.WriteFile(target, data, 0600)
 })
 if err != nil { t.Fatal(err) }
 initControllerReviewGitBaseline180(t, root)
 replaceT5Fixture180(t,root,"module-a/src/main/java/com/example/OrderController.java","orderService.create();","orderService.create(); // only module-a changed")
 start, err := Start(root)
 if err != nil { t.Fatal(err) }
 opts, err := Prepare(context.Background(), root, start.RunID, Intent{Mode:"CHANGES", Target:"OrderController.create"})
 if err != nil { t.Fatal(err) }
 if len(opts.Chains) != 1 { t.Fatalf("unmodified duplicate Controller was falsely affected: %+v", opts) }
 if !strings.HasPrefix(opts.Chains[0].Nodes[0].Path, "module-a/") {
  t.Fatalf("wrong module attributed to change: %+v", opts.Chains[0])
 }
}


func TestT5DuplicateControllerMenuIsDisambiguatedAndPathTargetResolves(t *testing.T) {
    root := copyControllerReviewFixture180(t)
    useRealAstGrep180(t, root)
    src := filepath.Join(root, "src")
    moduleA := filepath.Join(root,"module-a","src")
    if err:=os.MkdirAll(filepath.Dir(moduleA),0755);err!=nil {t.Fatal(err)}
    if err:=os.Rename(src,moduleA);err!=nil {t.Fatal(err)}
    err:=filepath.WalkDir(moduleA,func(p string,d os.DirEntry,e error) error {
        if e!=nil {return e}
        rel,e:=filepath.Rel(moduleA,p)
        if e!=nil {return e}
        dst:=filepath.Join(root,"module-b","src",rel)
        if d.IsDir() {return os.MkdirAll(dst,0755)}
        b,e:=os.ReadFile(p)
        if e!=nil {return e}
        return os.WriteFile(dst,b,0600)
    })
    if err!=nil {t.Fatal(err)}
    start,err:=Start(root)
    if err!=nil {t.Fatal(err)}
    opts,err:=Prepare(context.Background(),root,start.RunID,Intent{Mode:"CURRENT_IMPLEMENTATION",Target:"OrderController.create"})
    if err!=nil {t.Fatal(err)}
    if !opts.DiscoveryComplete||!opts.SelectionRequired||len(opts.Chains)!=2 {
        t.Fatalf("duplicate modules need exactly two distinct real options: %+v",opts)
    }
    for i,ch:=range opts.Chains {
        if !strings.Contains(ch.Name,"OrderController.create [module-")||!strings.Contains(ch.Name,"OrderController.java]") {
            t.Fatalf("candidate %d cannot be distinguished by module: %+v",i,ch)
        }
    }
    if opts.Chains[0].Name==opts.Chains[1].Name {t.Fatal("duplicate menu names remain indistinguishable")}
    file:="module-b/src/main/java/com/example/OrderController.java"
    second,err:=Start(root)
    if err!=nil {t.Fatal(err)}
    pathOptions,err:=Prepare(context.Background(),root,second.RunID,Intent{Mode:"CURRENT_IMPLEMENTATION",Target:file})
    if err!=nil {t.Fatal(err)}
    if !pathOptions.DiscoveryComplete||len(pathOptions.Chains)!=2 {
        t.Fatalf("exact module path was not accepted as Controller target: %+v",pathOptions)
    }
    for _,ch:=range pathOptions.Chains {
        if ch.Nodes[0].Path!=file {t.Fatalf("path target leaked another Maven module: %+v",ch)}
    }
}
