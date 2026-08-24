package reviewscope

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"codea-harness-tools/internal/chain"
)

const (
	ChainResolutionReady         = "READY"
	ChainResolutionStaleDecision = "STALE_REQUIRES_DECISION"
	ChainResolutionPartial       = "PARTIAL"
)

type ChainContext struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Source string `json:"source"` // ACCEPTED | DISCOVERED
	Status string `json:"status"` // VALID | TEMPORARY
}

type StaleChain struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ChainResolveOptions struct {
	RunID                  string `json:"runId"`
	AllowTemporaryForStale bool   `json:"allowTemporaryForStale,omitempty"`
}

type ChainResolution struct {
	Status     string         `json:"status"`
	Contexts   []ChainContext `json:"contexts"`
	Stale      []StaleChain   `json:"stale"`
	Unresolved []string       `json:"unresolved"`
}

var reviewChainRunIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// ResolveChainContexts prepares verified business-chain context for one Review scope.
// Accepted Project State is reused only after Runtime validation. Missing branches are
// lazily discovered as Run State. A stale accepted chain requires an explicit caller
// decision before temporary rediscovery is allowed.
func ResolveChainContexts(root string, selection Selection, changeAnalysisJSON []byte, opts ChainResolveOptions) (ChainResolution, error) {
	result := ChainResolution{
		Status:     ChainResolutionReady,
		Contexts:   []ChainContext{},
		Stale:      []StaleChain{},
		Unresolved: []string{},
	}
	if !reviewChainRunIDPattern.MatchString(strings.TrimSpace(opts.RunID)) {
		return result, fmt.Errorf("invalid review chain runId %q", opts.RunID)
	}
	var evidence chain.ChangeAnalysisEvidence
	if err := json.Unmarshal(changeAnalysisJSON, &evidence); err != nil {
		return result, fmt.Errorf("decode review chain ChangeAnalysis: %w", err)
	}

	required := requiredReviewCallChains(selection, evidence.CallChains)
	if len(required) == 0 {
		return result, nil
	}

	projectChains, err := loadProjectChains(root)
	if err != nil {
		return result, err
	}

	covered := make(map[string]bool, len(required))
	staleEntries := make(map[string]bool)
	contextSeen := make(map[string]bool)
	staleSeen := make(map[string]bool)

	for _, persisted := range projectChains {
		if persisted.Status != chain.StatusAccepted && persisted.Status != chain.StatusStale {
			continue
		}
		relevantEntries := relevantPersistedEntries(persisted, required)
		if len(relevantEntries) == 0 {
			continue
		}
		if persisted.Status == chain.StatusStale {
			for _, entry := range relevantEntries {
				staleEntries[entry] = true
			}
			if !staleSeen[persisted.ID] {
				result.Stale = append(result.Stale, StaleChain{ID: persisted.ID, Name: persisted.Name})
				staleSeen[persisted.ID] = true
			}
			continue
		}
		validation := chain.Validate(root, persisted, chain.EvidenceSnapshot(evidence))
		switch validation.Status {
		case chain.ValidationValid:
			for _, branch := range required {
				if acceptedMatchesBranch(persisted, branch) {
					covered[callChainKey(branch.EntryPoint, branch.Chain)] = true
				}
			}
			key := "ACCEPTED:" + persisted.ID
			if !contextSeen[key] {
				result.Contexts = append(result.Contexts, ChainContext{ID: persisted.ID, Name: persisted.Name, Source: "ACCEPTED", Status: "VALID"})
				contextSeen[key] = true
			}
		case chain.ValidationStale:
			for _, entry := range relevantEntries {
				staleEntries[entry] = true
			}
			if !staleSeen[persisted.ID] {
				result.Stale = append(result.Stale, StaleChain{ID: persisted.ID, Name: persisted.Name})
				staleSeen[persisted.ID] = true
			}
		case chain.ValidationInvalid:
			return result, fmt.Errorf("accepted review chain %q is invalid: %s", persisted.ID, strings.Join(validation.Errors, "; "))
		default:
			return result, fmt.Errorf("accepted review chain %q returned unknown validation status %q", persisted.ID, validation.Status)
		}
	}

	if len(result.Stale) > 0 && !opts.AllowTemporaryForStale {
		result.Status = ChainResolutionStaleDecision
		result.Contexts = filterAcceptedContextsForCovered(result.Contexts, projectChains, required, covered)
		sortChainResolution(&result)
		return result, nil
	}

	toDiscover := make([]chain.CallChainEvidence, 0)
	for _, branch := range required {
		key := callChainKey(branch.EntryPoint, branch.Chain)
		if covered[key] {
			continue
		}
		if staleEntries[branch.EntryPoint] && !opts.AllowTemporaryForStale {
			continue
		}
		toDiscover = append(toDiscover, branch)
	}

	for _, branch := range toDiscover {
		discovered, err := chain.Discover(root, chain.DiscoverInput{
			RunID:          opts.RunID,
			Target:         branch.EntryPoint,
			ChangeAnalysis: evidence,
		})
		if err != nil {
			return result, fmt.Errorf("discover temporary review chain for %q: %w", branch.EntryPoint, err)
		}
		if discovered.Status != chain.DiscoveryComplete {
			result.Status = ChainResolutionPartial
			result.Unresolved = append(result.Unresolved, discovered.Unresolved...)
			continue
		}
		matched := false
		for _, candidate := range discovered.Chains {
			if !acceptedMatchesBranch(candidate, branch) {
				continue
			}
			validation := chain.Validate(root, candidate, chain.EvidenceSnapshot(evidence))
			if validation.Status != chain.ValidationValid {
				return result, fmt.Errorf("temporary review chain %q failed validation: %s", candidate.ID, strings.Join(validation.Errors, "; "))
			}
			key := "DISCOVERED:" + candidate.ID
			if !contextSeen[key] {
				result.Contexts = append(result.Contexts, ChainContext{ID: candidate.ID, Name: candidate.Name, Source: "DISCOVERED", Status: "TEMPORARY"})
				contextSeen[key] = true
			}
			matched = true
		}
		if !matched {
			result.Status = ChainResolutionPartial
			result.Unresolved = append(result.Unresolved, "CHAIN_NOT_DISCOVERED: "+branch.EntryPoint)
		}
	}

	result.Unresolved = uniqueSorted(result.Unresolved)
	if len(result.Unresolved) > 0 {
		result.Status = ChainResolutionPartial
	}
	sortChainResolution(&result)
	return result, nil
}

