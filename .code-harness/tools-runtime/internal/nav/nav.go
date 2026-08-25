package nav

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path"
	"regexp"
	"strings"
)

var ErrInvalidScope = errors.New("invalid navigation scope")
var ErrInvalidSymbol = errors.New("invalid symbol")
var symbolRE = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*(\.[A-Za-z_$][A-Za-z0-9_$]*)?$`)

type Runner interface { Run(context.Context, string, ...string) ([]byte, error) }
type ExecRunner struct{}
func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) { return exec.CommandContext(ctx, name, args...).Output() }

type Match struct { Path string `json:"path"`; Line int `json:"line"`; Column int `json:"column"`; Text string `json:"text"` }
type Result struct { Symbol string `json:"symbol"`; Scope string `json:"scope"`; Matches []Match `json:"matches"` }
type Navigator struct { RepoRoot, AstGrepPath string; Runner Runner }
type sgLine struct { File string `json:"file"`; Text string `json:"text"`; Range struct { Start struct { Line int `json:"line"`; Column int `json:"column"` } `json:"start"` } `json:"range"` }

func (n Navigator) validate(symbol, scope string) error {
	if !symbolRE.MatchString(symbol) { return ErrInvalidSymbol }
	scope = strings.ReplaceAll(scope, "\\", "/")
	clean := path.Clean(scope)
	if clean == ".." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) || clean == "." || clean == "" { return ErrInvalidScope }
	return nil
}
func (n Navigator) run(ctx context.Context, symbol, scope string, patterns ...string) (Result, error) {
	if err := n.validate(symbol, scope); err != nil { return Result{}, err }
	runner := n.Runner; if runner == nil { runner = ExecRunner{} }
	res := Result{Symbol:symbol, Scope:scope}; seen := map[string]bool{}
	for _, p := range patterns {
		args := []string{"--lang","java","--json=stream","--pattern",p,strings.ReplaceAll(scope,"\\","/")}
		out, err := runner.Run(ctx,n.AstGrepPath,args...)
		if err != nil { var ee *exec.ExitError; if !errors.As(err,&ee) { return Result{},err }; if len(out)==0 { continue } }
		scanner := bufio.NewScanner(bytes.NewReader(out))
		for scanner.Scan() { var s sgLine; if json.Unmarshal(scanner.Bytes(),&s)!=nil { continue }; k:=fmt.Sprintf("%s:%d:%d",s.File,s.Range.Start.Line,s.Range.Start.Column); if seen[k] { continue }; seen[k]=true; res.Matches=append(res.Matches,Match{Path:strings.ReplaceAll(s.File,"\\","/"),Line:s.Range.Start.Line+1,Column:s.Range.Start.Column+1,Text:s.Text}) }
	}
	return res,nil
}
func splitSymbol(s string)(owner,member string){ if i:=strings.Index(s,"."); i>=0 { return s[:i],s[i+1:] }; return s,"" }
func (n Navigator) FindSymbol(ctx context.Context,symbol,scope string)(Result,error){ owner,member:=splitSymbol(symbol); if member!="" { return n.run(ctx,symbol,scope,"$RET "+member+"($$$ARGS) { $$$BODY }") }; return n.run(ctx,symbol,scope,"class "+owner+" { $$$BODY }","interface "+owner+" { $$$BODY }","enum "+owner+" { $$$BODY }") }
func (n Navigator) FindReferences(ctx context.Context,symbol,scope string)(Result,error){ _,member:=splitSymbol(symbol); if member!="" { return n.run(ctx,symbol,scope,"$OBJ."+member+"($$$ARGS)",member+"($$$ARGS)") }; return n.run(ctx,symbol,scope,"$T "+symbol,"new "+symbol+"($$$ARGS)") }
func (n Navigator) FindImplementations(ctx context.Context,symbol,scope string)(Result,error){
	owner,_:=splitSymbol(symbol)
	patterns:=[]string{
		"class $C implements "+owner+" { $$$BODY }",
		"public class $C implements "+owner+" { $$$BODY }",
		"final class $C implements "+owner+" { $$$BODY }",
		"public final class $C implements "+owner+" { $$$BODY }",
		"abstract class $C implements "+owner+" { $$$BODY }",
		"public abstract class $C implements "+owner+" { $$$BODY }",
		"class $C extends $SUPER implements "+owner+" { $$$BODY }",
		"public class $C extends $SUPER implements "+owner+" { $$$BODY }",
		"final class $C extends $SUPER implements "+owner+" { $$$BODY }",
		"public final class $C extends $SUPER implements "+owner+" { $$$BODY }",
		"abstract class $C extends $SUPER implements "+owner+" { $$$BODY }",
		"public abstract class $C extends $SUPER implements "+owner+" { $$$BODY }",
		"class $C extends "+owner+" { $$$BODY }",
		"public class $C extends "+owner+" { $$$BODY }",
		"final class $C extends "+owner+" { $$$BODY }",
		"public final class $C extends "+owner+" { $$$BODY }",
		"abstract class $C extends "+owner+" { $$$BODY }",
		"public abstract class $C extends "+owner+" { $$$BODY }",
	}
	return n.run(ctx,symbol,scope,patterns...)
}
