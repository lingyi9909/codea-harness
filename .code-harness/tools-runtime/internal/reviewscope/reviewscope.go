package reviewscope

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"codea-harness-tools/internal/projectpath"
	"codea-harness-tools/internal/symbolid"
)

const currentWorkspace = symbolid.CurrentWorkspace

type Target struct {
	Symbol string `json:"symbol"`
	Kind   string `json:"kind"`
}

type CallChain struct {
	EntryPoint    string         `json:"entryPoint"`
	Chain         []string       `json:"chain"`
	EntryPointRef *symbolid.Ref  `json:"entryPointRef,omitempty"`
	ChainRefs     []symbolid.Ref `json:"chainRefs,omitempty"`
}

type Selection struct {
	Mode               string      `json:"mode"`
	Target             *Target     `json:"target,omitempty"`
	SelectedCallChains []CallChain `json:"selectedCallChains"`
	ScopedFiles        []string    `json:"scopedFiles"`
	allChangedFiles    []string
}

type CoverageResult struct {
	Status            string   `json:"status"`
	MissingFiles      []string `json:"missingFiles,omitempty"`
	UnresolvedSymbols []string `json:"unresolvedSymbols,omitempty"`
}

type SymbolLocation struct {
	Workspace string `json:"workspace,omitempty"`
	Symbol    string `json:"symbol"`
	Path      string `json:"path"`
	Role      string `json:"role"`
	Source    string `json:"source"`
	From      string `json:"from,omitempty"`
}

type ResourceRelation struct {
	Path       string `json:"path"`
	Role       string `json:"role"`
	Resource   string `json:"resource"`
	FromSymbol string `json:"fromSymbol"`
	FromKind   string `json:"fromKind"`
	Source     string `json:"source"`
	Evidence   string `json:"evidence"`
}

type analysisFile struct {
	Path string `json:"path"`
	Role string `json:"role"`
}

type unresolvedSymbol struct {
	Symbol string `json:"symbol"`
	From   string `json:"from"`
}

type changeAnalysis struct {
	ChangedFiles      []analysisFile     `json:"changedFiles"`
	CallChains        []CallChain        `json:"callChains"`
	SymbolLocations   []SymbolLocation   `json:"symbolLocations"`
	ResourceRelations []ResourceRelation `json:"resourceRelations"`
	ReviewCoverage    struct {
		ReviewedFiles     []analysisFile     `json:"reviewedFiles"`
		UnresolvedSymbols []unresolvedSymbol `json:"unresolvedSymbols"`
	} `json:"reviewCoverage"`
}

type navigationEvidence struct {
	bySymbol         map[string]SymbolLocation
	byIdentity       map[string]SymbolLocation
	ambiguousSymbols map[string]struct{}
	locations        []SymbolLocation
	methodSymbols    map[string]struct{}
}

