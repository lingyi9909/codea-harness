package analysis

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

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
	workspaceIDs := map[string]struct{}{}

	for _, loc := range a.SymbolLocations {
		symbol := strings.TrimSpace(loc.Symbol)
		p, ok := safeEvidencePath153(loc.Path)
		if symbol == "" || !ok {
			return fmt.Errorf("SYMBOL_LOCATION_INVALID: symbol=%q path=%q", loc.Symbol, loc.Path)
		}
		workspaceID := normalizeWorkspace153(loc.Workspace)
		role := strings.TrimSpace(loc.Role)
		candidate := fact{workspace: workspaceID, path: p, role: role}
		if previous, exists := facts[symbol]; exists && previous != candidate {
			return fmt.Errorf("SYMBOL_LOCATION_CONFLICT: %s has %s/%s/%s and %s/%s/%s", symbol, previous.workspace, previous.path, previous.role, candidate.workspace, candidate.path, candidate.role)
		}
		facts[symbol] = candidate
		if workspaceID != "current" {
			workspaceIDs[workspaceID] = struct{}{}
		}
	}

	ids := make([]string, 0, len(workspaceIDs))
	for id := range workspaceIDs { ids = append(ids, id) }
	sort.Strings(ids)
	for _, id := range ids {
		if err := verifyWorkspaceEvidence153(root, id); err != nil {
			return err
		}
	}

	// Changed/reviewed paths are always current-workspace paths. A dependency may
	// legitimately contain the same relative path; workspace identity, not the
	// bare relative path, determines whether evidence belongs to a dependency.
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
				controller := entrypointController153(entry)
				if controller == "" || !affectedControllerContainsEntrypoint153(a.AffectedControllers, controller, entry) {
					return fmt.Errorf("ENTRYPOINT_ANALYSIS_INCONSISTENT: %s is confirmed by callChain but missing exact affectedControllers controller/endpoint", entry)
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
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return "current"
	}
	return workspaceID
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
