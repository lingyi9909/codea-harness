package reviewscope

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

// BuildTargetedSelection derives the exact current-workspace ReviewScope for a
// Runtime-selected set of verified call chains. When there is no explicit user
// target, Runtime chooses a deterministic verified anchor for the selected set.
func BuildTargetedSelection(selected []CallChain, changeAnalysisJSON []byte) (Selection, error) {
	return buildTargetedSelection153(selected, changeAnalysisJSON, nil)
}

// BuildTargetedSelectionForTarget preserves an explicit Runtime-certified
// review target. The selected upstream Chain set may vary, but the target is
// immutable and must still be represented by every resulting verified scope.
func BuildTargetedSelectionForTarget(selected []CallChain, changeAnalysisJSON []byte, target Target) (Selection, error) {
	target.Symbol = strings.TrimSpace(target.Symbol)
	target.Kind = strings.ToUpper(strings.TrimSpace(target.Kind))
	if target.Symbol == "" || (target.Kind != "CLASS" && target.Kind != "METHOD") {
		return Selection{}, errors.New("explicit TARGETED review requires a valid target")
	}
	return buildTargetedSelection153(selected, changeAnalysisJSON, &target)
}

func buildTargetedSelection153(selected []CallChain, changeAnalysisJSON []byte, explicit *Target) (Selection, error) {
	if len(selected) == 0 {
		return Selection{}, errors.New("TARGETED review requires at least one selected call chain")
	}
	analysis, err := parseChangeAnalysis(changeAnalysisJSON)
	if err != nil {
		return Selection{}, err
	}
	for _, candidate := range selected {
		if !containsChain(analysis.CallChains, candidate) {
			return Selection{}, errors.New("selected call chain is not present in validated ChangeAnalysis")
		}
	}
	changedRoles, err := validateChangedAndReviewedResourceRoles(analysis.ChangedFiles, analysis.ReviewCoverage.ReviewedFiles)
	if err != nil {
		return Selection{}, err
	}
	evidence, err := buildNavigationEvidence(analysis.SymbolLocations)
	if err != nil {
		return Selection{}, err
	}
	var target Target
	if explicit != nil {
		target = *explicit
	} else {
		target, err = runtimeSelectionAnchor153(selected, evidence)
		if err != nil {
			return Selection{}, err
		}
	}
	_, requiredPaths, err := exactScopePaths(target, selected, evidence, analysis.ResourceRelations, changedRoles)
	if err != nil {
		return Selection{}, err
	}
	scopedFiles := make([]string, 0, len(requiredPaths))
	for value := range requiredPaths {
		scopedFiles = append(scopedFiles, value)
	}
	sort.Strings(scopedFiles)
	proposed := Selection{Mode: "TARGETED", Target: &target, SelectedCallChains: append([]CallChain(nil), selected...), ScopedFiles: scopedFiles}
	encoded, err := json.Marshal(proposed)
	if err != nil {
		return Selection{}, err
	}
	return Verify(encoded, changeAnalysisJSON)
}

func BuildFullSelection(changeAnalysisJSON []byte) (Selection, error) {
	return Verify([]byte(`{"mode":"FULL","selectedCallChains":[],"scopedFiles":[]}`), changeAnalysisJSON)
}

func runtimeSelectionAnchor153(selected []CallChain, evidence navigationEvidence) (Target, error) {
	// Prefer one exact current-workspace non-Controller symbol shared by every
	// selected Chain for plain review when there was no explicit target.
	common := map[string]int{}
	for index, candidate := range selected {
		seen := map[string]bool{}
		for _, symbol := range candidate.Chain {
			symbol = strings.TrimSpace(symbol)
			loc, ok := evidence.bySymbol[symbol]
			if !ok || loc.Workspace != currentWorkspace || loc.Role == "Controller" || seen[symbol] {
				continue
			}
			seen[symbol] = true
			if index == 0 {
				common[symbol] = 1
			} else if common[symbol] == index {
				common[symbol] = index + 1
			}
		}
	}
	var shared []string
	for symbol, count := range common {
		if count == len(selected) {
			shared = append(shared, symbol)
		}
	}
	sort.Strings(shared)
	if len(shared) > 0 {
		return Target{Symbol: shared[0], Kind: "METHOD"}, nil
	}

	for _, candidate := range selected {
		for _, symbol := range candidate.Chain {
			symbol = strings.TrimSpace(symbol)
			loc, ok := evidence.bySymbol[symbol]
			if ok && loc.Workspace == currentWorkspace && loc.Role != "Controller" {
				return Target{Symbol: symbol, Kind: "METHOD"}, nil
			}
		}
	}
	entry := strings.TrimSpace(selected[0].EntryPoint)
	if entry == "" {
		return Target{}, errors.New("selected call chain has no entrypoint")
	}
	return Target{Symbol: entry, Kind: "METHOD"}, nil
}
