package nav

import (
	"context"
	"sort"
	"strings"
)

// DiscoverMethods170 returns exact CURRENT method identities only from the
// explicitly requested repository-relative Java paths. The paths are supplied
// by Runtime-owned ChangeSet authority; this method never widens them.
func (n Navigator) DiscoverMethods170(ctx context.Context, paths []string) ([]ReviewRef170, []SourceRange170, error) {
	idx, err := n.buildJavaIndex170(ctx)
	if err != nil { return nil, nil, err }
	allowed := map[string]bool{}
	for _, p := range paths {
		p = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(p), "\\", "/"))
		if p != "" { allowed[p] = true }
	}
	type pair struct { ref ReviewRef170; at SourceRange170; key string }
	pairs := []pair{}
	for _, method := range idx.Methods {
		if method == nil || method.Owner == nil || !allowed[strings.ToLower(strings.ReplaceAll(method.Owner.Path, "\\", "/"))] { continue }
		ref := idx.methodRef170(method, "current", "CURRENT")
		at, err := n.javaSourceRange170(ref, method.Raw)
		if err != nil { return nil, nil, err }
		key, err := ReviewRefKey170(ref)
		if err != nil { return nil, nil, err }
		pairs = append(pairs, pair{ref:ref, at:at, key:key})
	}
	sort.Slice(pairs, func(i,j int) bool { return pairs[i].key < pairs[j].key })
	refs := make([]ReviewRef170,0,len(pairs)); ranges := make([]SourceRange170,0,len(pairs))
	for _, p := range pairs { refs=append(refs,p.ref); ranges=append(ranges,p.at) }
	return refs,ranges,nil
}

// ExactCallers170 performs bounded reverse discovery by re-resolving every
// candidate method's forward Java calls and retaining only edges whose exact
// target identity matches the requested method. No name-only candidate becomes
// authority.
func (n Navigator) ExactCallers170(ctx context.Context, target ReviewRef170) ([]Relation170, error) {
	if target.Kind != "METHOD" || target.Side != "CURRENT" { return []Relation170{}, nil }
	targetKey, err := ReviewRefKey170(target)
	if err != nil { return nil, err }
	idx, err := n.buildJavaIndex170(ctx)
	if err != nil { return nil, err }
	if _, err := idx.findMethod170(target); err != nil { return nil, err }
	seen := map[string]bool{}; out := []Relation170{}
	for _, method := range idx.Methods {
		if err := ctx.Err(); err != nil { return nil, err }
		from := idx.methodRef170(method, target.Workspace, "CURRENT")
		calls, err := n.resolveMethodCalls170(ctx, idx, method, from)
		if err != nil { return nil, err }
		for _, call := range calls {
			rel := call.Relation
			if rel.Resolution != "EXACT" || rel.Kind != "JAVA_CALL" || len(rel.Targets) != 1 { continue }
			key, err := ReviewRefKey170(rel.Targets[0]); if err != nil { continue }
			if key != targetKey || seen[rel.ID] { continue }
			seen[rel.ID] = true; out = append(out, rel)
		}
	}
	sort.Slice(out,func(i,j int)bool{return relationSortKeyForCallers170(out[i])<relationSortKeyForCallers170(out[j])})
	return out,nil
}

func relationSortKeyForCallers170(rel Relation170) string {
	from,_:=ReviewRefKey170(rel.From)
	return from+"\x00"+rel.ID
}