func Verify(selectionJSON, changeAnalysisJSON []byte) (Selection, error) {
	selection, err := decodeSelection(selectionJSON)
	if err != nil { return Selection{}, err }
	analysis, err := parseChangeAnalysis(changeAnalysisJSON)
	if err != nil { return Selection{}, err }

	if selection.Mode != "FULL" && selection.Mode != "TARGETED" {
		return Selection{}, fmt.Errorf("invalid review scope mode %q", selection.Mode)
	}
	if selection.Mode == "FULL" {
		if selection.Target != nil { return Selection{}, errors.New("FULL review scope must not contain target") }
	} else {
		if selection.Target == nil || strings.TrimSpace(selection.Target.Symbol) == "" {
			return Selection{}, errors.New("TARGETED review scope requires target")
		}
		if selection.Target.Kind != "CLASS" && selection.Target.Kind != "METHOD" {
			return Selection{}, fmt.Errorf("invalid target kind %q", selection.Target.Kind)
		}
		if len(selection.SelectedCallChains) == 0 || len(selection.ScopedFiles) == 0 {
			return Selection{}, errors.New("TARGETED review scope requires selectedCallChains and scopedFiles")
		}
	}

	changedRoles, err := validateChangedAndReviewedResourceRoles(analysis.ChangedFiles, analysis.ReviewCoverage.ReviewedFiles)
	if err != nil { return Selection{}, err }

	selection.allChangedFiles = make([]string, 0, len(analysis.ChangedFiles))
	for _, f := range analysis.ChangedFiles {
		p := normalizePath(f.Path)
		if p != "" && p != "." { selection.allChangedFiles = append(selection.allChangedFiles, p) }
	}
	selection.allChangedFiles = uniqueSorted(selection.allChangedFiles)

	if selection.Mode == "FULL" {
		if len(analysis.ResourceRelations) > 0 {
			evidence, err := buildNavigationEvidence(analysis.SymbolLocations)
			if err != nil { return Selection{}, err }
			if err := validateResourceRelationsBase(analysis.ResourceRelations, changedRoles, evidence); err != nil { return Selection{}, err }
		}
		return selection, nil
	}

	for _, selected := range selection.SelectedCallChains {
		if !containsChain(analysis.CallChains, selected) {
			return Selection{}, fmt.Errorf("selected call chain %q is not present in validated ChangeAnalysis", selected.EntryPoint)
		}
	}
	if !targetInChains(*selection.Target, selection.SelectedCallChains) {
		return Selection{}, fmt.Errorf("target %q is not represented by selected call chains", selection.Target.Symbol)
	}

	evidence, err := buildNavigationEvidence(analysis.SymbolLocations)
	if err != nil { return Selection{}, err }
	targetRole, err := resolveTargetRole(*selection.Target, evidence, selection.SelectedCallChains)
	if err != nil { return Selection{}, err }
	if targetRole == "Controller" {
		if err := verifyAllControllerChains(*selection.Target, selection.SelectedCallChains, analysis.CallChains); err != nil { return Selection{}, err }
	}

	allowedPaths, requiredPaths, err := exactScopePaths(*selection.Target, selection.SelectedCallChains, evidence, analysis.ResourceRelations, changedRoles)
	if err != nil { return Selection{}, err }
	selectedPaths := make(map[string]struct{}, len(selection.ScopedFiles))
	for i, f := range selection.ScopedFiles {
		p, err := normalizeScopedPath(f)
		if err != nil { return Selection{}, err }
		selection.ScopedFiles[i] = p
		if _, ok := allowedPaths[p]; !ok {
			return Selection{}, fmt.Errorf("scoped file %q is not an exact Code Navigation path or evidence-related changed resource for the selected target/call chains", f)
		}
		selectedPaths[p] = struct{}{}
	}
	selection.ScopedFiles = uniqueSorted(selection.ScopedFiles)
	for required := range requiredPaths {
		if _, ok := selectedPaths[required]; ok { continue }
		if navigationPath(required, evidence) {
			return Selection{}, fmt.Errorf("selected internal symbol exact Code Navigation path %q is missing from scopedFiles", required)
		}
		return Selection{}, fmt.Errorf("evidence-related changed resource path %q is missing from scopedFiles", required)
	}
	return selection, nil
}

func ComputeCoverage(selection Selection, reviewedFiles []string) CoverageResult {
	required := selection.ScopedFiles
	if selection.Mode == "FULL" { required = selection.allChangedFiles }
	seen := make(map[string]struct{}, len(reviewedFiles))
	for _, f := range reviewedFiles { seen[normalizePath(f)] = struct{}{} }
	missing := make([]string, 0)
	for _, f := range required {
		p := normalizePath(f)
		if _, ok := seen[p]; !ok { missing = append(missing, p) }
	}
	missing = uniqueSorted(missing)
	status := "COMPLETE"
	if len(missing) > 0 { status = "PARTIAL" }
	return CoverageResult{Status: status, MissingFiles: missing}
}

