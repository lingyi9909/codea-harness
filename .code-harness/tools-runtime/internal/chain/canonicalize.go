package chain

import (
	"encoding/json"
	"sort"
)

func Canonicalize(chains []Chain) []Chain {
	if len(chains) == 0 {
		return []Chain{}
	}
	working := append([]Chain(nil), chains...)
	sort.SliceStable(working, func(i, j int) bool {
		left := firstEntrySymbol(working[i])
		right := firstEntrySymbol(working[j])
		if left == right {
			return coreSignature(working[i]) < coreSignature(working[j])
		}
		return left < right
	})

	index := map[string]int{}
	var out []Chain
	for _, c := range working {
		key := coreSignature(c)
		if existing, ok := index[key]; ok {
			merged := &out[existing]
			merged.EntryPoints = mergeEntryPoints(merged.EntryPoints, c.EntryPoints)
			continue
		}
		c.EntryPoints = mergeEntryPoints(nil, c.EntryPoints)
		index[key] = len(out)
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ID == out[j].ID {
			return firstEntrySymbol(out[i]) < firstEntrySymbol(out[j])
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func coreSignature(c Chain) string {
	nodes := make([]Node, len(c.Nodes))
	for i, node := range c.Nodes {
		node.Workspace = effectiveWorkspace(node.Workspace)
		nodes[i] = node
	}
	facts := struct {
		Nodes      []Node     `json:"nodes"`
		Resources  []Resource `json:"resources"`
		Boundaries []Boundary `json:"boundaries"`
	}{Nodes: nodes, Resources: c.Resources, Boundaries: c.Boundaries}
	b, _ := json.Marshal(facts)
	return string(b)
}

func mergeEntryPoints(left, right []EntryPoint) []EntryPoint {
	seen := map[string]bool{}
	out := make([]EntryPoint, 0, len(left)+len(right))
	for _, entry := range append(append([]EntryPoint(nil), left...), right...) {
		entry.Workspace = effectiveWorkspace(entry.Workspace)
		key := entry.Workspace + "\x00" + entry.Symbol + "\x00" + entry.Path
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Workspace == out[j].Workspace {
			if out[i].Symbol == out[j].Symbol {
				return out[i].Path < out[j].Path
			}
			return out[i].Symbol < out[j].Symbol
		}
		return out[i].Workspace < out[j].Workspace
	})
	return out
}

func firstEntrySymbol(c Chain) string {
	if len(c.EntryPoints) == 0 {
		return ""
	}
	best := c.EntryPoints[0].Symbol
	for _, entry := range c.EntryPoints[1:] {
		if entry.Symbol < best {
			best = entry.Symbol
		}
	}
	return best
}
