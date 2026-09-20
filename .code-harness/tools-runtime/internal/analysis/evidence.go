package analysis

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codea-harness-tools/internal/projectpath"
	"codea-harness-tools/internal/symbolid"
	"codea-harness-tools/internal/workspace"
)

func validateEvidence153(a ChangeAnalysis, inventory EntrypointInventory) error {
	return validateEvidenceAtRoot153(".", a, inventory)
}

func validateEvidenceAtRoot153(root string, a ChangeAnalysis, inventory EntrypointInventory) error {
	type fact struct {
		workspace string
		path      string
		role      string
	}
	facts := map[string]fact{}
	refsBySymbol := map[string][]symbolid.Ref{}
	workspaceIDs := map[string]struct{}{}

	for _, loc := range a.SymbolLocations {
		ref, ok := symbolid.FromLocation(loc.Workspace, loc.Path, loc.Symbol)
		if !ok {
			return fmt.Errorf("SYMBOL_LOCATION_INVALID: symbol=%q path=%q", loc.Symbol, loc.Path)
		}
		for _, previousRef := range refsBySymbol[ref.Symbol] {
			if previousRef.Workspace == ref.Workspace && previousRef.Path != ref.Path && moduleRoot153(previousRef.Path) == moduleRoot153(ref.Path) {
				return fmt.Errorf("SYMBOL_LOCATION_CONFLICT: %s has %s/%s and %s/%s", ref.Symbol, previousRef.Workspace, previousRef.Path, ref.Workspace, ref.Path)
			}
		}
		key, _ := symbolid.Key(ref)
		candidate := fact{workspace: ref.Workspace, path: ref.Path, role: strings.TrimSpace(loc.Role)}
		if previous, exists := facts[key]; exists && previous != candidate {
			return fmt.Errorf("SYMBOL_LOCATION_CONFLICT: %s/%s/%s has roles %s and %s", ref.Workspace, ref.Path, ref.Symbol, previous.role, candidate.role)
		}
		if _, exists := facts[key]; !exists {
			refsBySymbol[ref.Symbol] = append(refsBySymbol[ref.Symbol], ref)
		}
		facts[key] = candidate
		if ref.Workspace != symbolid.CurrentWorkspace {
			workspaceIDs[ref.Workspace] = struct{}{}
		}
	}

	resolveFact := func(symbol string, exact *SymbolRef) (fact, symbolid.Ref, error) {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			return fact{}, symbolid.Ref{}, fmt.Errorf("empty symbol")
		}
		if exact != nil {
			ref, ok := symbolid.Normalize(*exact)
			if !ok || ref.Symbol != symbol {
				return fact{}, symbolid.Ref{}, fmt.Errorf("exact ref does not match symbol %q", symbol)
			}
			key, _ := symbolid.Key(ref)
			value, exists := facts[key]
			if !exists {
				return fact{}, symbolid.Ref{}, fmt.Errorf("exact ref %s/%s/%s has no symbolLocation", ref.Workspace, ref.Path, ref.Symbol)
			}
			return value, ref, nil
		}
		refs := refsBySymbol[symbol]
		if len(refs) == 0 {
			return fact{}, symbolid.Ref{}, fmt.Errorf("symbol %q has no symbolLocation", symbol)
		}
		if len(refs) != 1 {
			return fact{}, symbolid.Ref{}, fmt.Errorf("symbol %q has %d path-qualified locations", symbol, len(refs))
		}
		key, _ := symbolid.Key(refs[0])
		return facts[key], refs[0], nil
	}

	ids := make([]string, 0, len(workspaceIDs))
	for id := range workspaceIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := verifyWorkspaceEvidence153(root, id); err != nil {
			return err
		}
	}

	for _, f := range a.ChangedFiles {
		if _, ok := safeEvidencePath153(f.Path); !ok {
			return fmt.Errorf("CHANGE_SET_PATH_INVALID: %q", f.Path)
		}
	}
	for _, f := range a.ReviewCoverage.ReviewedFiles {
		if _, ok := safeEvidencePath153(f.Path); !ok {
			return fmt.Errorf("REVIEWED_PATH_INVALID: %q", f.Path)
		}
	}

	inventoryBySymbol := map[string][]ExpectedEntrypoint{}
	for _, expected := range inventory.ExpectedEntrypoints {
		symbol := strings.TrimSpace(expected.Symbol)
		inventoryBySymbol[symbol] = append(inventoryBySymbol[symbol], expected)
	}

	confirmed := map[string]bool{}
	for _, c := range a.CallChains {
		entry := strings.TrimSpace(c.EntryPoint)
		if len(c.ChainRefs) != 0 && len(c.ChainRefs) != len(c.Chain) {
			return fmt.Errorf("CALL_CHAIN_EVIDENCE_MISSING: %s chainRefs length=%d chain length=%d", entry, len(c.ChainRefs), len(c.Chain))
		}
		if entry != "" {
			entryFact, entryRef, err := resolveFact(entry, c.EntryPointRef)
			if err != nil {
				if c.EntryPointRef == nil && len(refsBySymbol[entry]) == 0 {
					return fmt.Errorf("ENTRYPOINT_EVIDENCE_MISSING: %s requires current Controller symbolLocation", entry)
				}
				return fmt.Errorf("ENTRYPOINT_EVIDENCE_AMBIGUOUS: %s: %v", entry, err)
			}
			if entryFact.workspace != symbolid.CurrentWorkspace || entryFact.role != "Controller" {
				return fmt.Errorf("ENTRYPOINT_EVIDENCE_MISSING: %s requires current Controller symbolLocation", entry)
			}
			entryKey, _ := symbolid.Key(entryRef)
			confirmed[entryKey] = true

			expectedCandidates := inventoryBySymbol[entry]
			if len(expectedCandidates) > 0 {
				matched := false
				for _, expected := range expectedCandidates {
					expectedPath, valid := safeEvidencePath153(expected.Path)
					if valid && entryRef.Workspace == symbolid.CurrentWorkspace && entryRef.Path == expectedPath {
						matched = true
						break
					}
				}
				if !matched {
					return fmt.Errorf("ENTRYPOINT_EVIDENCE_MISSING: %s exact path %q is not an expected EntryPoint path", entry, entryFact.path)
				}
				controller := entrypointController153(entry)
				if controller == "" || !affectedControllerContainsEntrypoint153(a.AffectedControllers, controller, entry) {
					return fmt.Errorf("ENTRYPOINT_ANALYSIS_INCONSISTENT: %s is confirmed by callChain but missing exact affectedControllers controller/endpoint", entry)
				}
			}
		}
		for i, rawNode := range c.Chain {
			node := strings.TrimSpace(rawNode)
			if node == "" {
				return fmt.Errorf("CALL_CHAIN_EVIDENCE_MISSING: empty node in %q", entry)
			}
			var exact *SymbolRef
			if len(c.ChainRefs) > 0 {
				exact = &c.ChainRefs[i]
			}
			if _, _, err := resolveFact(node, exact); err != nil {
				if exact == nil && len(refsBySymbol[node]) == 0 {
					return fmt.Errorf("CALL_CHAIN_EVIDENCE_MISSING: %s", node)
				}
				return fmt.Errorf("CALL_CHAIN_EVIDENCE_AMBIGUOUS: %s: %v", node, err)
			}
		}
	}

	unresolvedReason := map[string]string{}
	for _, u := range a.ReviewCoverage.UnresolvedSymbols {
		reason := strings.TrimSpace(u.Reason)
		if strings.TrimSpace(u.Symbol) != "" {
			unresolvedReason[strings.TrimSpace(u.Symbol)] = reason
		}
		if strings.TrimSpace(u.From) != "" {
			unresolvedReason[strings.TrimSpace(u.From)] = reason
		}
	}
	for _, expected := range inventory.ExpectedEntrypoints {
		ref, ok := symbolid.FromLocation(symbolid.CurrentWorkspace, expected.Path, expected.Symbol)
		if !ok {
			return fmt.Errorf("ENTRYPOINT_EVIDENCE_MISSING: invalid expected EntryPoint identity %q/%q", expected.Symbol, expected.Path)
		}
		key, _ := symbolid.Key(ref)
		if expected.Disposition == DispositionRemoved || confirmed[key] {
			continue
		}
		if len(inventoryBySymbol[expected.Symbol]) == 1 {
			if reason, exists := unresolvedReason[expected.Symbol]; exists && reason == "" {
				return fmt.Errorf("UNRESOLVED_LIMITATION_REQUIRED: %s", expected.Symbol)
			}
		}
	}

	for _, relation := range a.ResourceRelations {
		p, ok := safeEvidencePath153(relation.Path)
		if !ok || !validResourceRelation153(p, relation) {
			return fmt.Errorf("RESOURCE_RELATION_INVALID: path=%q role=%q source=%q", relation.Path, relation.Role, relation.Source)
		}
		fromSymbol := strings.TrimSpace(relation.FromSymbol)
		if strings.TrimSpace(relation.Evidence) == "" || fromSymbol == "" {
			return fmt.Errorf("RESOURCE_RELATION_INVALID: missing evidence/fromSymbol for %q", relation.Path)
		}
		refs := refsBySymbol[fromSymbol]
		if len(refs) != 1 {
			return fmt.Errorf("RESOURCE_RELATION_INVALID: fromSymbol %q is not uniquely path-qualified", relation.FromSymbol)
		}
		key, _ := symbolid.Key(refs[0])
		from := facts[key]
		if from.workspace != symbolid.CurrentWorkspace {
			return fmt.Errorf("RESOURCE_RELATION_INVALID: fromSymbol %q lacks current-workspace symbol evidence", relation.FromSymbol)
		}
	}
	return nil
}