func ComputeCoverageFromAnalysis(selection Selection, changeAnalysisJSON []byte) (CoverageResult, error) {
	analysis, err := parseChangeAnalysis(changeAnalysisJSON)
	if err != nil { return CoverageResult{}, err }
	if _, err := validateChangedAndReviewedResourceRoles(analysis.ChangedFiles, analysis.ReviewCoverage.ReviewedFiles); err != nil { return CoverageResult{}, err }
	reviewed := make([]string, 0, len(analysis.ReviewCoverage.ReviewedFiles))
	for _, file := range analysis.ReviewCoverage.ReviewedFiles { reviewed = append(reviewed, file.Path) }
	result := ComputeCoverage(selection, reviewed)
	result.UnresolvedSymbols = relevantUnresolved(selection, analysis.ReviewCoverage.UnresolvedSymbols)
	if len(result.UnresolvedSymbols) > 0 { result.Status = "PARTIAL" }
	return result, nil
}

func validateChangedAndReviewedResourceRoles(changed, reviewed []analysisFile) (map[string]string, error) {
	changedRoles := make(map[string]string, len(changed))
	for _, file := range changed {
		p, err := normalizeScopedPath(file.Path)
		if err != nil { return nil, fmt.Errorf("changed file: %w", err) }
		role := strings.TrimSpace(file.Role)
		if err := validateResourcePathRole(p, role); err != nil { return nil, fmt.Errorf("changed file: %w", err) }
		if previous, exists := changedRoles[p]; exists && previous != role {
			return nil, fmt.Errorf("changed file %q has conflicting roles %q and %q", p, previous, role)
		}
		changedRoles[p] = role
	}
	for _, file := range reviewed {
		p, err := normalizeScopedPath(file.Path)
		if err != nil { return nil, fmt.Errorf("reviewed file: %w", err) }
		role := strings.TrimSpace(file.Role)
		changedRole, changedFile := changedRoles[p]
		if changedFile && (isResourceRole(changedRole) || isResourceRole(role) || expectedResourceRole(p) != "") && role != changedRole {
			return nil, fmt.Errorf("reviewed file role %q for %q does not match changed file role %q", role, p, changedRole)
		}
		if err := validateResourcePathRole(p, role); err != nil { return nil, fmt.Errorf("reviewed file: %w", err) }
	}
	return changedRoles, nil
}

func validateResourcePathRole(value, role string) error {
	expected := expectedResourceRole(value)
	if expected != "" && role != expected { return fmt.Errorf("resource path %q must use role %s, got %q", value, expected, role) }
	switch role {
	case "MapperXml":
		if expected != "MapperXml" { return fmt.Errorf("MapperXml path %q must match src/main/resources/**/*Mapper.xml", value) }
	case "YamlConfig":
		if expected != "YamlConfig" { return fmt.Errorf("YamlConfig path %q must match src/main/resources/**/*.yml", value) }
	}
	return nil
}

func expectedResourceRole(value string) string {
	switch projectpath.Classify(value) {
	case projectpath.MapperXML: return "MapperXml"
	case projectpath.YAMLConfig: return "YamlConfig"
	default: return ""
	}
}

func isResourceRole(role string) bool { return role == "MapperXml" || role == "YamlConfig" }

