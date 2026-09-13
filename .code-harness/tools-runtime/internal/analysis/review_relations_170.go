package analysis

import (
	"sort"
	"strings"

	"codea-harness-tools/internal/nav"
	"codea-harness-tools/internal/symbolid"
)

// ProjectLegacyCallChains170 deliberately projects only identities that the
// historical CallChain representation can express without losing authority.
// Dependency-workspace edges and overloaded method identities stay in the
// relation record and never expand legacy ReviewUnit selection.
func ProjectLegacyCallChains170(relations []nav.Relation170) []CallChain {
	type edge struct{ from, to nav.ReviewRef170 }
	edges := []edge{}
	for _, relation := range relations {
		if relation.Kind != "JAVA_CALL" || relation.Resolution != "EXACT" || len(relation.Targets) != 1 {
			continue
		}
		from, to := relation.From, relation.Targets[0]
		if from.Workspace != symbolid.CurrentWorkspace || to.Workspace != symbolid.CurrentWorkspace || from.Side != "CURRENT" || to.Side != "CURRENT" || from.Kind != "METHOD" || to.Kind != "METHOD" {
			continue
		}
		if len(from.ParameterTypes) != 0 || len(to.ParameterTypes) != 0 {
			continue
		}
		edges = append(edges, edge{from: from, to: to})
	}
	if len(edges) == 0 { return []CallChain{} }

	outgoing := map[string][]edge{}
	incoming := map[string]int{}
	refs := map[string]nav.ReviewRef170{}
	for _, e := range edges {
		fk, _ := nav.ReviewRefKey170(e.from)
		tk, _ := nav.ReviewRefKey170(e.to)
		outgoing[fk] = append(outgoing[fk], e)
		incoming[tk]++
		refs[fk] = e.from
		refs[tk] = e.to
	}
	starts := []string{}
	for key := range outgoing {
		if incoming[key] == 0 { starts = append(starts, key) }
	}
	sort.Strings(starts)
	seenEdge := map[string]bool{}
	chains := []CallChain{}
	appendChain := func(start string) {
		current := start
		refsInChain := []nav.ReviewRef170{refs[current]}
		for {
			outs := outgoing[current]
			if len(outs) != 1 { break }
			nextKey, _ := nav.ReviewRefKey170(outs[0].to)
			edgeKey := current + "\x00" + nextKey
			if seenEdge[edgeKey] || incoming[nextKey] > 1 { break }
			seenEdge[edgeKey] = true
			refsInChain = append(refsInChain, outs[0].to)
			current = nextKey
		}
		if len(refsInChain) < 2 { return }
		chain := make([]string, 0, len(refsInChain))
		chainRefs := make([]SymbolRef, 0, len(refsInChain))
		for _, ref := range refsInChain {
			symbol := legacySymbol170(ref)
			chain = append(chain, symbol)
			chainRefs = append(chainRefs, symbolid.Ref{Workspace: symbolid.CurrentWorkspace, Path: ref.Path, Symbol: symbol})
		}
		entry := chain[0]
		entryRef := chainRefs[0]
		chains = append(chains, CallChain{EntryPoint: entry, Chain: chain, EntryPointRef: &entryRef, ChainRefs: chainRefs})
	}
	for _, start := range starts { appendChain(start) }
	// Isolated unambiguous edges inside a cycle or fan-out are still safe as
	// two-node projections; never synthesize ordering across ambiguous edges.
	for _, e := range edges {
		fk, _ := nav.ReviewRefKey170(e.from)
		tk, _ := nav.ReviewRefKey170(e.to)
		key := fk + "\x00" + tk
		if seenEdge[key] { continue }
		fromSymbol, toSymbol := legacySymbol170(e.from), legacySymbol170(e.to)
		fromRef := SymbolRef{Workspace:symbolid.CurrentWorkspace, Path:e.from.Path, Symbol:fromSymbol}
		chains = append(chains, CallChain{EntryPoint:fromSymbol, Chain:[]string{fromSymbol,toSymbol}, EntryPointRef:&fromRef, ChainRefs:[]SymbolRef{{Workspace:symbolid.CurrentWorkspace,Path:e.from.Path,Symbol:fromSymbol},{Workspace:symbolid.CurrentWorkspace,Path:e.to.Path,Symbol:toSymbol}}})
		seenEdge[key] = true
	}
	sort.Slice(chains, func(i,j int) bool { return strings.Join(chains[i].Chain,"\x00") < strings.Join(chains[j].Chain,"\x00") })
	return chains
}

func legacySymbol170(ref nav.ReviewRef170) string {
	owner := strings.TrimSpace(ref.OwnerFQCN)
	if i := strings.LastIndex(owner, "."); i >= 0 { owner = owner[i+1:] }
	return owner + "." + strings.TrimSpace(ref.Name)
}