func requiredReviewCallChains(selection Selection, all []chain.CallChainEvidence) []chain.CallChainEvidence {
	if selection.Mode == "FULL" {
		out := append([]chain.CallChainEvidence(nil), all...)
		sort.Slice(out, func(i, j int) bool { return callChainKey(out[i].EntryPoint, out[i].Chain) < callChainKey(out[j].EntryPoint, out[j].Chain) })
		return out
	}
	out := make([]chain.CallChainEvidence, 0, len(selection.SelectedCallChains))
	for _, selected := range selection.SelectedCallChains {
		out = append(out, chain.CallChainEvidence{EntryPoint: selected.EntryPoint, Chain: append([]string(nil), selected.Chain...)})
	}
	return out
}

func loadProjectChains(root string) ([]chain.Chain, error) {
	dir := filepath.Join(filepath.Clean(root), ".code-harness", "chains")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []chain.Chain{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read project chains: %w", err)
	}
	out := make([]chain.Chain, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".yaml") {
			continue
		}
		loaded, err := chain.Load(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("load project chain %q: %w", entry.Name(), err)
		}
		out = append(out, loaded)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func relevantPersistedEntries(c chain.Chain, required []chain.CallChainEvidence) []string {
	requiredEntries := make(map[string]bool, len(required))
	for _, branch := range required {
		requiredEntries[strings.TrimSpace(branch.EntryPoint)] = true
	}
	var out []string
	for _, entry := range c.EntryPoints {
		symbol := strings.TrimSpace(entry.Symbol)
		if requiredEntries[symbol] {
			out = append(out, symbol)
		}
	}
	return uniqueSorted(out)
}

func acceptedMatchesBranch(c chain.Chain, branch chain.CallChainEvidence) bool {
	entry := strings.TrimSpace(branch.EntryPoint)
	if entry == "" || len(branch.Chain) == 0 || strings.TrimSpace(branch.Chain[0]) != entry {
		return false
	}
	entryFound := false
	for _, candidate := range c.EntryPoints {
		if strings.TrimSpace(candidate.Symbol) == entry {
			entryFound = true
			break
		}
	}
	if !entryFound || len(c.Nodes) != len(branch.Chain)-1 {
		return false
	}
	for i, node := range c.Nodes {
		if strings.TrimSpace(node.Symbol) != strings.TrimSpace(branch.Chain[i+1]) {
			return false
		}
	}
	return true
}

func filterAcceptedContextsForCovered(contexts []ChainContext, project []chain.Chain, required []chain.CallChainEvidence, covered map[string]bool) []ChainContext {
	validIDs := make(map[string]bool)
	for _, c := range project {
		for _, branch := range required {
			if covered[callChainKey(branch.EntryPoint, branch.Chain)] && acceptedMatchesBranch(c, branch) {
				validIDs[c.ID] = true
			}
		}
	}
	out := make([]ChainContext, 0, len(contexts))
	for _, ctx := range contexts {
		if ctx.Source == "ACCEPTED" && validIDs[ctx.ID] {
			out = append(out, ctx)
		}
	}
	return out
}

func callChainKey(entry string, nodes []string) string {
	return strings.TrimSpace(entry) + "\x00" + strings.Join(trimStrings(nodes), "\x00")
}

func trimStrings(in []string) []string {
	out := make([]string, len(in))
	for i, value := range in {
		out[i] = strings.TrimSpace(value)
	}
	return out
}

func sortChainResolution(result *ChainResolution) {
	sort.Slice(result.Contexts, func(i, j int) bool {
		if result.Contexts[i].Source != result.Contexts[j].Source {
			return result.Contexts[i].Source < result.Contexts[j].Source
		}
		return result.Contexts[i].ID < result.Contexts[j].ID
	})
	sort.Slice(result.Stale, func(i, j int) bool { return result.Stale[i].ID < result.Stale[j].ID })
	result.Unresolved = uniqueSorted(result.Unresolved)
}