func buildNavigationEvidence(locations []SymbolLocation) (navigationEvidence, error) {
	if len(locations) == 0 { return navigationEvidence{}, errors.New("TARGETED review requires ChangeAnalysis.symbolLocations from Code Navigation") }
	e := navigationEvidence{
		bySymbol: make(map[string]SymbolLocation),
		byIdentity: make(map[string]SymbolLocation),
		ambiguousSymbols: make(map[string]struct{}),
		locations: make([]SymbolLocation, 0, len(locations)),
		methodSymbols: make(map[string]struct{}),
	}
	for _, raw := range locations {
		ref, ok := symbolid.FromLocation(raw.Workspace, raw.Path, raw.Symbol)
		if !ok { return navigationEvidence{}, fmt.Errorf("navigation symbol location requires valid symbol/path: %q/%q", raw.Symbol, raw.Path) }
		loc := raw
		loc.Workspace = ref.Workspace
		loc.Symbol = ref.Symbol
		loc.Path = ref.Path
		loc.Role = strings.TrimSpace(loc.Role)
		loc.From = strings.TrimSpace(loc.From)
		key, _ := symbolid.Key(ref)
		if previous, exists := e.byIdentity[key]; exists {
			if previous.Role != loc.Role { return navigationEvidence{}, fmt.Errorf("conflicting Code Navigation role for identity %s/%q/%q", loc.Workspace, loc.Path, loc.Symbol) }
			continue
		}
		e.byIdentity[key] = loc
		e.locations = append(e.locations, loc)
		if _, ambiguous := e.ambiguousSymbols[loc.Symbol]; ambiguous { continue }
		if previous, exists := e.bySymbol[loc.Symbol]; exists {
			if previous.Workspace != loc.Workspace || previous.Path != loc.Path || previous.Role != loc.Role {
				delete(e.bySymbol, loc.Symbol)
				e.ambiguousSymbols[loc.Symbol] = struct{}{}
			}
			continue
		}
		e.bySymbol[loc.Symbol] = loc
	}
	for symbol, loc := range e.bySymbol {
		parent := parentSymbol(symbol)
		if parent == "" { continue }
		if parentLoc, ok := e.bySymbol[parent]; ok && parentLoc.Workspace == loc.Workspace && parentLoc.Path == loc.Path { e.methodSymbols[symbol] = struct{}{} }
	}
	return e, nil
}

func navigationPath(value string, evidence navigationEvidence) bool {
	for _, loc := range evidence.locations {
		if loc.Workspace == currentWorkspace && loc.Path == value { return true }
	}
	return false
}

func resolveNavigationLocation(symbol string, exact *symbolid.Ref, evidence navigationEvidence) (SymbolLocation, error) {
	symbol = strings.TrimSpace(symbol)
	if exact != nil {
		ref, ok := symbolid.Normalize(*exact)
		if !ok || ref.Symbol != symbol { return SymbolLocation{}, fmt.Errorf("symbol %q has invalid exact Code Navigation ref", symbol) }
		key, _ := symbolid.Key(ref)
		loc, exists := evidence.byIdentity[key]
		if !exists { return SymbolLocation{}, fmt.Errorf("symbol %q exact ref %s/%q has no Code Navigation evidence", symbol, ref.Workspace, ref.Path) }
		return loc, nil
	}
	if _, ambiguous := evidence.ambiguousSymbols[symbol]; ambiguous {
		return SymbolLocation{}, fmt.Errorf("symbol %q has ambiguous Code Navigation path evidence", symbol)
	}
	loc, ok := evidence.bySymbol[symbol]
	if !ok { return SymbolLocation{}, fmt.Errorf("symbol %q has no exact Code Navigation path evidence", symbol) }
	return loc, nil
}

func targetExactRef(target Target, chains []CallChain) (*symbolid.Ref, error) {
	refs := map[string]symbolid.Ref{}
	add := func(symbol string, ref *symbolid.Ref) {
		if ref == nil { return }
		normalized, ok := symbolid.Normalize(*ref)
		if !ok { return }
		matches := strings.TrimSpace(symbol) == strings.TrimSpace(target.Symbol)
		if target.Kind == "CLASS" { matches = className(symbol, "METHOD") == className(target.Symbol, "CLASS") || className(symbol, "CLASS") == className(target.Symbol, "CLASS") }
		if !matches { return }
		key, _ := symbolid.Key(normalized)
		refs[key] = normalized
	}
	for _, chain := range chains {
		add(chain.EntryPoint, chain.EntryPointRef)
		if len(chain.ChainRefs) != 0 && len(chain.ChainRefs) != len(chain.Chain) { return nil, fmt.Errorf("selected call chain %q has invalid chainRefs length", chain.EntryPoint) }
		for i, node := range chain.Chain {
			if len(chain.ChainRefs) > 0 { ref := chain.ChainRefs[i]; add(node, &ref) }
		}
	}
	if len(refs) == 0 { return nil, nil }
	if len(refs) != 1 { return nil, fmt.Errorf("target %q spans multiple path-qualified symbol identities", target.Symbol) }
	for _, ref := range refs { copy := ref; return &copy, nil }
	return nil, nil
}

