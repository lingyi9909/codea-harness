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
)

const currentWorkspace = "current"

type Target struct {
	Symbol string `json:"symbol"`
	Kind   string `json:"kind"`
}

type CallChain struct {
	EntryPoint string   `json:"entryPoint"`
	Chain      []string `json:"chain"`
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
	bySymbol      map[string]SymbolLocation
	locations     []SymbolLocation
	methodSymbols map[string]struct{}
}

func Verify(selectionJSON, changeAnalysisJSON []byte) (Selection, error) {
	selection, err := decodeSelection(selectionJSON)
	if err != nil {
		return Selection{}, err
	}
	analysis, err := parseChangeAnalysis(changeAnalysisJSON)
	if err != nil {
		return Selection{}, err
	}

	if selection.Mode != "FULL" && selection.Mode != "TARGETED" {
		return Selection{}, fmt.Errorf("invalid review scope mode %q", selection.Mode)
	}
	if selection.Mode == "FULL" {
		if selection.Target != nil {
			return Selection{}, errors.New("FULL review scope must not contain target")
		}
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
	if err != nil {
		return Selection{}, err
	}

	selection.allChangedFiles = make([]string, 0, len(analysis.ChangedFiles))
	for _, f := range analysis.ChangedFiles {
		p := normalizePath(f.Path)
		if p != "" && p != "." {
			selection.allChangedFiles = append(selection.allChangedFiles, p)
		}
	}
	selection.allChangedFiles = uniqueSorted(selection.allChangedFiles)

	if selection.Mode == "FULL" {
		if len(analysis.ResourceRelations) > 0 {
			evidence, err := buildNavigationEvidence(analysis.SymbolLocations)
			if err != nil {
				return Selection{}, err
			}
			if err := validateResourceRelationsBase(analysis.ResourceRelations, changedRoles, evidence); err != nil {
				return Selection{}, err
			}
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
	if err != nil {
		return Selection{}, err
	}
	targetRole, err := resolveTargetRole(*selection.Target, evidence)
	if err != nil {
		return Selection{}, err
	}
	if targetRole == "Controller" {
		if err := verifyAllControllerChains(*selection.Target, selection.SelectedCallChains, analysis.CallChains); err != nil {
			return Selection{}, err
		}
	}

	allowedPaths, requiredPaths, err := exactScopePaths(*selection.Target, selection.SelectedCallChains, evidence, analysis.ResourceRelations, changedRoles)
	if err != nil {
		return Selection{}, err
	}
	selectedPaths := make(map[string]struct{}, len(selection.ScopedFiles))
	for i, f := range selection.ScopedFiles {
		p, err := normalizeScopedPath(f)
		if err != nil {
			return Selection{}, err
		}
		selection.ScopedFiles[i] = p
		if _, ok := allowedPaths[p]; !ok {
			return Selection{}, fmt.Errorf("scoped file %q is not an exact Code Navigation path or evidence-related changed resource for the selected target/call chains", f)
		}
		selectedPaths[p] = struct{}{}
	}
	selection.ScopedFiles = uniqueSorted(selection.ScopedFiles)
	for required := range requiredPaths {
		if _, ok := selectedPaths[required]; ok {
			continue
		}
		if navigationPath(required, evidence) {
			return Selection{}, fmt.Errorf("selected internal symbol exact Code Navigation path %q is missing from scopedFiles", required)
		}
		return Selection{}, fmt.Errorf("evidence-related changed resource path %q is missing from scopedFiles", required)
	}

	return selection, nil
}

func ComputeCoverage(selection Selection, reviewedFiles []string) CoverageResult {
	required := selection.ScopedFiles
	if selection.Mode == "FULL" {
		required = selection.allChangedFiles
	}
	seen := make(map[string]struct{}, len(reviewedFiles))
	for _, f := range reviewedFiles {
		seen[normalizePath(f)] = struct{}{}
	}
	missing := make([]string, 0)
	for _, f := range required {
		p := normalizePath(f)
		if _, ok := seen[p]; !ok {
			missing = append(missing, p)
		}
	}
	missing = uniqueSorted(missing)
	status := "COMPLETE"
	if len(missing) > 0 {
		status = "PARTIAL"
	}
	return CoverageResult{Status: status, MissingFiles: missing}
}

func ComputeCoverageFromAnalysis(selection Selection, changeAnalysisJSON []byte) (CoverageResult, error) {
	analysis, err := parseChangeAnalysis(changeAnalysisJSON)
	if err != nil {
		return CoverageResult{}, err
	}
	if _, err := validateChangedAndReviewedResourceRoles(analysis.ChangedFiles, analysis.ReviewCoverage.ReviewedFiles); err != nil {
		return CoverageResult{}, err
	}
	reviewed := make([]string, 0, len(analysis.ReviewCoverage.ReviewedFiles))
	for _, file := range analysis.ReviewCoverage.ReviewedFiles {
		reviewed = append(reviewed, file.Path)
	}
	result := ComputeCoverage(selection, reviewed)
	result.UnresolvedSymbols = relevantUnresolved(selection, analysis.ReviewCoverage.UnresolvedSymbols)
	if len(result.UnresolvedSymbols) > 0 {
		result.Status = "PARTIAL"
	}
	return result, nil
}

func validateChangedAndReviewedResourceRoles(changed, reviewed []analysisFile) (map[string]string, error) {
	changedRoles := make(map[string]string, len(changed))
	for _, file := range changed {
		p, err := normalizeScopedPath(file.Path)
		if err != nil {
			return nil, fmt.Errorf("changed file: %w", err)
		}
		role := strings.TrimSpace(file.Role)
		if err := validateResourcePathRole(p, role); err != nil {
			return nil, fmt.Errorf("changed file: %w", err)
		}
		if previous, exists := changedRoles[p]; exists && previous != role {
			return nil, fmt.Errorf("changed file %q has conflicting roles %q and %q", p, previous, role)
		}
		changedRoles[p] = role
	}
	for _, file := range reviewed {
		p, err := normalizeScopedPath(file.Path)
		if err != nil {
			return nil, fmt.Errorf("reviewed file: %w", err)
		}
		role := strings.TrimSpace(file.Role)
		changedRole, changedFile := changedRoles[p]
		if changedFile && (isResourceRole(changedRole) || isResourceRole(role) || expectedResourceRole(p) != "") && role != changedRole {
			return nil, fmt.Errorf("reviewed file role %q for %q does not match changed file role %q", role, p, changedRole)
		}
		if err := validateResourcePathRole(p, role); err != nil {
			return nil, fmt.Errorf("reviewed file: %w", err)
		}
	}
	return changedRoles, nil
}

func validateResourcePathRole(value, role string) error {
	expected := expectedResourceRole(value)
	if expected != "" && role != expected {
		return fmt.Errorf("resource path %q must use role %s, got %q", value, expected, role)
	}
	switch role {
	case "MapperXml":
		if expected != "MapperXml" {
			return fmt.Errorf("MapperXml path %q must match src/main/resources/**/*Mapper.xml", value)
		}
	case "YamlConfig":
		if expected != "YamlConfig" {
			return fmt.Errorf("YamlConfig path %q must match src/main/resources/**/*.yml", value)
		}
	}
	return nil
}

func expectedResourceRole(value string) string {
	switch projectpath.Classify(value) {
	case projectpath.MapperXML:
		return "MapperXml"
	case projectpath.YAMLConfig:
		return "YamlConfig"
	default:
		return ""
	}
}

func isResourceRole(role string) bool {
	return role == "MapperXml" || role == "YamlConfig"
}

func buildNavigationEvidence(locations []SymbolLocation) (navigationEvidence, error) {
	if len(locations) == 0 {
		return navigationEvidence{}, errors.New("TARGETED review requires ChangeAnalysis.symbolLocations from Code Navigation")
	}
	e := navigationEvidence{
		bySymbol:      make(map[string]SymbolLocation),
		locations:     make([]SymbolLocation, 0, len(locations)),
		methodSymbols: make(map[string]struct{}),
	}
	for _, raw := range locations {
		symbol := strings.TrimSpace(raw.Symbol)
		if symbol == "" {
			return navigationEvidence{}, errors.New("navigation symbol location requires symbol")
		}
		p, err := normalizeScopedPath(raw.Path)
		if err != nil {
			return navigationEvidence{}, fmt.Errorf("navigation symbol %q: %w", symbol, err)
		}
		loc := raw
		loc.Workspace = effectiveWorkspace(raw.Workspace)
		loc.Symbol = symbol
		loc.Path = p
		loc.Role = strings.TrimSpace(loc.Role)
		loc.From = strings.TrimSpace(loc.From)
		if previous, exists := e.bySymbol[symbol]; exists {
			if previous.Workspace != loc.Workspace || previous.Path != loc.Path || previous.Role != loc.Role {
				return navigationEvidence{}, fmt.Errorf("ambiguous Code Navigation path for symbol %q: %s/%q vs %s/%q", symbol, previous.Workspace, previous.Path, loc.Workspace, loc.Path)
			}
			continue
		}
		e.bySymbol[symbol] = loc
		e.locations = append(e.locations, loc)
	}
	for symbol, loc := range e.bySymbol {
		parent := parentSymbol(symbol)
		if parent == "" {
			continue
		}
		if parentLoc, ok := e.bySymbol[parent]; ok && parentLoc.Workspace == loc.Workspace && parentLoc.Path == loc.Path {
			e.methodSymbols[symbol] = struct{}{}
		}
	}
	return e, nil
}

func navigationPath(value string, evidence navigationEvidence) bool {
	for _, loc := range evidence.locations {
		if loc.Workspace == currentWorkspace && loc.Path == value {
			return true
		}
	}
	return false
}

func resolveTargetRole(target Target, evidence navigationEvidence) (string, error) {
	if target.Kind == "METHOD" {
		loc, ok := evidence.bySymbol[strings.TrimSpace(target.Symbol)]
		if !ok || loc.Workspace != currentWorkspace {
			return "", fmt.Errorf("target %q has no exact current-workspace Code Navigation path evidence", target.Symbol)
		}
		return loc.Role, nil
	}

	targetClass := className(target.Symbol, "CLASS")
	roles := map[string]struct{}{}
	paths := map[string]struct{}{}
	for _, loc := range evidence.locations {
		if loc.Workspace != currentWorkspace {
			continue
		}
		if className(loc.Symbol, "METHOD") != targetClass && className(loc.Symbol, "CLASS") != targetClass {
			continue
		}
		roles[loc.Role] = struct{}{}
		paths[loc.Path] = struct{}{}
	}
	if len(roles) == 0 {
		return "", fmt.Errorf("target %q has no exact current-workspace Code Navigation path evidence", target.Symbol)
	}
	if len(roles) != 1 || len(paths) != 1 {
		return "", fmt.Errorf("target %q has ambiguous Code Navigation role/path evidence", target.Symbol)
	}
	for role := range roles {
		return role, nil
	}
	return "", errors.New("unreachable target role")
}

func exactScopePaths(target Target, chains []CallChain, evidence navigationEvidence, relations []ResourceRelation, changedRoles map[string]string) (map[string]struct{}, map[string]struct{}, error) {
	nodes := selectedNodes(chains)
	allowed := make(map[string]struct{})
	required := make(map[string]struct{})
	selectedNavigationPaths := make(map[string]struct{})
	for node := range nodes {
		loc, ok := evidence.bySymbol[node]
		if !ok {
			return nil, nil, fmt.Errorf("selected internal symbol %q has no exact Code Navigation path evidence", node)
		}
		if loc.Workspace != currentWorkspace {
			continue
		}
		allowed[loc.Path] = struct{}{}
		required[loc.Path] = struct{}{}
		selectedNavigationPaths[loc.Path] = struct{}{}
	}
	for _, loc := range evidence.locations {
		if loc.Workspace != currentWorkspace {
			continue
		}
		_, symbolSelected := nodes[loc.Symbol]
		_, fromSelected := nodes[loc.From]
		targetClassRelated := target.Kind == "CLASS" && className(loc.Symbol, "METHOD") == className(target.Symbol, "CLASS")
		if symbolSelected || fromSelected || targetClassRelated {
			allowed[loc.Path] = struct{}{}
		}
	}

	for _, raw := range relations {
		relation, err := normalizeResourceRelation(raw)
		if err != nil {
			return nil, nil, err
		}
		if err := validateResourceRelationChangedRole(relation, changedRoles); err != nil {
			return nil, nil, err
		}
		fromLocation, err := resolveResourceRelationEvidence(relation, evidence)
		if err != nil {
			return nil, nil, err
		}
		touches, err := resourceRelationTouchesSelected(relation, fromLocation, nodes, selectedNavigationPaths)
		if err != nil {
			return nil, nil, err
		}
		if !touches {
			continue
		}
		allowed[relation.Path] = struct{}{}
		required[relation.Path] = struct{}{}
	}
	return allowed, required, nil
}

func validateResourceRelationsBase(relations []ResourceRelation, changedRoles map[string]string, evidence navigationEvidence) error {
	for _, raw := range relations {
		relation, err := normalizeResourceRelation(raw)
		if err != nil {
			return err
		}
		if err := validateResourceRelationChangedRole(relation, changedRoles); err != nil {
			return err
		}
		if _, err := resolveResourceRelationEvidence(relation, evidence); err != nil {
			return err
		}
	}
	return nil
}

func validateResourceRelationChangedRole(relation ResourceRelation, changedRoles map[string]string) error {
	changedRole, changed := changedRoles[relation.Path]
	if !changed {
		return fmt.Errorf("resource relation path %q is not present in changedFiles", relation.Path)
	}
	if changedRole != relation.Role {
		return fmt.Errorf("resource relation role %q for %q does not match changed file role %q", relation.Role, relation.Path, changedRole)
	}
	return nil
}

func normalizeResourceRelation(raw ResourceRelation) (ResourceRelation, error) {
	relation := raw
	p, err := normalizeScopedPath(raw.Path)
	if err != nil {
		return ResourceRelation{}, fmt.Errorf("resource relation: %w", err)
	}
	relation.Path = p
	relation.Role = strings.TrimSpace(raw.Role)
	relation.Resource = strings.TrimSpace(raw.Resource)
	relation.FromSymbol = strings.TrimSpace(raw.FromSymbol)
	relation.FromKind = strings.TrimSpace(raw.FromKind)
	relation.Source = strings.TrimSpace(raw.Source)
	relation.Evidence = strings.TrimSpace(raw.Evidence)
	if relation.Resource == "" || relation.FromSymbol == "" || relation.Evidence == "" {
		return ResourceRelation{}, fmt.Errorf("resource relation %q requires resource, fromSymbol, and evidence", relation.Path)
	}
	if relation.FromKind != "CLASS" && relation.FromKind != "METHOD" {
		return ResourceRelation{}, fmt.Errorf("resource relation %q has invalid fromKind %q", relation.Path, relation.FromKind)
	}
	if err := validateResourcePathRole(relation.Path, relation.Role); err != nil {
		return ResourceRelation{}, fmt.Errorf("resource relation: %w", err)
	}
	switch relation.Role {
	case "MapperXml":
		if relation.Source != "MAPPER_STATEMENT" {
			return ResourceRelation{}, fmt.Errorf("MapperXml resource relation %q requires MAPPER_STATEMENT source", relation.Path)
		}
	case "YamlConfig":
		if relation.Source != "CONFIG_REFERENCE" {
			return ResourceRelation{}, fmt.Errorf("YamlConfig resource relation %q requires CONFIG_REFERENCE source", relation.Path)
		}
	default:
		return ResourceRelation{}, fmt.Errorf("resource relation %q has unsupported role %q", relation.Path, relation.Role)
	}
	return relation, nil
}

func resolveResourceRelationEvidence(relation ResourceRelation, evidence navigationEvidence) (SymbolLocation, error) {
	loc, ok := evidence.bySymbol[relation.FromSymbol]
	if !ok {
		return SymbolLocation{}, fmt.Errorf("resource relation fromSymbol %q has no exact Code Navigation path evidence", relation.FromSymbol)
	}
	if loc.Workspace != currentWorkspace {
		return SymbolLocation{}, fmt.Errorf("resource relation fromSymbol %q belongs to dependency workspace %q", relation.FromSymbol, loc.Workspace)
	}
	if relation.FromKind == "CLASS" {
		if _, method := evidence.methodSymbols[relation.FromSymbol]; method {
			return SymbolLocation{}, fmt.Errorf("resource relation fromKind=CLASS requires class symbol, got method symbol %q", relation.FromSymbol)
		}
	}
	return loc, nil
}

func resourceRelationTouchesSelected(relation ResourceRelation, fromLocation SymbolLocation, nodes, selectedNavigationPaths map[string]struct{}) (bool, error) {
	if relation.FromKind == "METHOD" {
		if _, selected := nodes[relation.FromSymbol]; !selected {
			return false, nil
		}
		_, selectedPath := selectedNavigationPaths[fromLocation.Path]
		return selectedPath, nil
	}

	classMatched := false
	for node := range nodes {
		if parentSymbol(node) == relation.FromSymbol {
			classMatched = true
			break
		}
	}
	if !classMatched {
		return false, fmt.Errorf("resource relation class %q does not match any selected chain class", relation.FromSymbol)
	}
	_, selectedPath := selectedNavigationPaths[fromLocation.Path]
	return selectedPath, nil
}

func parentSymbol(symbol string) string {
	symbol = strings.TrimSpace(symbol)
	index := strings.LastIndex(symbol, ".")
	if index <= 0 {
		return ""
	}
	return symbol[:index]
}

func verifyAllControllerChains(target Target, selected, all []CallChain) error {
	required := make([]CallChain, 0)
	for _, chain := range all {
		if target.Kind == "METHOD" {
			if strings.TrimSpace(chain.EntryPoint) == strings.TrimSpace(target.Symbol) {
				required = append(required, chain)
			}
			continue
		}
		if className(chain.EntryPoint, "METHOD") == className(target.Symbol, "CLASS") {
			required = append(required, chain)
		}
	}
	if len(required) == 0 {
		return fmt.Errorf("Controller target %q has no confirmed call chains", target.Symbol)
	}
	if !sameChainSet(selected, required) {
		return fmt.Errorf("Controller target %q must include all confirmed Controller chains; selected=%d required=%d", target.Symbol, len(selected), len(required))
	}
	return nil
}

func sameChainSet(a, b []CallChain) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[string]int, len(a))
	for _, chain := range a {
		set[chainKey(chain)]++
	}
	for _, chain := range b {
		key := chainKey(chain)
		if set[key] == 0 {
			return false
		}
		set[key]--
	}
	return true
}

func chainKey(chain CallChain) string {
	parts := []string{strings.TrimSpace(chain.EntryPoint)}
	for _, node := range chain.Chain {
		parts = append(parts, strings.TrimSpace(node))
	}
	return strings.Join(parts, "\x00")
}

func selectedNodes(chains []CallChain) map[string]struct{} {
	nodes := make(map[string]struct{})
	for _, chain := range chains {
		entry := strings.TrimSpace(chain.EntryPoint)
		if entry != "" {
			nodes[entry] = struct{}{}
		}
		for _, node := range chain.Chain {
			node = strings.TrimSpace(node)
			if node != "" {
				nodes[node] = struct{}{}
			}
		}
	}
	return nodes
}

func relevantUnresolved(selection Selection, unresolved []unresolvedSymbol) []string {
	if selection.Mode == "FULL" {
		values := make([]string, 0, len(unresolved))
		for _, item := range unresolved {
			values = append(values, strings.TrimSpace(item.Symbol))
		}
		return uniqueSorted(values)
	}
	nodes := selectedNodes(selection.SelectedCallChains)
	values := make([]string, 0)
	for _, item := range unresolved {
		_, symbolSelected := nodes[strings.TrimSpace(item.Symbol)]
		_, fromSelected := nodes[strings.TrimSpace(item.From)]
		if symbolSelected || fromSelected {
			values = append(values, strings.TrimSpace(item.Symbol))
		}
	}
	return uniqueSorted(values)
}

func parseChangeAnalysis(data []byte) (changeAnalysis, error) {
	var analysis changeAnalysis
	if err := json.Unmarshal(data, &analysis); err != nil {
		return changeAnalysis{}, fmt.Errorf("parse change analysis: %w", err)
	}
	return analysis, nil
}

func decodeSelection(data []byte) (Selection, error) {
	var selection Selection
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&selection); err != nil {
		return Selection{}, fmt.Errorf("parse review scope selection: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err == nil {
		return Selection{}, errors.New("parse review scope selection: multiple JSON values are not allowed")
	}
	return selection, nil
}

func containsChain(chains []CallChain, selected CallChain) bool {
	for _, candidate := range chains {
		if strings.TrimSpace(candidate.EntryPoint) != strings.TrimSpace(selected.EntryPoint) || len(candidate.Chain) != len(selected.Chain) {
			continue
		}
		equal := true
		for i := range candidate.Chain {
			if strings.TrimSpace(candidate.Chain[i]) != strings.TrimSpace(selected.Chain[i]) {
				equal = false
				break
			}
		}
		if equal {
			return true
		}
	}
	return false
}

func targetInChains(target Target, chains []CallChain) bool {
	for _, chain := range chains {
		nodes := append([]string{chain.EntryPoint}, chain.Chain...)
		for _, node := range nodes {
			if target.Kind == "METHOD" && strings.TrimSpace(node) == strings.TrimSpace(target.Symbol) {
				return true
			}
			if target.Kind == "CLASS" && className(node, "METHOD") == className(target.Symbol, "CLASS") {
				return true
			}
		}
	}
	return false
}

func className(symbol, kind string) string {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return ""
	}
	parts := strings.Split(symbol, ".")
	if kind == "METHOD" && len(parts) >= 2 {
		return parts[len(parts)-2]
	}
	return parts[len(parts)-1]
}

func effectiveWorkspace(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return currentWorkspace
	}
	return value
}

func normalizeScopedPath(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" || strings.HasPrefix(value, "/") || strings.Contains(value, ":") {
		return "", fmt.Errorf("scoped file %q must be repository-relative", value)
	}
	clean := path.Clean(value)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("scoped file %q escapes repository root", value)
	}
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
		if value == "" || value == "." {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
