package reviewrun

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"codea-harness-tools/internal/nav"
)

type scopedNavigation180 struct {
	Calls map[string][]nav.DirectMethodCall
	Impls map[string][]nav.ImplementationType
	Infos map[string]nav.SymbolInfo
}

// javaSourceRoot180 is the exact Maven Java-source root, not the first
// lexicographic top-level directory. A single project can contain dozens of
// modules. Legacy non-Maven trees use the nearest top-level directory.
func javaSourceRoot180(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	const marker = "src/main/java/"
	if at := strings.Index(p, marker); at >= 0 {
		return p[:at] + "src/main/java"
	}
	if at := strings.IndexByte(p, '/'); at > 0 {
		return p[:at]
	}
	return p
}

func discoverScopedNavigation180(ctx context.Context, n nav.Navigator, javaFiles []string, endpoints []nav.ControllerEndpointMatch) (map[string]scopedNavigation180, error) {
	seen := map[string]bool{}
	roots := []string{}
	for _, file := range javaFiles {
		root := javaSourceRoot180(file)
		if !seen[root] {
			roots = append(roots, root)
			seen[root] = true
		}
	}
	sort.Strings(roots)
	out := make(map[string]scopedNavigation180, len(roots))
	for _, root := range roots {
		// Avoid scanning standalone utility and test modules with no review
		// entrypoints. They cannot create authoritative Controller chains.
		selected := []nav.ControllerEndpointMatch{}
		for _, ep := range endpoints {
			if javaSourceRoot180(ep.Path) == root {
				selected = append(selected, ep)
			}
		}
		if len(selected) == 0 { continue }
		calls, err := n.FindDirectMethodCallsBatch180(ctx, root)
		if err != nil {
			return nil, fmt.Errorf("CALL_DISCOVERY_FAILED: scope=%s: %w", root, err)
		}
		receivers := []string{}
		for _, ep := range selected {
			for _, fact := range calls[ep.Symbol] {
				if fact.Resolved && fact.ReceiverType != "" {
					receivers = append(receivers, fact.ReceiverType)
				}
			}
		}
		impls, err := n.FindImplementationTypesBatch180(ctx, uniqueStrings180(receivers), root)
		if err != nil {
			return nil, fmt.Errorf("IMPLEMENTATION_DISCOVERY_FAILED: scope=%s: %w", root, err)
		}
		targets := []string{}
		infosToQuery := append([]string{}, receivers...)
		// Restrict downstream symbol analysis to this module.
		for _, ep := range selected {
			for _, fact := range calls[ep.Symbol] {
				if !fact.Resolved { continue }
				symbol := fact.TargetSymbol
				if xs := impls[fact.ReceiverType]; len(xs) == 1 {
					symbol = xs[0].Symbol + "." + fact.Method
				}
				for _, next := range calls[symbol] {
					if next.Resolved {
						targets = append(targets, next.TargetSymbol)
						infosToQuery = append(infosToQuery, next.ReceiverType)
					}
				}
			}
		}
		infosToQuery = append(infosToQuery, targets...)
		infos := map[string]nav.SymbolInfo{}
		if len(infosToQuery) > 0 {
			infos, err = n.GetSymbolInfosPartial180(ctx, uniqueStrings180(infosToQuery), root)
			if err != nil {
				return nil, fmt.Errorf("TARGET_DISCOVERY_FAILED: scope=%s: %w", root, err)
			}
		}
		out[root] = scopedNavigation180{Calls: calls, Impls: impls, Infos: infos}
	}

	// A Controller module can depend on a different Maven module containing
	// the ServiceImpl or Mapper. Resolve those missing downstream declarations
	// without merging identically named Controller entrypoints across modules.
	if err := recoverExternalNavigation180(ctx, n, roots, endpoints, out); err != nil {
		return nil, err
	}
	return out, nil
}

 
// recoverExternalNavigation180 follows unresolved Service interfaces into
// other Java source roots. All candidate implementations are retained:
// ambiguous implementations must never be turned into a unique service path.
// Controller methods always retain their own module-local call facts.
func recoverExternalNavigation180(ctx context.Context, n nav.Navigator, roots []string, endpoints []nav.ControllerEndpointMatch, out map[string]scopedNavigation180) error {
	missingOwners := map[string]bool{}
	for _, ep := range endpoints {
		facts, ok := out[javaSourceRoot180(ep.Path)]
		if !ok {
			continue
		}
		for _, call := range facts.Calls[ep.Symbol] {
			if call.Resolved && call.ReceiverType != "" && len(facts.Impls[call.ReceiverType]) == 0 {
				missingOwners[call.ReceiverType] = true
			}
		}
	}
	if len(missingOwners) == 0 {
		return nil
	}
	owners := make([]string, 0, len(missingOwners))
	for name := range missingOwners {
		owners = append(owners, name)
	}
	sort.Strings(owners)

	// Batch implementation discovery once per Maven Java-source root,
	// including dependency modules without any Controller.
	globalImpls := map[string][]nav.ImplementationType{}
	for _, sourceRoot := range roots {
		candidates, err := n.FindImplementationTypesBatch180(ctx, owners, sourceRoot)
		if err != nil {
			return fmt.Errorf("IMPLEMENTATION_DISCOVERY_FAILED: scope=%s: %w", sourceRoot, err)
		}
		for name, xs := range candidates {
			for _, candidate := range xs {
				duplicate := false
				for _, existing := range globalImpls[name] {
					if existing.Path == candidate.Path && existing.Symbol == candidate.Symbol {
						duplicate = true
						break
					}
				}
				if !duplicate {
					globalImpls[name] = append(globalImpls[name], candidate)
				}
			}
		}
	}
	for name := range globalImpls {
		sort.Slice(globalImpls[name], func(i, j int) bool {
			a, b := globalImpls[name][i], globalImpls[name][j]
			if a.Path != b.Path {
				return a.Path < b.Path
			}
			return a.Symbol < b.Symbol
		})
	}

	// A separate source root may define the concrete service method calls.
	// Collect only roots containing newly discovered implementations.
	callRoots := map[string]bool{}
	for _, ep := range endpoints {
		root := javaSourceRoot180(ep.Path)
		facts, ok := out[root]
		if !ok {
			continue
		}
		for _, first := range facts.Calls[ep.Symbol] {
			if !first.Resolved || first.ReceiverType == "" || len(facts.Impls[first.ReceiverType]) != 0 {
				continue
			}
			facts.Impls[first.ReceiverType] = globalImpls[first.ReceiverType]
			for _, impl := range globalImpls[first.ReceiverType] {
				callRoots[javaSourceRoot180(impl.Path)] = true
			}
		}
		out[root] = facts
	}
	externalCalls := map[string]map[string][]nav.DirectMethodCall{}
	for _, root := range roots {
		if !callRoots[root] {
			continue
		}
		calls, err := n.FindDirectMethodCallsBatch180(ctx, root)
		if err != nil {
			return fmt.Errorf("CALL_DISCOVERY_FAILED: scope=%s: %w", root, err)
		}
		externalCalls[root] = calls
	}

	// Gather declarations required by downstream edges. Look across source
	// roots, and never choose arbitrarily between duplicate symbol identities.
	neededInfos := map[string]bool{}
	for _, ep := range endpoints {
		root := javaSourceRoot180(ep.Path)
		facts, ok := out[root]
		if !ok {
			continue
		}
		for _, first := range facts.Calls[ep.Symbol] {
			if !first.Resolved {
				continue
			}
			if _, ok := facts.Infos[first.ReceiverType]; !ok {
				neededInfos[first.ReceiverType] = true
			}
			for _, impl := range facts.Impls[first.ReceiverType] {
				method := impl.Symbol + "." + first.Method
				if len(facts.Calls[method]) == 0 {
					facts.Calls[method] = append([]nav.DirectMethodCall(nil), externalCalls[javaSourceRoot180(impl.Path)][method]...)
				}
				for _, downstream := range facts.Calls[method] {
					if downstream.Resolved {
						if _, ok := facts.Infos[downstream.TargetSymbol]; !ok {
							neededInfos[downstream.TargetSymbol] = true
						}
					}
				}
			}
		}
		out[root] = facts
	}
	if len(neededInfos) == 0 {
		return nil
	}
	symbols := make([]string, 0, len(neededInfos))
	for symbol := range neededInfos {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)
	unique := map[string]nav.SymbolInfo{}
	ambiguous := map[string]bool{}
	for _, sourceRoot := range roots {
		found, err := n.GetSymbolInfosPartial180(ctx, symbols, sourceRoot)
		if err != nil {
			return fmt.Errorf("TARGET_DISCOVERY_FAILED: scope=%s: %w", sourceRoot, err)
		}
		for name, info := range found {
			if prior, exists := unique[name]; exists && prior.Path != info.Path {
				ambiguous[name] = true
			}
			if !ambiguous[name] {
				unique[name] = info
			}
		}
	}
	for _, ep := range endpoints {
		root := javaSourceRoot180(ep.Path)
		facts, ok := out[root]
		if !ok {
			continue
		}
		for symbol, info := range unique {
			if ambiguous[symbol] {
				continue
			}
			if _, exists := facts.Infos[symbol]; !exists {
				facts.Infos[symbol] = info
			}
		}
		out[root] = facts
	}
	return nil
}
