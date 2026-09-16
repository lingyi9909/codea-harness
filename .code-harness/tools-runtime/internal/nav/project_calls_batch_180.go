package nav

import (
    "context"
    "fmt"
    "sort"
    "strings"
)

// FindDirectMethodCallsBatch180 snapshots Java declarations and call expressions
// with one ast-grep process, then projects direct calls for every method. It is
// intentionally narrow for the 1.8 review prepare path.
func (n Navigator) FindDirectMethodCallsBatch180(ctx context.Context, scope string) (map[string][]DirectMethodCall, error) {
    patterns := append([]string{}, allTypePatterns()...)
    patterns = append(patterns, allMethodPatterns()...)
    patterns = append(patterns, fieldDeclarationKind167, "$OBJ.$M($$$ARGS)", "$M($$$ARGS)")
    records, err := n.runRawBatch167(ctx, scope, "codea-direct-calls-180", patterns)
    if err != nil { return nil, err }
    var types, methods, calls []rawMatch
    for _, r := range records {
        if r.RuleID == "codea-direct-calls-180-fields" { continue }
        if k, _ := typeKindAndName(r.Text); k != "" { types = append(types, r); continue }
        if _, _, ok := directCallParts163(r.Text); ok { calls = append(calls, r); continue }
        if methodName(r.Text) != "" { methods = append(methods, r); continue }
    }
    out := map[string][]DirectMethodCall{}
    for _, method := range methods {
        ownerType, ok := smallestContaining(types, method); if !ok { continue }
        _, owner := typeKindAndName(ownerType.Text); member := methodName(method.Text)
        if owner == "" || member == "" { continue }
        from := owner+"."+member
        seen := map[string]bool{}
        for _, call := range calls {
            if !contains(method, call) { continue }
            receiver, called, ok := directCallParts163(call.Text); if !ok { continue }
            fact := DirectMethodCall{FromSymbol:from, Receiver:receiver, Method:called, Path:call.Path, Line:call.StartLine}
            switch receiver {
            case "", "this", "super":
                fact.ReceiverType = owner; fact.TargetSymbol = owner+"."+called; fact.Resolved = true
            default:
                if typ, ok := receiverFieldType(types, call, receiver); ok {
                    fact.ReceiverType = simpleType(typ)
                    if identRE.MatchString(fact.ReceiverType) { fact.TargetSymbol = fact.ReceiverType+"."+called; fact.Resolved = true }
                }
            }
            key := fmt.Sprintf("%s:%d:%s:%s", fact.Path, fact.Line, fact.TargetSymbol, fact.Receiver)
            if !seen[key] { seen[key]=true; out[from]=append(out[from],fact) }
        }
        sort.Slice(out[from], func(i,j int) bool { if out[from][i].Line!=out[from][j].Line{return out[from][i].Line<out[from][j].Line}; return out[from][i].TargetSymbol<out[from][j].TargetSymbol })
    }
    return out,nil
}

// FindImplementationTypesBatch180 resolves concrete implementations for a set
// of receiver types in one ast-grep process. It never guesses from class names.
func (n Navigator) FindImplementationTypesBatch180(ctx context.Context, owners []string, scope string) (map[string][]ImplementationType,error) {
    uniq := map[string]bool{}; clean := make([]string,0,len(owners)); patterns:=[]string{}
    for _, owner := range owners { owner=strings.TrimSpace(owner); if owner==""||uniq[owner] {continue}; if err:=n.validate(owner,scope); err!=nil{return nil,err}; uniq[owner]=true; clean=append(clean,owner); patterns=append(patterns, implementationPatterns180(owner)...)}
    out:=map[string][]ImplementationType{}; if len(patterns)==0{return out,nil}
    records,err:=n.runRawBatch167(ctx,scope,"codea-implementations-180",patterns); if err!=nil{return nil,err}
    for _, r:=range records { kind,name:=typeKindAndName(r.Text); if kind!="CLASS"||name==""{continue}; for _,owner:=range clean { if !declarationRelates180(r.Text,owner){continue}; key:=r.Path+"\x00"+name; duplicate:=false; for _,v:=range out[owner]{if v.Path+"\x00"+v.Symbol==key{duplicate=true;break}}; if !duplicate{out[owner]=append(out[owner],ImplementationType{Symbol:name,Path:r.Path})} } }
    for owner:=range out { sort.Slice(out[owner],func(i,j int)bool{if out[owner][i].Path!=out[owner][j].Path{return out[owner][i].Path<out[owner][j].Path};return out[owner][i].Symbol<out[owner][j].Symbol}) }
    return out,nil
}

func implementationPatterns180(owner string) []string {
    bases:=[]string{"class $C implements "+owner+" { $$$BODY }","public class $C implements "+owner+" { $$$BODY }","final class $C implements "+owner+" { $$$BODY }","public final class $C implements "+owner+" { $$$BODY }","class $C extends $SUPER implements "+owner+" { $$$BODY }","public class $C extends $SUPER implements "+owner+" { $$$BODY }","class $C extends "+owner+" { $$$BODY }","public class $C extends "+owner+" { $$$BODY }"}
    return withAnnotationVariants(bases)
}
func declarationRelates180(text, owner string) bool { normalized:=strings.NewReplacer("{"," ","}"," ",","," ","\n"," ","\r"," ","\t"," ").Replace(text); fields:=strings.Fields(normalized); for i,f:=range fields { if (f=="implements"||f=="extends") && i+1<len(fields) && simpleType(strings.Trim(fields[i+1],","))==owner{return true} }; return false }
