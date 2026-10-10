package reviewrun

import (
    "fmt"
    "path/filepath"
    "strings"

    "codea-harness-tools/internal/nav"
)

const (
    maxReviewChainDepth180 = 6
    maxReviewChainEdges180 = 96
)

// buildBoundedChain180 traverses all direct internal call edges instead of
// accepting only exactly one call at each layer. Unknown, cyclic or ambiguous
// edges remain explicit gaps; a single valid branch never conceals a gap in
// another branch. No per-call ast-grep or network access is performed.
func buildBoundedChain180(
    root string, ep nav.ControllerEndpointMatch, facts scopedNavigation180,
    xmlCandidates map[string][]string, impact *impactRanges180,
) (Chain, bool) {
    ch:=Chain{Name:ep.Symbol,Nodes:[]Node{{Path:filepath.ToSlash(ep.Path),Symbol:ep.Symbol,Role:"CONTROLLER",Workspace:"current"}},Unresolved:[]string{}}
    affected:=false
    if impact!=nil && impact.intersects(ep.Path,ep.StartLine,ep.EndLine) {affected=true}
    seenNodes:=map[string]bool{}
    seenNodes["CONTROLLER\x00"+ep.Path+"\x00"+ep.Symbol]=true
    addNode:=func(node Node) {
        node.Path=filepath.ToSlash(node.Path)
        key:=node.Role+"\x00"+node.Path+"\x00"+node.Symbol
        if !seenNodes[key] {seenNodes[key]=true;ch.Nodes=append(ch.Nodes,node)}
    }
    edges:=0
    active:=map[string]bool{}
    var walk func(from string,depth int)
    walk=func(from string,depth int) {
        if depth>maxReviewChainDepth180 {
            ch.Unresolved=append(ch.Unresolved,"CALL_DEPTH_LIMIT_REACHED: "+from)
            return
        }
        if active[from] {
            ch.Unresolved=append(ch.Unresolved,"CALL_CYCLE_UNRESOLVED: "+from)
            return
        }
        calls:=facts.Calls[from]
        if len(calls)==0 {
            // A leaf helper/service that performs validation or computation
            // legitimately has no further internal calls. The Controller and
            // first Service hop still require an observable downstream edge.
            if depth<=1 {ch.Unresolved=append(ch.Unresolved,fmt.Sprintf("%s direct internal calls=0",from))}
            return
        }
        active[from]=true
        defer delete(active,from)
        for _,call:=range calls {
            edges++
            if edges>maxReviewChainEdges180 {
                ch.Unresolved=append(ch.Unresolved,"CALL_EDGE_LIMIT_REACHED: "+from)
                return
            }
            if !call.Resolved {
                ch.Unresolved=append(ch.Unresolved,from+" receiver unresolved at "+call.Path)
                continue
            }
            method:=call.TargetSymbol
            if info,ok:=facts.Infos[method];ok && strings.HasSuffix(call.ReceiverType,"Mapper") {
                // A Mapper declaration without an unambiguous matching SQL
                // remains an explicit gap, never an implementation guess.
                if xmlKey,identityOK:=mapperXMLIdentity180(root,info,method);identityOK {
                    if xp,found:=selectMapperXML180(info.Path,xmlCandidates[xmlKey]);found {
                        addNode(Node{Path:info.Path,Symbol:method,Role:"MAPPER",Workspace:"current"})
                        addNode(Node{Path:xp,Symbol:method,Role:"SQL",Workspace:"current"})
                        if impact!=nil {
                            if impact.intersects(info.Path,info.LineStart,info.LineEnd) {affected=true}
                            if impact.statement(root,xp,xmlKey) {affected=true}
                        }
                        continue
                    }
                }
                ch.Unresolved=append(ch.Unresolved,method+" XML statement unresolved")
                continue
            }
            // Java this/super/unqualified calls are direct methods on the
            // current implementation, not interface-implementation lookups.
            owner:=from
            if at:=strings.LastIndexByte(from,'.');at>=0 {owner=from[:at]}
            if owner==call.ReceiverType {
                own:=[]nav.MethodSpan180{}
                for _,span:=range facts.Methods[method] {
                    if filepath.ToSlash(span.Path)==filepath.ToSlash(call.Path) {own=append(own,span)}
                }
                if len(own)!=1 {
                    ch.Unresolved=append(ch.Unresolved,"SELF_METHOD_UNRESOLVED: "+method)
                    continue
                }
                addNode(Node{Path:own[0].Path,Symbol:method,Role:"SERVICE",Workspace:"current"})
                if impact!=nil && impact.method(own,own[0].Path,method) {affected=true}
                walk(method,depth+1)
                continue
            }
            xs:=facts.Impls[call.ReceiverType]
            if len(xs)==0 {
                // A field statically typed as an ordinary concrete class
                // needs no interface implementation lookup. Only a unique
                // in-scope actual method body can authorize this edge.
                bodies:=facts.Methods[method]
                if len(bodies)==1 && filepath.ToSlash(bodies[0].Path)==filepath.ToSlash(call.Path) {
                    addNode(Node{Path:bodies[0].Path,Symbol:method,Role:"SERVICE",Workspace:"current"})
                    if impact!=nil && impact.method(bodies,bodies[0].Path,method) {affected=true}
                    walk(method,depth+1)
                    continue
                }
            }
            if len(xs)!=1 {
                // Retain proven candidate impact but never guess which
                // implementation runs or authorize AUTO_SINGLE.
                if impact != nil {
                    for _,candidate:=range xs {
                        sym:=candidate.Symbol+"."+call.Method
                        if impact.method(facts.Methods[sym],candidate.Path,sym) {affected=true}
                    }
                }
                ch.Unresolved=append(ch.Unresolved,fmt.Sprintf("%s implementations=%d",call.ReceiverType,len(xs)))
                continue
            }
            service:=xs[0]
            symbol:=service.Symbol+"."+call.Method
            addNode(Node{Path:service.Path,Symbol:symbol,Role:"SERVICE",Workspace:"current"})
            if impact!=nil && impact.method(facts.Methods[symbol],service.Path,symbol) {affected=true}
            walk(symbol,depth+1)
        }
    }
    walk(ep.Symbol,0)
    return ch,affected
}
