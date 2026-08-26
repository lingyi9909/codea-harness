package analysis

import (
	"fmt"
	"path"
	"strings"
)

func validateEvidence153(a ChangeAnalysis, inventory EntrypointInventory) error {
	type fact struct {
		workspace string
		path      string
		role      string
	}
	facts := map[string]fact{}
	dependencyPaths := map[string]string{}

	for _, loc := range a.SymbolLocations {
		symbol := strings.TrimSpace(loc.Symbol)
		p, ok := safeEvidencePath153(loc.Path)
		if symbol == "" || !ok {
			return fmt.Errorf("SYMBOL_LOCATION_INVALID: symbol=%q path=%q", loc.Symbol, loc.Path)
		}
		workspace := normalizeWorkspace153(loc.Workspace)
		role := strings.TrimSpace(loc.Role)
		candidate := fact{workspace: workspace, path: p, role: role}
		if previous, exists := facts[symbol]; exists && previous != candidate {
			return fmt.Errorf("SYMBOL_LOCATION_CONFLICT: %s has %s/%s/%s and %s/%s/%s", symbol, previous.workspace, previous.path, previous.role, candidate.workspace, candidate.path, candidate.role)
		}
		facts[symbol] = candidate
		if workspace != "current" {
			dependencyPaths[p] = workspace
		}
	}

	for _, f := range a.ChangedFiles {
		p, ok := safeEvidencePath153(f.Path)
		if !ok {
			return fmt.Errorf("CHANGE_SET_PATH_INVALID: %q", f.Path)
		}
		if workspace := dependencyPaths[p]; workspace != "" {
			return fmt.Errorf("WORKSPACE_DEPENDENCY_SCOPE_VIOLATION: changed file %q belongs to workspace %q", p, workspace)
		}
	}
	for _, f := range a.ReviewCoverage.ReviewedFiles {
		p, ok := safeEvidencePath153(f.Path)
		if !ok {
			return fmt.Errorf("REVIEWED_PATH_INVALID: %q", f.Path)
		}
		if workspace := dependencyPaths[p]; workspace != "" {
			return fmt.Errorf("WORKSPACE_DEPENDENCY_SCOPE_VIOLATION: reviewed file %q belongs to workspace %q", p, workspace)
		}
	}

	inventoryBySymbol := map[string]ExpectedEntrypoint{}
	for _, expected := range inventory.ExpectedEntrypoints {
		inventoryBySymbol[expected.Symbol] = expected
	}

	confirmed := map[string]bool{}
	for _, c := range a.CallChains {
		entry := strings.TrimSpace(c.EntryPoint)
		if entry != "" {
			confirmed[entry] = true
			entryFact, ok := facts[entry]
			if !ok || entryFact.workspace != "current" || entryFact.role != "Controller" {
				return fmt.Errorf("ENTRYPOINT_EVIDENCE_MISSING: %s requires current Controller symbolLocation", entry)
			}
			if expected, exists := inventoryBySymbol[entry]; exists {
				expectedPath, valid := safeEvidencePath153(expected.Path)
				if !valid || entryFact.path != expectedPath {
					return fmt.Errorf("ENTRYPOINT_EVIDENCE_MISSING: %s expected path %q got %q", entry, expected.Path, entryFact.path)
				}
			}
		}
		for _, node := range c.Chain {
			node = strings.TrimSpace(node)
			if node == "" {
				return fmt.Errorf("CALL_CHAIN_EVIDENCE_MISSING: empty node in %q", entry)
			}
			if _, ok := facts[node]; !ok {
				return fmt.Errorf("CALL_CHAIN_EVIDENCE_MISSING: %s", node)
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
		if expected.Disposition == DispositionRemoved || confirmed[expected.Symbol] {
			continue
		}
		if reason, exists := unresolvedReason[expected.Symbol]; exists && reason == "" {
			return fmt.Errorf("UNRESOLVED_LIMITATION_REQUIRED: %s", expected.Symbol)
		}
	}

	for _, relation := range a.ResourceRelations {
		p, ok := safeEvidencePath153(relation.Path)
		if !ok || !validResourceRelation153(p, relation) {
			return fmt.Errorf("RESOURCE_RELATION_INVALID: path=%q role=%q source=%q", relation.Path, relation.Role, relation.Source)
		}
		if strings.TrimSpace(relation.Evidence) == "" || strings.TrimSpace(relation.FromSymbol) == "" {
			return fmt.Errorf("RESOURCE_RELATION_INVALID: missing evidence/fromSymbol for %q", relation.Path)
		}
		from, exists := facts[strings.TrimSpace(relation.FromSymbol)]
		if !exists || from.workspace != "current" {
			return fmt.Errorf("RESOURCE_RELATION_INVALID: fromSymbol %q lacks current-workspace symbol evidence", relation.FromSymbol)
		}
	}
	return nil
}

func normalizeWorkspace153(workspace string) string {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		return "current"
	}
	return workspace
}

func safeEvidencePath153(value string) (string, bool) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" || path.IsAbs(value) {
		return "", false
	}
	clean := path.Clean(value)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	return clean, true
}

func validResourceRelation153(p string, relation ResourceRelation) bool {
	switch strings.TrimSpace(relation.Role) {
	case "MapperXml":
		return strings.HasPrefix(p, "src/main/resources/") && strings.HasSuffix(path.Base(p), "Mapper.xml") && relation.Source == "MAPPER_STATEMENT"
	case "YamlConfig":
		return strings.HasPrefix(p, "src/main/resources/") && strings.HasSuffix(p, ".yml") && relation.Source == "CONFIG_REFERENCE"
	default:
		return false
	}
}