func resolveTargetRole(target Target, evidence navigationEvidence, chains []CallChain) (string, error) {
	exact, err := targetExactRef(target, chains)
	if err != nil { return "", err }
	if target.Kind == "METHOD" {
		loc, err := resolveNavigationLocation(target.Symbol, exact, evidence)
		if err != nil || loc.Workspace != currentWorkspace {
			return "", fmt.Errorf("target %q has no unique current-workspace Code Navigation path evidence", target.Symbol)
		}
		return loc.Role, nil
	}
	if exact != nil {
		key, _ := symbolid.Key(*exact)
		loc, ok := evidence.byIdentity[key]
		if !ok || loc.Workspace != currentWorkspace { return "", fmt.Errorf("target %q has no exact current-workspace Code Navigation path evidence", target.Symbol) }
		return loc.Role, nil
	}
	targetClass := className(target.Symbol, "CLASS")
	roles := map[string]struct{}{}
	paths := map[string]struct{}{}
	for _, loc := range evidence.locations {
		if loc.Workspace != currentWorkspace { continue }
		if className(loc.Symbol, "METHOD") != targetClass && className(loc.Symbol, "CLASS") != targetClass { continue }
		roles[loc.Role] = struct{}{}
		paths[loc.Path] = struct{}{}
	}
	if len(roles) == 0 { return "", fmt.Errorf("target %q has no exact current-workspace Code Navigation path evidence", target.Symbol) }
	if len(roles) != 1 || len(paths) != 1 { return "", fmt.Errorf("target %q has ambiguous Code Navigation role/path evidence", target.Symbol) }
	for role := range roles { return role, nil }
	return "", errors.New("unreachable target role")
}

func exactScopePaths(target Target, chains []CallChain, evidence navigationEvidence, relations []ResourceRelation, changedRoles map[string]string) (map[string]struct{}, map[string]struct{}, error) {
	nodes := selectedNodes(chains)
	allowed := make(map[string]struct{})
	required := make(map[string]struct{})
	selectedNavigationPaths := make(map[string]struct{})
	selectedIdentities := make(map[string]struct{})
	selectedBySymbol := make(map[string]map[string]struct{})
	addResolved := func(symbol string, exact *symbolid.Ref) error {
		loc, err := resolveNavigationLocation(symbol, exact, evidence)
		if err != nil { return err }
		ref, _ := symbolid.FromLocation(loc.Workspace, loc.Path, loc.Symbol)
		key, _ := symbolid.Key(ref)
		selectedIdentities[key] = struct{}{}
		if selectedBySymbol[loc.Symbol] == nil { selectedBySymbol[loc.Symbol] = map[string]struct{}{} }
		selectedBySymbol[loc.Symbol][key] = struct{}{}
		if loc.Workspace == currentWorkspace {
			allowed[loc.Path] = struct{}{}
			required[loc.Path] = struct{}{}
			selectedNavigationPaths[loc.Path] = struct{}{}
		}
		return nil
	}
	for _, chain := range chains {
		if err := addResolved(strings.TrimSpace(chain.EntryPoint), chain.EntryPointRef); err != nil { return nil, nil, fmt.Errorf("selected entryPoint: %w", err) }
		if len(chain.ChainRefs) != 0 && len(chain.ChainRefs) != len(chain.Chain) { return nil, nil, fmt.Errorf("selected call chain %q has invalid chainRefs length", chain.EntryPoint) }
		for i, rawNode := range chain.Chain {
			node := strings.TrimSpace(rawNode)
			var exact *symbolid.Ref
			if len(chain.ChainRefs) > 0 { ref := chain.ChainRefs[i]; exact = &ref }
			if err := addResolved(node, exact); err != nil { return nil, nil, fmt.Errorf("selected internal symbol %q: %w", node, err) }
		}
	}
	for _, loc := range evidence.locations {
		if loc.Workspace != currentWorkspace { continue }
		ref, _ := symbolid.FromLocation(loc.Workspace, loc.Path, loc.Symbol)
		key, _ := symbolid.Key(ref)
		_, symbolSelected := selectedIdentities[key]
		fromSelected := loc.From != "" && len(selectedBySymbol[loc.From]) == 1
		targetClassRelated := target.Kind == "CLASS" && className(loc.Symbol, "METHOD") == className(target.Symbol, "CLASS")
		if targetClassRelated {
			classPaths := map[string]struct{}{}
			for _, candidate := range evidence.locations {
				if candidate.Workspace == currentWorkspace && (className(candidate.Symbol, "METHOD") == className(target.Symbol, "CLASS") || className(candidate.Symbol, "CLASS") == className(target.Symbol, "CLASS")) { classPaths[candidate.Path] = struct{}{} }
			}
			targetClassRelated = len(classPaths) == 1
		}
		if symbolSelected || fromSelected || targetClassRelated { allowed[loc.Path] = struct{}{} }
	}

	for _, raw := range relations {
		relation, err := normalizeResourceRelation(raw)
		if err != nil { return nil, nil, err }
		if err := validateResourceRelationChangedRole(relation, changedRoles); err != nil { return nil, nil, err }
		fromLocation, err := resolveResourceRelationEvidence(relation, evidence)
		if err != nil { return nil, nil, err }
		touches, err := resourceRelationTouchesSelected(relation, fromLocation, nodes, selectedNavigationPaths)
		if err != nil { return nil, nil, err }
		if !touches { continue }
		allowed[relation.Path] = struct{}{}
		required[relation.Path] = struct{}{}
	}
	return allowed, required, nil
}

