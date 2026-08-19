package nav_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"codea-harness-tools/internal/nav"
)

type fakeRunner struct { calls [][]string; out []byte }
func (f *fakeRunner) Run(_ context.Context,_ string,args ...string)([]byte,error){ f.calls=append(f.calls,append([]string(nil),args...)); return f.out,nil }

func TestFindImplementationsUsesFixedAstGrepArguments(t *testing.T){
	r:=&fakeRunner{out:[]byte(`{"file":"src/main/java/OrderServiceImpl.java","range":{"start":{"line":4,"column":0},"end":{"line":4,"column":40}},"text":"class OrderServiceImpl implements OrderService"}`+"\n")}
	n:=nav.Navigator{RepoRoot:`C:\\repo`,AstGrepPath:`C:\\repo\\.code-harness\\bin\\ast-grep.exe`,Runner:r}
	got,err:=n.FindImplementations(context.Background(),"OrderService","src/main/java"); if err!=nil{t.Fatal(err)}
	if len(got.Matches)!=1||got.Matches[0].Path!="src/main/java/OrderServiceImpl.java"{t.Fatalf("got=%+v",got)}
	if len(r.calls)==0{t.Fatal("ast-grep not invoked")}
	for _,a:=range r.calls[0]{ if a=="cmd"||a=="/c"||a=="powershell"||a=="bash"||a=="-c"{t.Fatalf("shell arg leaked: %v",r.calls[0])} }
}
func TestNavigationRejectsScopeEscape(t *testing.T){ n:=nav.Navigator{RepoRoot:`C:\\repo`,AstGrepPath:`C:\\repo\\.code-harness\\bin\\ast-grep.exe`,Runner:&fakeRunner{}}; _,err:=n.FindSymbol(context.Background(),"OrderService","../outside"); if !errors.Is(err,nav.ErrInvalidScope){t.Fatalf("err=%v",err)} }
func TestNavigationRejectsShellMetacharactersInSymbol(t *testing.T){ n:=nav.Navigator{RepoRoot:`C:\\repo`,AstGrepPath:`C:\\repo\\.code-harness\\bin\\ast-grep.exe`,Runner:&fakeRunner{}}; _,err:=n.FindReferences(context.Background(),"OrderService;calc.exe","src/main/java"); if !errors.Is(err,nav.ErrInvalidSymbol){t.Fatalf("err=%v",err)} }
func TestFindReferencesBuildsMethodCallPatternWithoutUserCommand(t *testing.T){ r:=&fakeRunner{out:[]byte{}}; n:=nav.Navigator{RepoRoot:`C:\\repo`,AstGrepPath:`C:\\repo\\.code-harness\\bin\\ast-grep.exe`,Runner:r}; _,err:=n.FindReferences(context.Background(),"OrderService.approve","src/main/java"); if err!=nil{t.Fatal(err)}; want:=[]string{"--lang","java","--json=stream","--pattern","$OBJ.approve($$$ARGS)","src/main/java"}; found:=false; for _,call:=range r.calls{if reflect.DeepEqual(call,want){found=true}}; if !found{t.Fatalf("calls=%v",r.calls)} }
