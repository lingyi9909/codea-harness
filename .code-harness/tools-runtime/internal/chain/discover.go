package chain

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DiscoveryComplete = "COMPLETE"
	DiscoveryPartial  = "PARTIAL"
)

type ChangedFileEvidence struct {
	Path string `json:"path"`
	Role string `json:"role"`
}

type AffectedControllerEvidence struct {
	Controller    string   `json:"controller"`
	Endpoints     []string `json:"endpoints"`
	ImpactType    string   `json:"impactType"`
	SourceSymbols []string `json:"sourceSymbols"`
}

type CallChainEvidence struct {
	EntryPoint string   `json:"entryPoint"`
	Chain      []string `json:"chain"`
}

type SymbolLocationEvidence struct {
	Symbol string `json:"symbol"`
	Path   string `json:"path"`
	Role   string `json:"role"`
	Source string `json:"source"`
	From   string `json:"from,omitempty"`
}

type ResourceRelationEvidence struct {
	Path       string `json:"path"`
	Role       string `json:"role"`
	Resource   string `json:"resource"`
	FromSymbol string `json:"fromSymbol"`
	FromKind   string `json:"fromKind"`
	Source     string `json:"source"`
	Evidence   string `json:"evidence"`
}

type UnresolvedSymbolEvidence struct {
	Symbol string `json:"symbol"`
	From   string `json:"from"`
	Reason string `json:"reason"`
}

type ReviewCoverageEvidence struct {
	UnresolvedSymbols []UnresolvedSymbolEvidence `json:"unresolvedSymbols"`
}

type ChangeAnalysisEvidence struct {
	ChangedFiles         []ChangedFileEvidence         `json:"changedFiles"`
	AffectedControllers  []AffectedControllerEvidence  `json:"affectedControllers"`
	CallChains           []CallChainEvidence           `json:"callChains"`
	SymbolLocations      []SymbolLocationEvidence      `json:"symbolLocations"`
	ResourceRelations    []ResourceRelationEvidence    `json:"resourceRelations"`
	ExternalDependencies []string                      `json:"externalDependencies"`
	ReviewCoverage       ReviewCoverageEvidence        `json:"reviewCoverage"`
}

type DiscoverInput struct {
	RunID          string                 `json:"runId"`
	Target         string                 `json:"target,omitempty"`
	ChangeAnalysis ChangeAnalysisEvidence `json:"changeAnalysis"`
}

type DiscoveryResult struct {
	Status     string   `json:"status"`
	Chains     []Chain  `json:"chains"`
	Unresolved []string `json:"unresolved"`
}

var runIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
var targetPattern = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*(\.[A-Za-z_$][A-Za-z0-9_$]*)?$`)

func Discover(root string, in DiscoverInput) (DiscoveryResult, error) {
	result := DiscoveryResult{Status: DiscoveryComplete, Chains: []Chain{}, Unresolved: []string{}}
	if !runIDPattern.MatchString(in.RunID) {
		return result, fmt.Errorf("invalid chain discovery runId %q", in.RunID)
	}
	if in.Target != "" && !targetPattern.MatchString(in.Target) {
		return result, fmt.Errorf("invalid chain discovery target %q", in.Target)
	}

	var discovered []Chain
	for _, affected := range in.ChangeAnalysis.AffectedControllers {
		if !affectedMatchesTarget(affected, in.Target, in.ChangeAnalysis.CallChains) {
			continue
		}
		for _, endpoint := range affected.Endpoints {
			if in.Target != "" && !endpointMatchesTarget(endpoint, affected, in.Target, in.ChangeAnalysis.CallChains) {
				continue
			}
			entry, ok, unresolved := resolveEntry(endpoint, affected.Controller, in.ChangeAnalysis.SymbolLocations)
			if unresolved != "" {
				result.Status = DiscoveryPartial
				result.Unresolved = append(result.Unresolved, unresolved)
			}
			if !ok {
				continue
			}

			branches := callChainsForEntry(endpoint, in.ChangeAnalysis.CallChains)
			if len(branches) == 0 {
				result.Status = DiscoveryPartial
				result.Unresolved = append(result.Unresolved, "CALL_CHAIN_NOT_FOUND: "+endpoint)
				continue
			}
			for _, branch := range branches {
				candidate, unresolved := buildDiscoveredChain(entry, branch, in.ChangeAnalysis)
				if len(unresolved) != 0 {
					result.Status = DiscoveryPartial
					result.Unresolved = append(result.Unresolved, unresolved...)
					continue
				}
				discovered = append(discovered, candidate)
			}
		}
	}

	discovered = Canonicalize(discovered)
	for _, unresolved := range in.ChangeAnalysis.ReviewCoverage.UnresolvedSymbols {
		if unresolvedTouchesChains(unresolved, discovered) {
			result.Status = DiscoveryPartial
			result.Unresolved = append(result.Unresolved, fmt.Sprintf("%s <- %s: %s", unresolved.Symbol, unresolved.From, unresolved.Reason))
		}
	}
	result.Unresolved = uniqueSorted(result.Unresolved)
	result.Chains = discovered
	if err := persistDiscovered(root, in.RunID, discovered); err != nil {
		return result, err
	}
	return result, nil
}

func resolveEntry(symbol, affectedController string, locations []SymbolLocationEvidence) (EntryPoint, bool, string) {
	symbol = strings.TrimSpace(symbol)
	controller := strings.TrimSpace(affectedController)
	if parentSymbol(symbol) == "" || parentSymbol(symbol) != controller {
		return EntryPoint{}, false, "ENTRYPOINT_NOT_AFFECTED_CONTROLLER_METHOD: " + symbol
	}
	var matches []SymbolLocationEvidence
	for _, location := range locations {
		if location.Symbol != symbol || location.Role != "Controller" || !isProductionControllerPath(location.Path) {
			continue
		}
		matches = append(matches, location)
	}
	matches = uniqueLocations(matches)
	if len(matches) == 0 {
		return EntryPoint{}, false, "ENTRYPOINT_NOT_RESOLVED: " + symbol
	}
	if len(matches) > 1 {
		return EntryPoint{}, false, "AMBIGUOUS_ENTRYPOINT: " + symbol
	}
	return EntryPoint{Symbol: symbol, Path: normalizeRepoPath(matches[0].Path)}, true, ""
}

func buildDiscoveredChain(entry EntryPoint, branch CallChainEvidence, analysis ChangeAnalysisEvidence) (Chain, []string) {
	if len(branch.Chain) == 0 || branch.Chain[0] != branch.EntryPoint || branch.EntryPoint != entry.Symbol {
		return Chain{}, []string{"INVALID_CONFIRMED_CHAIN: " + entry.Symbol}
	}
	candidate := Chain{
		Version:     1,
		Name:        entry.Symbol,
		Status:      StatusDiscovered,
		EntryPoints: []EntryPoint{entry},
		Nodes:       []Node{},
		Resources:   []Resource{},
		Boundaries:  []Boundary{},
	}
	coreSymbols := map[string]bool{}
	var unresolved []string
	for _, symbol := range branch.Chain[1:] {
		location, ok, reason := resolveInternalNode(symbol, analysis.SymbolLocations)
		if !ok {
			unresolved = append(unresolved, reason)
			continue
		}
		candidate.Nodes = append(candidate.Nodes, Node{Symbol: symbol, Path: normalizeRepoPath(location.Path), Role: nodeRole(location.Role)})
		coreSymbols[symbol] = true
	}
	if len(unresolved) != 0 {
		return Chain{}, unresolved
	}

	for _, relation := range analysis.ResourceRelations {
		if !resourceRelationTouchesCore(relation, coreSymbols) {
			continue
		}
		role := resourceRole(relation.Role)
		if role == "" {
			continue
		}
		candidate.Resources = append(candidate.Resources, Resource{Path: normalizeRepoPath(relation.Path), Symbol: relation.FromSymbol, Role: role})
	}
	candidate.Resources = uniqueResources(candidate.Resources)

	for _, external := range analysis.ExternalDependencies {
		for _, location := range analysis.SymbolLocations {
			if location.Symbol == external && coreSymbols[location.From] && isProductionJavaPath(location.Path) {
				candidate.Boundaries = append(candidate.Boundaries, Boundary{Symbol: external, Path: normalizeRepoPath(location.Path), Role: "EXTERNAL"})
			}
		}
	}
	candidate.Boundaries = uniqueBoundaries(candidate.Boundaries)
	candidate.ID = discoveredID(candidate)
	return candidate, nil
}

func resolveInternalNode(symbol string, locations []SymbolLocationEvidence) (SymbolLocationEvidence, bool, string) {
	var matches []SymbolLocationEvidence
	for _, location := range locations {
		if location.Symbol == symbol && isProductionJavaPath(location.Path) {
			matches = append(matches, location)
		}
	}
	matches = uniqueLocations(matches)
	if len(matches) == 0 {
		return SymbolLocationEvidence{}, false, "INTERNAL_SYMBOL_NOT_RESOLVED: " + symbol
	}
	if len(matches) > 1 {
		return SymbolLocationEvidence{}, false, "AMBIGUOUS_INTERNAL_SYMBOL: " + symbol
	}
	return matches[0], true, ""
}

func affectedMatchesTarget(affected AffectedControllerEvidence, target string, chains []CallChainEvidence) bool {
	if target == "" {
		return true
	}
	if symbolMatchesTarget(affected.Controller, target) {
		return true
	}
	for _, endpoint := range affected.Endpoints {
		if symbolMatchesTarget(endpoint, target) {
			return true
		}
	}
	for _, source := range affected.SourceSymbols {
		if symbolMatchesTarget(source, target) {
			return true
		}
	}
	for _, chain := range chains {
		if !containsString(affected.Endpoints, chain.EntryPoint) {
			continue
		}
		for _, symbol := range chain.Chain {
			if symbolMatchesTarget(symbol, target) {
				return true
			}
		}
	}
	return false
}

func endpointMatchesTarget(endpoint string, affected AffectedControllerEvidence, target string, chains []CallChainEvidence) bool {
	if target == "" || symbolMatchesTarget(endpoint, target) || symbolMatchesTarget(affected.Controller, target) {
		return true
	}
	for _, chain := range chains {
		if chain.EntryPoint != endpoint {
			continue
		}
		for _, symbol := range chain.Chain {
			if symbolMatchesTarget(symbol, target) {
				return true
			}
		}
	}
	return false
}

func symbolMatchesTarget(symbol, target string) bool {
	if symbol == target {
		return true
	}
	if strings.Contains(target, ".") {
		return false
	}
	owner := symbol
	if i := strings.Index(owner, "."); i >= 0 {
		owner = owner[:i]
	}
	return owner == target
}

func callChainsForEntry(entry string, chains []CallChainEvidence) []CallChainEvidence {
	var out []CallChainEvidence
	for _, chain := range chains {
		if chain.EntryPoint == entry {
			out = append(out, chain)
		}
	}
	return out
}

func resourceRelationTouchesCore(relation ResourceRelationEvidence, coreSymbols map[string]bool) bool {
	switch strings.TrimSpace(relation.FromKind) {
	case "METHOD":
		return coreSymbols[strings.TrimSpace(relation.FromSymbol)]
	case "CLASS":
		fromClass := strings.TrimSpace(relation.FromSymbol)
		if fromClass == "" {
			return false
		}
		for symbol := range coreSymbols {
			if parentSymbol(symbol) == fromClass {
				return true
			}
		}
	}
	return false
}

func nodeRole(role string) string {
	switch role {
	case "Service":
		return "SERVICE"
	case "Repository":
		return "REPOSITORY"
	case "Mapper":
		return "MAPPER"
	default:
		return "OTHER"
	}
}

func resourceRole(role string) string {
	switch role {
	case "MapperXml":
		return "MAPPER_XML"
	case "YamlConfig":
		return "YAML_CONFIG"
	default:
		return ""
	}
}

func unresolvedTouchesChains(unresolved UnresolvedSymbolEvidence, chains []Chain) bool {
	for _, c := range chains {
		for _, entry := range c.EntryPoints {
			if entry.Symbol == unresolved.Symbol || entry.Symbol == unresolved.From {
				return true
			}
		}
		for _, node := range c.Nodes {
			if node.Symbol == unresolved.Symbol || node.Symbol == unresolved.From {
				return true
			}
		}
	}
	return false
}

func isProductionControllerPath(path string) bool {
	if !isProductionJavaPath(path) {
		return false
	}
	lower := strings.ToLower("/" + normalizeRepoPath(path) + "/")
	for _, marker := range []string{"/demo/", "/sample/", "/mock/"} {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	return true
}

func isProductionJavaPath(path string) bool {
	lower := strings.ToLower(normalizeRepoPath(path))
	if strings.HasPrefix(lower, "src/test/") || strings.Contains(lower, "/src/test/") {
		return false
	}
	return strings.HasPrefix(lower, "src/main/java/") || strings.Contains(lower, "/src/main/java/")
}

func normalizeRepoPath(path string) string {
	return strings.ReplaceAll(filepath.ToSlash(filepath.Clean(path)), "\\", "/")
}

func discoveredID(c Chain) string {
	sum := sha256.Sum256([]byte(coreSignature(c)))
	return fmt.Sprintf("%x", sum[:])
}

func persistDiscovered(root, runID string, chains []Chain) error {
	if !runIDPattern.MatchString(runID) {
		return fmt.Errorf("invalid chain discovery runId %q", runID)
	}
	dir := filepath.Join(filepath.Clean(root), ".code-harness", "runs", runID, "analysis", "discovered-chains")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create discovered chain directory: %w", err)
	}
	for _, c := range chains {
		if c.Status != StatusDiscovered {
			return errors.New("discovery may persist only DISCOVERED chains")
		}
		if err := ValidateID(c.ID); err != nil {
			return err
		}
		data, err := yaml.Marshal(c)
		if err != nil {
			return fmt.Errorf("marshal discovered chain %s: %w", c.ID, err)
		}
		path := filepath.Join(dir, c.ID+".yaml")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("write discovered chain %s: %w", c.ID, err)
		}
	}
	return nil
}

func uniqueLocations(in []SymbolLocationEvidence) []SymbolLocationEvidence {
	seen := map[string]bool{}
	var out []SymbolLocationEvidence
	for _, location := range in {
		key := location.Symbol + "\x00" + normalizeRepoPath(location.Path) + "\x00" + location.Role + "\x00" + location.Source
		if seen[key] {
			continue
		}
		seen[key] = true
		location.Path = normalizeRepoPath(location.Path)
		out = append(out, location)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			return out[i].Source < out[j].Source
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func uniqueResources(in []Resource) []Resource {
	seen := map[string]bool{}
	var out []Resource
	for _, resource := range in {
		key := resource.Path + "\x00" + resource.Symbol + "\x00" + resource.Role
		if !seen[key] {
			seen[key] = true
			out = append(out, resource)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			return out[i].Symbol < out[j].Symbol
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func uniqueBoundaries(in []Boundary) []Boundary {
	seen := map[string]bool{}
	var out []Boundary
	for _, boundary := range in {
		key := boundary.Symbol + "\x00" + boundary.Path + "\x00" + boundary.Role
		if !seen[key] {
			seen[key] = true
			out = append(out, boundary)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			return out[i].Symbol < out[j].Symbol
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func uniqueSorted(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range in {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