func validateResourceRelationsBase(relations []ResourceRelation, changedRoles map[string]string, evidence navigationEvidence) error {
	for _, raw := range relations {
		relation, err := normalizeResourceRelation(raw)
		if err != nil { return err }
		if err := validateResourceRelationChangedRole(relation, changedRoles); err != nil { return err }
		if _, err := resolveResourceRelationEvidence(relation, evidence); err != nil { return err }
	}
	return nil
}

func validateResourceRelationChangedRole(relation ResourceRelation, changedRoles map[string]string) error {
	changedRole, changed := changedRoles[relation.Path]
	if !changed { return fmt.Errorf("resource relation path %q is not present in changedFiles", relation.Path) }
	if changedRole != relation.Role { return fmt.Errorf("resource relation role %q for %q does not match changed file role %q", relation.Role, relation.Path, changedRole) }
	return nil
}

func normalizeResourceRelation(raw ResourceRelation) (ResourceRelation, error) {
	relation := raw
	p, err := normalizeScopedPath(raw.Path)
	if err != nil { return ResourceRelation{}, fmt.Errorf("resource relation: %w", err) }
	relation.Path = p
	relation.Role = strings.TrimSpace(raw.Role)
	relation.Resource = strings.TrimSpace(raw.Resource)
	relation.FromSymbol = strings.TrimSpace(raw.FromSymbol)
	relation.FromKind = strings.TrimSpace(raw.FromKind)
	relation.Source = strings.TrimSpace(raw.Source)
	relation.Evidence = strings.TrimSpace(raw.Evidence)
	if relation.Resource == "" || relation.FromSymbol == "" || relation.Evidence == "" { return ResourceRelation{}, fmt.Errorf("resource relation %q requires resource, fromSymbol, and evidence", relation.Path) }
	if relation.FromKind != "CLASS" && relation.FromKind != "METHOD" { return ResourceRelation{}, fmt.Errorf("resource relation %q has invalid fromKind %q", relation.Path, relation.FromKind) }
	if err := validateResourcePathRole(relation.Path, relation.Role); err != nil { return ResourceRelation{}, fmt.Errorf("resource relation: %w", err) }
	switch relation.Role {
	case "MapperXml":
		if relation.Source != "MAPPER_STATEMENT" { return ResourceRelation{}, fmt.Errorf("MapperXml resource relation %q requires MAPPER_STATEMENT source", relation.Path) }
	case "YamlConfig":
		if relation.Source != "CONFIG_REFERENCE" { return ResourceRelation{}, fmt.Errorf("YamlConfig resource relation %q requires CONFIG_REFERENCE source", relation.Path) }
	default:
		return ResourceRelation{}, fmt.Errorf("resource relation %q has unsupported role %q", relation.Path, relation.Role)
	}
	return relation, nil
}