func moduleRoot153(value string) string {
	p, ok := safeEvidencePath153(value)
	if !ok {
		return ""
	}
	if strings.HasPrefix(p, "src/") {
		return ""
	}
	if i := strings.Index(p, "/src/"); i >= 0 {
		return p[:i]
	}
	return p
}

func verifyWorkspaceEvidence153(root, id string) error {
	configPath := filepath.Join(root, ".code-harness", "harness.yaml")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("WORKSPACE_DEPENDENCY_NOT_CONFIGURED: %s: %w", id, err)
	}
	deps, err := workspace.ValidateConfigYAML(root, configData)
	if err != nil {
		return err
	}
	var selected *workspace.Dependency
	for i := range deps {
		if deps[i].ID == id {
			selected = &deps[i]
			break
		}
	}
	if selected == nil {
		return fmt.Errorf("WORKSPACE_DEPENDENCY_NOT_CONFIGURED: %s", id)
	}
	results := workspace.VerifyDirectMavenDependencies(root, []workspace.Dependency{*selected})
	if len(results) != 1 {
		return fmt.Errorf("WORKSPACE_DEPENDENCY_COORDINATE_MISMATCH: %s", id)
	}
	result := results[0]
	if result.Status != workspace.StatusVerified {
		code := strings.TrimSpace(result.Code)
		if code == "" {
			code = workspace.CodeCoordinateMismatch
		}
		return fmt.Errorf("%s: %s", code, id)
	}
	return nil
}

func entrypointController153(symbol string) string {
	symbol = strings.TrimSpace(symbol)
	i := strings.LastIndex(symbol, ".")
	if i <= 0 {
		return ""
	}
	return symbol[:i]
}

func affectedControllerContainsEntrypoint153(controllers []AffectedController, controller, endpoint string) bool {
	for _, affected := range controllers {
		if strings.TrimSpace(affected.Controller) != controller {
			continue
		}
		for _, candidate := range affected.Endpoints {
			if strings.TrimSpace(candidate) == endpoint {
				return true
			}
		}
	}
	return false
}

func normalizeWorkspace153(workspaceID string) string {
	return symbolid.NormalizeWorkspace(workspaceID)
}

func safeEvidencePath153(value string) (string, bool) {
	return projectpath.Normalize(value)
}

func validResourceRelation153(p string, relation ResourceRelation) bool {
	switch strings.TrimSpace(relation.Role) {
	case "MapperXml":
		return projectpath.IsMapperXML(p) && relation.Source == "MAPPER_STATEMENT"
	case "YamlConfig":
		return projectpath.IsYAMLConfig(p) && relation.Source == "CONFIG_REFERENCE"
	default:
		return false
	}
}
