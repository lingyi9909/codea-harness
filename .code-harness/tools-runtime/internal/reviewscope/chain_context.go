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

const ChainResolutionStrategyAutoTemporary = "AUTO_TEMPORARY"

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
	Strategy               string `json:"-"`
	AllowTemporaryForStale bool   `json:"allowTemporaryForStale,omitempty"`
	CertifyDiscovered      func(chain.Chain) error `json:"-"`
}

type ChainResolution struct {
	Status     string         `json:"status"`
	Contexts   []ChainContext `json:"contexts"`
	Stale      []StaleChain   `json:"stale"`
	Unresolved []string       `json:"unresolved"`
	Notices    []string       `json:"notices,omitempty"`
}

var reviewChainRunIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// ResolveChainContexts prepares verified business-chain context for one Review scope.
// The default behavior remains compatible with the pre-1.7 explicit stale decision.
// Normal 1.7 Review passes AUTO_TEMPORARY from the Runtime product entrypoint: saved
// Project State is then presentation/history only, current certified analysis owns
// code facts, and stale/corrupt Project State is never written by this resolver.
func ResolveChainContexts(root string, selection Selection, changeAnalysisJSON []byte, opts ChainResolveOptions) (ChainResolution, error) {
	result := ChainResolution{
		Status:     ChainResolutionReady,
		Contexts:   []ChainContext{},
		Stale:      []StaleChain{},
		Unresolved: []string{},
		Notices:    []string{},
	}
	if !reviewChainRunIDPattern.MatchString(strings.TrimSpace(opts.RunID)) {
		return result, fmt.Errorf("invalid review chain runId %q", opts.RunID)
	}
	var evidence chain.ChangeAnalysisEvidence
	if err := json.Unmarshal(changeAnalysisJSON, &evidence); err != nil {
		return result, fmt.Errorf("decode review chain ChangeAnalysis: %w", err)
	}

	autoTemporary := opts.Strategy == ChainResolutionStrategyAutoTemporary
	if opts.Strategy != "" && !autoTemporary {
		return result, fmt.Errorf("unsupported review chain resolution strategy %q", opts.Strategy)
	}
	allowTemporaryForStale := opts.AllowTemporaryForStale || autoTemporary
	required := requiredReviewCallChains(selection, evidence.CallChains)
	if len(required) == 0 && !autoTemporary {
		return result, nil
	}

	projectChains, loadNotices, err := loadProjectChainsForReview(root, autoTemporary)
	if err != nil {
		return result, err
	}
	result.Notices = append(result.Notices, loadNotices...)
	removedEntries := removedPersistedEntries170(projectChains, evidence)
	for entry := range removedEntries {
		result.Notices = append(result.Notices, "CHAIN_ENTRYPOINT_REMOVED: "+entry)
	}
	if len(required) == 0 {
		sortChainResolution(&result)
		return result, nil
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
		filteredEntries := relevantEntries[:0]
		for _, entry := range relevantEntries {
			if !removedEntries[entry] {
				filteredEntries = append(filteredEntries, entry)
			}
		}
		relevantEntries = filteredEntries
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
			if autoTemporary {
				result.Notices = append(result.Notices, "PERSISTED_CHAIN_INVALID: "+persisted.ID)
				continue
			}
			return result, fmt.Errorf("accepted review chain %q is invalid: %s", persisted.ID, strings.Join(validation.Errors, "; "))
		default:
			return result, fmt.Errorf("accepted review chain %q returned unknown validation status %q", persisted.ID, validation.Status)
		}
	}

	if len(result.Stale) > 0 && !allowTemporaryForStale {
		result.Status = ChainResolutionStaleDecision
		result.Contexts = filterAcceptedContextsForCovered(result.Contexts, projectChains, required, covered)
		sortChainResolution(&result)
		return result, nil
	}

	toDiscover := make([]chain.CallChainEvidence, 0)
	for _, branch := range required {
		key := callChainKey(branch.EntryPoint, branch.Chain)
		if covered[key] || removedEntries[branch.EntryPoint] {
			continue
		}
		if staleEntries[branch.EntryPoint] && !allowTemporaryForStale {
			continue
		}
		toDiscover = append(toDiscover, branch)
	}
	if len(toDiscover) > 0 && opts.CertifyDiscovered == nil {
		return result, errors.New("review chain lazy discovery requires Runtime candidate certification")
	}

	for _, branch := range toDiscover {
		branchEvidence := evidence
		if autoTemporary {
			branchEvidence = evidenceForRequiredBranch170(evidence, branch)
		}
		discovered, err := chain.Discover(root, chain.DiscoverInput{
			RunID:          opts.RunID,
			Target:         branch.EntryPoint,
			ChangeAnalysis: branchEvidence,
		})
		if err != nil {
			return result, fmt.Errorf("discover temporary review chain for %q: %w", branch.EntryPoint, err)
		}

		matched := false
		for _, candidate := range discovered.Chains {
			if !acceptedMatchesBranch(candidate, branch) {
				continue
			}
			if err := opts.CertifyDiscovered(candidate); err != nil {
				return result, fmt.Errorf("certify temporary review chain %q: %w", candidate.ID, err)
			}
			validation := chain.Validate(root, candidate, chain.EvidenceSnapshot(branchEvidence))
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
		if discovered.Status != chain.DiscoveryComplete {
			result.Status = ChainResolutionPartial
			result.Unresolved = append(result.Unresolved, discovered.Unresolved...)
		}
		if !matched {
			result.Status = ChainResolutionPartial
			if len(discovered.Unresolved) == 0 {
				result.Unresolved = append(result.Unresolved, "CHAIN_NOT_DISCOVERED: "+branch.EntryPoint)
			}
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
	loaded, _, err := loadProjectChainsForReview(root, false)
	return loaded, err
}

func loadProjectChainsForReview(root string, tolerateInvalid bool) ([]chain.Chain, []string, error) {
	dir := filepath.Join(filepath.Clean(root), ".code-harness", "chains")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []chain.Chain{}, []string{}, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read project chains: %w", err)
	}
	out := make([]chain.Chain, 0, len(entries))
	notices := []string{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".yaml") {
			continue
		}
		loaded, err := chain.Load(filepath.Join(dir, entry.Name()))
		if err != nil {
			if tolerateInvalid {
				notices = append(notices, "PERSISTED_CHAIN_INVALID: "+entry.Name())
				continue
			}
			return nil, nil, fmt.Errorf("load project chain %q: %w", entry.Name(), err)
		}
		out = append(out, loaded)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, uniqueSorted(notices), nil
}

func removedPersistedEntries170(project []chain.Chain, evidence chain.ChangeAnalysisEvidence) map[string]bool {
	changed := map[string]bool{}
	for _, file := range evidence.ChangedFiles {
		changed[strings.ToLower(filepath.ToSlash(filepath.Clean(file.Path)))] = true
	}
	currentSymbols := map[string]bool{}
	for _, location := range evidence.SymbolLocations {
		workspace := strings.TrimSpace(location.Workspace)
		if workspace == "" || workspace == chain.CurrentWorkspace {
			currentSymbols[strings.TrimSpace(location.Symbol)] = true
		}
	}
	removed := map[string]bool{}
	for _, persisted := range project {
		if persisted.Status != chain.StatusAccepted && persisted.Status != chain.StatusStale {
			continue
		}
		for _, entry := range persisted.EntryPoints {
			pathKey := strings.ToLower(filepath.ToSlash(filepath.Clean(entry.Path)))
			symbol := strings.TrimSpace(entry.Symbol)
			if changed[pathKey] && symbol != "" && !currentSymbols[symbol] {
				removed[symbol] = true
			}
		}
	}
	return removed
}

func evidenceForRequiredBranch170(in chain.ChangeAnalysisEvidence, branch chain.CallChainEvidence) chain.ChangeAnalysisEvidence {
	out := in
	out.CallChains = []chain.CallChainEvidence{{EntryPoint: branch.EntryPoint, Chain: append([]string(nil), branch.Chain...)}}
	out.AffectedControllers = nil
	for _, affected := range in.AffectedControllers {
		found := false
		for _, endpoint := range affected.Endpoints {
			if strings.TrimSpace(endpoint) == strings.TrimSpace(branch.EntryPoint) {
				found = true
				break
			}
		}
		if !found {
			continue
		}
		copyAffected := affected
		copyAffected.Endpoints = []string{branch.EntryPoint}
		out.AffectedControllers = append(out.AffectedControllers, copyAffected)
	}
	return out
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
	result.Notices = uniqueSorted(result.Notices)
}
