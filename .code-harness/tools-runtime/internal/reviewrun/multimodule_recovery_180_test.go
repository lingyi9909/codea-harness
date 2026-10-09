package reviewrun

import (
 "context"
 "os"
 "path/filepath"
 "strings"
 "testing"
)

func Test180JavaSourceRootAcrossModules(t *testing.T) {
 cases := map[string]string{
   "a-module/src/main/java/acme/AController.java":"a-module/src/main/java",
   "z-module/src/main/java/acme/OrderServiceImpl.java":"z-module/src/main/java",
   "src/main/java/com/example/OrderController.java":"src/main/java",
   "src/test/java/com/example/Suite.java":"src",
 }
 for input,want := range cases {
   if got:=javaSourceRoot180(input);got!=want {t.Fatalf("root(%q)=%q want=%q",input,got,want)}
 }
}

func Test180PrepareNoRelevantChangesRetainsRunForRetry(t *testing.T) {
 root:=copyControllerReviewFixture180(t)
 initControllerReviewGitBaseline180(t,root)
 started,err:=Start(root)
 if err!=nil {t.Fatal(err)}
 first,err:=Prepare(context.Background(),root,started.RunID,Intent{Mode:"CHANGES",Target:"OrderController.create"})
 if err!=nil||first.DiscoveryComplete||!strings.Contains(strings.Join(first.Gaps,";"),"NO_RELEVANT_CHANGES"){
   t.Fatalf("no-diff must be retriable INCOMPLETE, options=%+v err=%v",first,err)
 }
 status,err:=Status(root,started.RunID)
 if err!=nil||status.Execution!="INCOMPLETE" {t.Fatalf("wrong status: %+v %v",status,err)}
 file:=filepath.Join(root,"src","main","java","com","example","OrderController.java")
 f,err:=os.OpenFile(file,os.O_APPEND|os.O_WRONLY,0)
 if err!=nil{t.Fatal(err)}
 if _,err=f.WriteString("\n// changed after original start\n");err!=nil{t.Fatal(err)}
 if err=f.Close();err!=nil{t.Fatal(err)}
 paths,err:=changedSourceFiles180(context.Background(),root)
 if err!=nil||len(paths)!=1{t.Fatalf("same run changes now missing: %v %v",paths,err)}
 useRealAstGrep180(t,root)
 recovered,err:=Prepare(context.Background(),root,started.RunID,Intent{Mode:"CHANGES",Target:"OrderController.create"})
 if err!=nil||!recovered.DiscoveryComplete||len(recovered.Chains)!=1||recovered.RunID!=started.RunID{
    t.Fatalf("cannot recover original run: %+v %v",recovered,err)
 }
}

func Test180MultiModulePrepareScansAllRoots(t *testing.T) {
 root:=copyControllerReviewFixture180(t)
 // Put the real reviewed module after the empty module in lexical order.
 src:=filepath.Join(root,"src")
 dst:=filepath.Join(root,"z-module","src")
 if err:=os.MkdirAll(filepath.Dir(dst),0755);err!=nil{t.Fatal(err)}
 if err:=os.Rename(src,dst);err!=nil{t.Fatal(err)}
 dummy:=filepath.Join(root,"a-module","src","main","java","acme","Utility.java")
 if err:=os.MkdirAll(filepath.Dir(dummy),0755);err!=nil{t.Fatal(err)}
 if err:=os.WriteFile(dummy,[]byte("package acme; class Utility {}\n"),0600);err!=nil{t.Fatal(err)}
 useRealAstGrep180(t,root)
 started,err:=Start(root)
 if err!=nil{t.Fatal(err)}
 opts,err:=Prepare(context.Background(),root,started.RunID,Intent{Mode:"CURRENT_IMPLEMENTATION",Target:"OrderController.create"})
 if err!=nil||!opts.DiscoveryComplete||len(opts.Chains)!=1||len(opts.Chains[0].Nodes)<3 {
  t.Fatalf("cross-module navigation dropped internal calls: options=%+v err=%v",opts,err)
 }
}

func Test180UnrelatedControllerNotMarkedChangedByIncompleteCall(t *testing.T) {
 root:=copyControllerReviewFixture180(t)
 other:="src/main/java/com/example/UnrelatedController.java"
 p:=filepath.Join(root,filepath.FromSlash(other))
 source:="package com.example;\nimport org.springframework.web.bind.annotation.RestController;\nimport org.springframework.web.bind.annotation.GetMapping;\n@RestController public class UnrelatedController {\n @GetMapping(\"/unrelated\") public String status() { return \"ok\"; }\n}\n"
 if err:=os.WriteFile(p,[]byte(source),0600);err!=nil{t.Fatal(err)}
 initControllerReviewGitBaseline180(t,root)
 changed:=filepath.Join(root,"src","main","java","com","example","OrderController.java")
 f,err:=os.OpenFile(changed,os.O_APPEND|os.O_WRONLY,0)
 if err!=nil{t.Fatal(err)}
 if _,err=f.WriteString("\n// one controller changed\n");err!=nil{t.Fatal(err)}
 if err=f.Close();err!=nil{t.Fatal(err)}
 useRealAstGrep180(t,root)
 start,err:=Start(root)
 if err!=nil{t.Fatal(err)}
 opts,err:=Prepare(context.Background(),root,start.RunID,Intent{Mode:"CHANGES"})
 if err!=nil{t.Fatal(err)}
 if len(opts.Chains)==0 {t.Fatalf("actual changed controller was lost: %+v",opts)}
 for _,ch:=range opts.Chains {
  if strings.HasPrefix(ch.Name,"UnrelatedController.") {
   t.Fatalf("unrelated unresolved Controller incorrectly selected: %+v",ch)
  }
 }
}
