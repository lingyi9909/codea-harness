package chain

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	ValidationValid   = "VALID"
	ValidationStale   = "STALE"
	ValidationInvalid = "INVALID"
)

type ValidationResult struct {
	ChainID  string   `json:"chainId"`
	Status   string   `json:"status"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

type EvidenceProvider interface {
	Evidence() ChangeAnalysisEvidence
}

type EvidenceSnapshot ChangeAnalysisEvidence

func (e EvidenceSnapshot) Evidence() ChangeAnalysisEvidence {
	return ChangeAnalysisEvidence(e)
}

func Validate(root string, c Chain, evidence EvidenceProvider) ValidationResult {
	result := ValidationResult{ChainID: c.ID, Status: ValidationValid, Errors: []string{}, Warnings: []string{}}
	if err := validateModel(c); err != nil {
		result.Status = ValidationInvalid
		result.Errors = []string{"CHAIN_CONTRACT_INVALID: " + err.Error()}
		return result
	}
	if evidence == nil {
		result.Status = ValidationInvalid
		result.Errors = []string{"CHAIN_EVIDENCE_REQUIRED"}
		return result
	}
	if projectErrors := validateProjectIdentity(root, c.ID); len(projectErrors) != 0 {
		result.Status = ValidationInvalid
		result.Errors = projectErrors
		return result
	}

	analysis := evidence.Evidence()
	var factErrors []string
	for _, entry := range c.EntryPoints {
		if !verifiedEntryPoint(entry, analysis) {
			factErrors = append(factErrors, "ENTRYPOINT_NOT_VERIFIED: "+entry.Symbol+" @ "+normalizeRepoPath(entry.Path))
		}
	}
	for _, node := range c.Nodes {
		if !verifiedNode(node, analysis) {
			factErrors = append(factErrors, "NODE_NOT_VERIFIED: "+node.Symbol+" @ "+normalizeRepoPath(node.Path))
		}
	}
	for _, entry := range c.EntryPoints {
		if !verifiedNodeOrder(entry.Symbol, c.Nodes, analysis.CallChains) {
			factErrors = append(factErrors, "CALL_ORDER_NOT_VERIFIED: "+entry.Symbol)
		}
	}

	core := map[string]bool{}
	for _, node := range c.Nodes {
		core[node.Symbol] = true
	}
	for _, resource := range c.Resources {
		if !safeProjectRelativePath(resource.Path) {
			factErrors = append(factErrors, "RESOURCE_PATH_INVALID: "+resource.Path)
			continue
		}
		if _, err := os.Stat(filepath.Join(filepath.Clean(root), filepath.FromSlash(normalizeRepoPath(resource.Path)))); err != nil {
			factErrors = append(factErrors, "RESOURCE_MISSING: "+normalizeRepoPath(resource.Path))
			continue
		}
		if !verifiedResource(resource, core, analysis.ResourceRelations) {
			factErrors = append(factErrors, "RESOURCE_RELATION_NOT_VERIFIED: "+normalizeRepoPath(resource.Path))
		}
	}
	for _, boundary := range c.Boundaries {
		if !verifiedBoundary(boundary, core, analysis) {
			factErrors = append(factErrors, "BOUNDARY_NOT_VERIFIED: "+boundary.Symbol+" @ "+normalizeRepoPath(boundary.Path))
		}
	}

	if len(factErrors) == 0 {
		return result
	}
	sort.Strings(factErrors)
	result.Errors = factErrors
	if (c.Status == StatusAccepted || c.Status == StatusStale) && projectChainExists(root, c.ID) {
		result.Status = ValidationStale
	} else {
		result.Status = ValidationInvalid
	}
	return result
}

func verifiedEntryPoint(entry EntryPoint, analysis ChangeAnalysisEvidence) bool {
	symbol := strings.TrimSpace(entry.Symbol)
	path := normalizeRepoPath(entry.Path)
	if parentSymbol(symbol) == "" || !isProductionControllerPath(path) {
		return false
	}
	endpoint := false
	for _, affected := range analysis.AffectedControllers {
		if strings.TrimSpace(affected.Controller) != parentSymbol(symbol) {
			continue
		}
		for _, candidate := range affected.Endpoints {
			if strings.TrimSpace(candidate) == symbol {
				endpoint = true
				break
			}
		}
	}
	if !endpoint {
		return false
	}
	matches := 0
	for _, location := range analysis.SymbolLocations {
		if strings.TrimSpace(location.Symbol) == symbol && location.Role == "Controller" && normalizeRepoPath(location.Path) == path && isProductionControllerPath(location.Path) {
			matches++
		}
	}
	return matches == 1
}

func verifiedNode(node Node, analysis ChangeAnalysisEvidence) bool {
	matches := 0
	for _, location := range analysis.SymbolLocations {
		if strings.TrimSpace(location.Symbol) != strings.TrimSpace(node.Symbol) || normalizeRepoPath(location.Path) != normalizeRepoPath(node.Path) || !isProductionJavaPath(location.Path) {
			continue
		}
		if nodeRole(location.Role) != node.Role {
			continue
		}
		matches++
	}
	return matches == 1
}

func verifiedNodeOrder(entry string, nodes []Node, chains []CallChainEvidence) bool {
	want := make([]string, 0, len(nodes)+1)
	want = append(want, strings.TrimSpace(entry))
	for _, node := range nodes {
		want = append(want, strings.TrimSpace(node.Symbol))
	}
	for _, branch := range chains {
		if strings.TrimSpace(branch.EntryPoint) != strings.TrimSpace(entry) || len(branch.Chain) != len(want) {
			continue
		}
		matched := true
		for i := range want {
			if strings.TrimSpace(branch.Chain[i]) != want[i] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func verifiedResource(resource Resource, core map[string]bool, relations []ResourceRelationEvidence) bool {
	for _, relation := range relations {
		if normalizeRepoPath(relation.Path) != normalizeRepoPath(resource.Path) || resourceRole(relation.Role) != resource.Role {
			continue
		}
		if strings.TrimSpace(resource.Symbol) != "" && strings.TrimSpace(resource.Symbol) != strings.TrimSpace(relation.FromSymbol) {
			continue
		}
		if resourceRelationTouchesCore(relation, core) {
			return true
		}
	}
	return false
}

func verifiedBoundary(boundary Boundary, core map[string]bool, analysis ChangeAnalysisEvidence) bool {
	if !containsString(analysis.ExternalDependencies, boundary.Symbol) {
		return false
	}
	for _, location := range analysis.SymbolLocations {
		if strings.TrimSpace(location.Symbol) != strings.TrimSpace(boundary.Symbol) || normalizeRepoPath(location.Path) != normalizeRepoPath(boundary.Path) || !isProductionJavaPath(location.Path) {
			continue
		}
		if strings.TrimSpace(location.From) != "" && !core[strings.TrimSpace(location.From)] {
			continue
		}
		if boundaryRoleFromEvidence(location.Role) == boundary.Role {
			return true
		}
	}
	return false
}

func boundaryRoleFromEvidence(role string) string {
	switch strings.TrimSpace(role) {
	case "Cache", "CACHE":
		return "CACHE"
	case "MQ", "MessageQueue":
		return "MQ"
	default:
		return "EXTERNAL"
	}
}

func safeProjectRelativePath(path string) bool {
	if filepath.IsAbs(path) {
		return false
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

func projectChainExists(root, id string) bool {
	path, err := ChainPath(root, id)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

func validateProjectIdentity(root, id string) []string {
	dir := filepath.Join(filepath.Clean(root), ".code-harness", "chains")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return []string{"CHAIN_PROJECT_STATE_READ_FAILED: " + err.Error()}
	}
	var matches []string
	mismatch := ""
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".yaml") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		loaded, err := Load(path)
		if err != nil {
			continue
		}
		if loaded.ID != id {
			continue
		}
		matches = append(matches, entry.Name())
		if strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())) != id && mismatch == "" {
			mismatch = entry.Name()
		}
	}
	if len(matches) > 1 {
		sort.Strings(matches)
		return []string{fmt.Sprintf("DUPLICATE_PROJECT_CHAIN_ID: %q appears in %s", id, strings.Join(matches, ", "))}
	}
	if mismatch != "" {
		return []string{fmt.Sprintf("CHAIN_ID_FILENAME_MISMATCH: id %q is stored in %q", id, mismatch)}
	}
	return nil
}
