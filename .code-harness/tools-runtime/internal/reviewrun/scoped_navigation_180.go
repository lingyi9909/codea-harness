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
	return out, nil
}