func resolveResourceRelationEvidence(relation ResourceRelation, evidence navigationEvidence) (SymbolLocation, error) {
	loc, err := resolveNavigationLocation(relation.FromSymbol, nil, evidence)
	if err != nil { return SymbolLocation{}, fmt.Errorf("resource relation fromSymbol %q: %w", relation.FromSymbol, err) }
	if loc.Workspace != currentWorkspace { return SymbolLocation{}, fmt.Errorf("resource relation fromSymbol %q belongs to dependency workspace %q", relation.FromSymbol, loc.Workspace) }
	if relation.FromKind == "CLASS" {
		if _, method := evidence.methodSymbols[relation.FromSymbol]; method { return SymbolLocation{}, fmt.Errorf("resource relation fromKind=CLASS requires class symbol, got method symbol %q", relation.FromSymbol) }
	}
	return loc, nil
}

func resourceRelationTouchesSelected(relation ResourceRelation, fromLocation SymbolLocation, nodes, selectedNavigationPaths map[string]struct{}) (bool, error) {
	if relation.FromKind == "METHOD" {
		if _, selected := nodes[relation.FromSymbol]; !selected { return false, nil }
		_, selectedPath := selectedNavigationPaths[fromLocation.Path]
		return selectedPath, nil
	}
	classMatched := false
	for node := range nodes {
		if parentSymbol(node) == relation.FromSymbol { classMatched = true; break }
	}
	if !classMatched { return false, fmt.Errorf("resource relation class %q does not match any selected chain class", relation.FromSymbol) }
	_, selectedPath := selectedNavigationPaths[fromLocation.Path]
	return selectedPath, nil
}

func parentSymbol(symbol string) string {
	symbol = strings.TrimSpace(symbol)
	index := strings.LastIndex(symbol, ".")
	if index <= 0 { return "" }
	return symbol[:index]
}

func verifyAllControllerChains(target Target, selected, all []CallChain) error {
	required := make([]CallChain, 0)
	for _, chain := range all {
		if target.Kind == "METHOD" {
			if strings.TrimSpace(chain.EntryPoint) == strings.TrimSpace(target.Symbol) { required = append(required, chain) }
			continue
		}
		if className(chain.EntryPoint, "METHOD") == className(target.Symbol, "CLASS") { required = append(required, chain) }
	}
	if len(required) == 0 { return fmt.Errorf("Controller target %q has no confirmed call chains", target.Symbol) }
	if !sameChainSet(selected, required) { return fmt.Errorf("Controller target %q must include all confirmed Controller chains; selected=%d required=%d", target.Symbol, len(selected), len(required)) }
	return nil
}

func sameChainSet(a, b []CallChain) bool {
	if len(a) != len(b) { return false }
	set := make(map[string]int, len(a))
	for _, chain := range a { set[chainKey(chain)]++ }
	for _, chain := range b {
		key := chainKey(chain)
		if set[key] == 0 { return false }
		set[key]--
	}
	return true
}

func hasExactRefs(chain CallChain) bool { return chain.EntryPointRef != nil || len(chain.ChainRefs) > 0 }

func chainKey(chain CallChain) string {
	parts := []string{strings.TrimSpace(chain.EntryPoint)}
	for _, node := range chain.Chain { parts = append(parts, strings.TrimSpace(node)) }
	if chain.EntryPointRef != nil {
		if key, ok := symbolid.Key(*chain.EntryPointRef); ok { parts = append(parts, "entryRef="+key) }
	}
	for _, ref := range chain.ChainRefs {
		if key, ok := symbolid.Key(ref); ok { parts = append(parts, "chainRef="+key) }
	}
	return strings.Join(parts, "\x00")
}

func visibleChainEqual(left, right CallChain) bool {
	if strings.TrimSpace(left.EntryPoint) != strings.TrimSpace(right.EntryPoint) || len(left.Chain) != len(right.Chain) { return false }
	for i := range left.Chain {
		if strings.TrimSpace(left.Chain[i]) != strings.TrimSpace(right.Chain[i]) { return false }
	}
	return true
}

func selectedNodes(chains []CallChain) map[string]struct{} {
	nodes := make(map[string]struct{})
	for _, chain := range chains {
		entry := strings.TrimSpace(chain.EntryPoint)
		if entry != "" { nodes[entry] = struct{}{} }
		for _, node := range chain.Chain {
			node = strings.TrimSpace(node)
			if node != "" { nodes[node] = struct{}{} }
		}
	}
	return nodes
}

func relevantUnresolved(selection Selection, unresolved []unresolvedSymbol) []string {
	if selection.Mode == "FULL" {
		values := make([]string, 0, len(unresolved))
		for _, item := range unresolved { values = append(values, strings.TrimSpace(item.Symbol)) }
		return uniqueSorted(values)
	}
	nodes := selectedNodes(selection.SelectedCallChains)
	values := make([]string, 0)
	for _, item := range unresolved {
		_, symbolSelected := nodes[strings.TrimSpace(item.Symbol)]
		_, fromSelected := nodes[strings.TrimSpace(item.From)]
		if symbolSelected || fromSelected { values = append(values, strings.TrimSpace(item.Symbol)) }
	}
	return uniqueSorted(values)
}

func parseChangeAnalysis(data []byte) (changeAnalysis, error) {
	var analysis changeAnalysis
	if err := json.Unmarshal(data, &analysis); err != nil { return changeAnalysis{}, fmt.Errorf("parse change analysis: %w", err) }
	return analysis, nil
}

func decodeSelection(data []byte) (Selection, error) {
	var selection Selection
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&selection); err != nil { return Selection{}, fmt.Errorf("parse review scope selection: %w", err) }
	var extra any
	if err := dec.Decode(&extra); err == nil { return Selection{}, errors.New("parse review scope selection: multiple JSON values are not allowed") }
	return selection, nil
}

func containsChain(chains []CallChain, selected CallChain) bool {
	matches := 0
	for _, candidate := range chains {
		if !visibleChainEqual(candidate, selected) { continue }
		if hasExactRefs(selected) {
			if chainKey(candidate) == chainKey(selected) { return true }
			continue
		}
		matches++
	}
	return matches == 1
}

func targetInChains(target Target, chains []CallChain) bool {
	for _, chain := range chains {
		nodes := append([]string{chain.EntryPoint}, chain.Chain...)
		for _, node := range nodes {
			if target.Kind == "METHOD" && strings.TrimSpace(node) == strings.TrimSpace(target.Symbol) { return true }
			if target.Kind == "CLASS" && className(node, "METHOD") == className(target.Symbol, "CLASS") { return true }
		}
	}
	return false
}

func className(symbol, kind string) string {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" { return "" }
	parts := strings.Split(symbol, ".")
	if kind == "METHOD" && len(parts) >= 2 { return parts[len(parts)-2] }
	return parts[len(parts)-1]
}

func effectiveWorkspace(value string) string { return symbolid.NormalizeWorkspace(value) }

func normalizeScopedPath(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" || strings.HasPrefix(value, "/") || strings.Contains(value, ":") { return "", fmt.Errorf("scoped file %q must be repository-relative", value) }
	clean := path.Clean(value)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") { return "", fmt.Errorf("scoped file %q escapes repository root", value) }
	return clean, nil
}

func normalizePath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	return path.Clean(value)
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || value == "." { continue }
		if _, ok := seen[value]; ok { continue }
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
