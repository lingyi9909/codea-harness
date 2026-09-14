package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/changeset"
	"codea-harness-tools/internal/nav"
	"codea-harness-tools/internal/reviewcontext"
	"codea-harness-tools/internal/reviewprogress"
	"codea-harness-tools/internal/reviewrules"
	"codea-harness-tools/internal/reviewunit"
	"codea-harness-tools/internal/workspace"
)

type reviewContextRequest170 struct {
	RunID string `json:"runId"`
	Phase string `json:"phase"`
}

func decodeReviewContextRequest170(pathRunID string, raw []byte) (reviewContextRequest170, error) {
	var req reviewContextRequest170
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return req, fmt.Errorf("REVIEW_CONTEXT_REQUEST_INVALID: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return req, fmt.Errorf("REVIEW_CONTEXT_REQUEST_INVALID: trailing JSON")
	}
	req.RunID = strings.TrimSpace(req.RunID)
	if req.RunID == "" || req.RunID != strings.TrimSpace(pathRunID) {
		return req, fmt.Errorf("RUN_ID_MISMATCH: body=%q path=%q", req.RunID, pathRunID)
	}
	req.Phase = strings.ToUpper(strings.TrimSpace(req.Phase))
	if req.Phase != "DISCOVERY" && req.Phase != "RULES" {
		return req, fmt.Errorf("REVIEW_CONTEXT_PHASE_INVALID: %q", req.Phase)
	}
	return req, nil
}

func validateReviewContextProgress170(state reviewprogress.State, phase string) error {
	if state.Status != reviewprogress.StatusRunning {
		return fmt.Errorf("REVIEW_CONTEXT_PROGRESS_TERMINAL: %s", state.Status)
	}
	switch strings.ToUpper(strings.TrimSpace(phase)) {
	case "DISCOVERY":
		if state.CurrentStage != reviewprogress.StageChangeAnalysis {
			return fmt.Errorf("REVIEW_CONTEXT_PHASE_ILLEGAL: DISCOVERY requires CHANGE_ANALYSIS, current=%s", state.CurrentStage)
		}
	case "RULES":
		if state.CurrentStage != reviewprogress.StageReviewPlanning {
			return fmt.Errorf("REVIEW_CONTEXT_PHASE_ILLEGAL: RULES requires REVIEW_PLANNING, current=%s", state.CurrentStage)
		}
	default:
		return fmt.Errorf("REVIEW_CONTEXT_PHASE_INVALID: %q", phase)
	}
	return nil
}

func reviewContextAuthority170(runID, phase string) (string, bool, error) {
	phase = strings.ToUpper(strings.TrimSpace(phase))
	switch phase {
	case "DISCOVERY":
		return filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "analysis", "change-set.json")), false, nil
	case "RULES":
		return filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json")), true, nil
	default:
		return "", false, fmt.Errorf("REVIEW_CONTEXT_PHASE_INVALID: %q", phase)
	}
}

func reviewContextArtifactPath170(runID, phase string) string {
	name := "review-call-context.json"
	if strings.EqualFold(strings.TrimSpace(phase), "RULES") {
		name = "review-rule-context.json"
	}
	return filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "analysis", name))
}

func loadFreshChangeSet170(runID string) (changeset.Snapshot, error) {
	path, _, err := reviewContextAuthority170(runID, "DISCOVERY")
	if err != nil {
		return changeset.Snapshot{}, err
	}
	raw, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		return changeset.Snapshot{}, fmt.Errorf("REVIEW_CONTEXT_CHANGESET_UNAVAILABLE: %w", err)
	}
	var snap changeset.Snapshot
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&snap); err != nil {
		return snap, fmt.Errorf("REVIEW_CONTEXT_CHANGESET_INVALID: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return snap, fmt.Errorf("REVIEW_CONTEXT_CHANGESET_INVALID: trailing JSON")
	}
	fresh, err := changeset.Compute(".", snap.RequestedBaseRef, snap.IncludeWorkingTree)
	if err != nil {
		return snap, fmt.Errorf("REVIEW_CONTEXT_CHANGESET_STALE: %w", err)
	}
	if fresh.SnapshotSHA256 != snap.SnapshotSHA256 || fresh.HeadCommit != snap.HeadCommit || fresh.MergeBase != snap.MergeBase || fresh.ResolvedBaseCommit != snap.ResolvedBaseCommit {
		return snap, fmt.Errorf("REVIEW_CONTEXT_CHANGESET_STALE: current ChangeSet identity changed")
	}
	return snap, nil
}

func currentJavaChangePaths170(snap changeset.Snapshot) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, file := range snap.Files {
		p := filepath.ToSlash(filepath.Clean(file.Path))
		if !strings.HasSuffix(strings.ToLower(p), ".java") {
			continue
		}
		if _, err := os.Stat(filepath.FromSlash(p)); err != nil {
			continue
		}
		key := strings.ToLower(p)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func resourcesForSeeds170(ctx context.Context, n nav.Navigator, seeds []nav.ReviewRef170) ([]nav.SourceRange170, error) {
	paths := []string{}
	seen := map[string]bool{}
	wanted := map[string]bool{}
	for _, seed := range seeds {
		key, err := nav.ReviewRefKey170(seed)
		if err != nil {
			return nil, err
		}
		wanted[key] = true
		p := filepath.ToSlash(seed.Path)
		pk := strings.ToLower(p)
		if !seen[pk] {
			seen[pk] = true
			paths = append(paths, p)
		}
	}
	refs, ranges, err := n.DiscoverMethods170(ctx, paths)
	if err != nil {
		return nil, err
	}
	out := []nav.SourceRange170{}
	for i, ref := range refs {
		key, _ := nav.ReviewRefKey170(ref)
		if wanted[key] && i < len(ranges) {
			out = append(out, ranges[i])
		}
	}
	return out, nil
}

func runReviewContext170(args []string) error {
	fs := flag.NewFlagSet("review context", flag.ContinueOnError)
	inputPath := fs.String("input", "", "same-run review context request under .code-harness/runs/<runId>/requests")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || strings.TrimSpace(*inputPath) == "" {
		return errors.New("review context requires --input")
	}
	runID, cleanInput, err := validateAnalysisRequestPath153(*inputPath)
	if err != nil {
		return errors.New("review context input must be under .code-harness/runs/<runId>/requests")
	}
	if err := verifyReviewTransportPath153(runID, cleanInput); err != nil {
		return err
	}
	raw, err := os.ReadFile(cleanInput)
	if err != nil {
		return fmt.Errorf("REVIEW_CONTEXT_REQUEST_READ_FAILED: %w", err)
	}
	req, err := decodeReviewContextRequest170(runID, raw)
	if err != nil {
		return err
	}
	state, err := reviewprogress.Read(".", runID)
	if err != nil {
		return err
	}
	if err := validateReviewContextProgress170(state, req.Phase); err != nil {
		return err
	}
	resolver, err := newRuntimeResolver170(".")
	if err != nil {
		return err
	}
	buildInput := reviewcontext.BuildInput170{RunID: runID, Phase: req.Phase, Budget: reviewcontext.DefaultBudget170()}
	var unitsPath, dispatchPath string
	if req.Phase == "DISCOVERY" {
		snap, err := loadFreshChangeSet170(runID)
		if err != nil {
			return err
		}
		methods, ranges, err := resolver.navigators["current"].DiscoverMethods170(context.Background(), currentJavaChangePaths170(snap))
		if err != nil {
			return fmt.Errorf("REVIEW_CONTEXT_DISCOVERY_METHODS_FAILED: %w", err)
		}
		buildInput.Seeds = methods
		buildInput.Resources = ranges
		cleanup, baseErr := addBaseInputs170(context.Background(), snap, resolver, &buildInput)
		if baseErr != nil {
			return fmt.Errorf("REVIEW_CONTEXT_BASE_SOURCE_UNAVAILABLE: %w", baseErr)
		}
		defer cleanup()
	} else {
		if _, err := os.Stat(filepath.FromSlash(reviewContextArtifactPath170(runID, "DISCOVERY"))); err != nil {
			return fmt.Errorf("REVIEW_CONTEXT_DISCOVERY_UNAVAILABLE: %w", err)
		}
		analysisPath, _, _ := reviewContextAuthority170(runID, "RULES")
		analysisValue, _, err := analysisruntime.LoadCertified(".", analysisPath)
		if err != nil {
			return fmt.Errorf("REVIEW_CONTEXT_ANALYSIS_UNAVAILABLE: %w", err)
		}
		buildInput.Seeds = seedsFromAnalysis170(analysisValue)
		buildInput.Resources, err = resourcesForSeeds170(context.Background(), resolver.navigators["current"], buildInput.Seeds)
		if err != nil {
			return fmt.Errorf("REVIEW_CONTEXT_RULE_RESOURCES_FAILED: %w", err)
		}
		unitsPath = filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "analysis", "review-units.json"))
		dispatchPath = filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "analysis", "rule-dispatch.json"))
		units, err := reviewunit.Load(reviewunit.BuildInput{RunID: runID, CertifiedRunID: runID, RepoRoot: "."})
		if err != nil {
			return fmt.Errorf("REVIEW_CONTEXT_RULES_UNITS_UNAVAILABLE: %w", err)
		}
		dispatch, err := loadRuleDispatch170(dispatchPath)
		if err != nil {
			return err
		}
		if dispatch.RunID != runID || dispatch.ReviewUnitsSHA256 != units.SHA256 {
			return fmt.Errorf("REVIEW_CONTEXT_RULES_DISPATCH_STALE")
		}
		buildInput.Needs = needsFromDispatch170(units, dispatch, analysisValue)
		snap, snapErr := loadFreshChangeSet170(runID)
		if snapErr != nil {
			return snapErr
		}
		cleanup, baseErr := addBaseInputs170(context.Background(), snap, resolver, &buildInput)
		if baseErr != nil {
			return fmt.Errorf("REVIEW_CONTEXT_BASE_SOURCE_UNAVAILABLE: %w", baseErr)
		}
		defer cleanup()
	}
	built, err := reviewcontext.Build170(context.Background(), buildInput, resolver)
	if err != nil {
		return err
	}
	if err := reviewcontext.VerifyRelations170(context.Background(), built, buildInput, resolver); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(built, "", "  ")
	if err != nil {
		return fmt.Errorf("REVIEW_CONTEXT_ENCODE_FAILED: %w", err)
	}
	encoded = append(encoded, '\n')
	artifactPath := reviewContextArtifactPath170(runID, req.Phase)
	if err := atomicReviewWrite153(filepath.FromSlash(artifactPath), encoded); err != nil {
		return fmt.Errorf("REVIEW_CONTEXT_WRITE_FAILED: %w", err)
	}
	if req.Phase == "RULES" {
		if err := advanceReviewProgressStage164(runID, reviewprogress.StageReviewPlanning, unitsPath, dispatchPath, artifactPath); err != nil {
			return err
		}
	}
	return writeJSONAndStatus(map[string]any{"status": "READY", "runId": runID, "phase": req.Phase, "artifactPath": artifactPath, "context": built}, true)
}

type runtimeResolver170 struct {
	root        string
	astGrepPath string
	navigators  map[string]nav.Navigator
	providers   []reviewcontext.ProviderRoot170
}

func newRuntimeResolver170(root string) (*runtimeResolver170, error) {
	ast := filepath.Join(root, ".code-harness", "bin", "ast-grep.exe")
	r := &runtimeResolver170{root: root, astGrepPath: ast, navigators: map[string]nav.Navigator{"current": {RepoRoot: root, AstGrepPath: ast}}, providers: []reviewcontext.ProviderRoot170{}}
	configPath := filepath.Join(root, ".code-harness", "harness.yaml")
	raw, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return r, nil
	}
	if err != nil {
		return nil, fmt.Errorf("REVIEW_CONTEXT_WORKSPACE_CONFIG_READ_FAILED: %w", err)
	}
	deps, err := workspace.ValidateConfigYAML(root, raw)
	if err != nil {
		return nil, err
	}
	verified := workspace.VerifyDirectMavenDependencies(root, deps)
	r.providers = reviewcontext.ProviderRootsFromVerification170(verified)
	for _, result := range verified {
		if result.Status != workspace.StatusVerified || strings.TrimSpace(result.ConfirmedRoot) == "" {
			continue
		}
		r.navigators[result.DependencyID] = nav.Navigator{RepoRoot: result.ConfirmedRoot, AstGrepPath: ast}
	}
	return r, nil
}

func (r *runtimeResolver170) navigator(ref nav.ReviewRef170) (nav.Navigator, error) {
	if ref.Side == "BASE" {
		if n, ok := r.navigators["base"]; ok {
			return n, nil
		}
		return nav.Navigator{}, fmt.Errorf("BASE_SOURCE_UNAVAILABLE: base source adapter is not prepared")
	}
	n, ok := r.navigators[ref.Workspace]
	if !ok {
		return nav.Navigator{}, fmt.Errorf("PROVIDER_SOURCE_UNAVAILABLE: workspace %s is not verified", ref.Workspace)
	}
	return n, nil
}
func (r *runtimeResolver170) Method(ctx context.Context, ref nav.ReviewRef170) (nav.MethodFacts170, error) {
	n, err := r.navigator(ref)
	if err != nil {
		return nav.MethodFacts170{}, err
	}
	return n.InspectMethod170(ctx, ref)
}
func (r *runtimeResolver170) Callers(ctx context.Context, ref nav.ReviewRef170) ([]nav.Relation170, error) {
	n, err := r.navigator(ref)
	if err != nil {
		return nil, err
	}
	return n.ExactCallers170(ctx, ref)
}
func (r *runtimeResolver170) CallersBounded(ctx context.Context, ref nav.ReviewRef170, maxCandidates int) ([]nav.Relation170, int, bool, error) {
	n, err := r.navigator(ref)
	if err != nil {
		return nil, 0, false, err
	}
	return n.ExactCallersBounded170(ctx, ref, maxCandidates)
}
func (r *runtimeResolver170) Spring(ctx context.Context, ref nav.ReviewRef170) ([]nav.Relation170, []nav.Issue170, error) {
	n, err := r.navigator(ref)
	if err != nil {
		return nil, nil, err
	}
	return n.ResolveSpringMethodContext170(ctx, ref)
}
func (r *runtimeResolver170) SourceBytes(_ context.Context, ref nav.ReviewRef170) (int, error) {
	n, err := r.navigator(ref)
	if err != nil {
		return 0, err
	}
	root, err := filepath.Abs(n.RepoRoot)
	if err != nil {
		return 0, err
	}
	candidate, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(ref.Path)))
	if err != nil {
		return 0, err
	}
	rel, err := filepath.Rel(root, candidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return 0, fmt.Errorf("REVIEW_CONTEXT_SOURCE_PATH_INVALID: %s", ref.Path)
	}
	info, err := os.Lstat(candidate)
	if err != nil {
		return 0, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return 0, fmt.Errorf("REVIEW_CONTEXT_SOURCE_PATH_INVALID: %s", ref.Path)
	}
	return int(info.Size()), nil
}
func (r *runtimeResolver170) Mapper(ctx context.Context, source nav.SourceRange170) ([]nav.Relation170, []nav.Issue170, error) {
	n, err := r.navigator(source.Ref)
	if err != nil {
		return nil, nil, err
	}
	return reviewcontext.ResolveMapper170(ctx, n.RepoRoot, source)
}
func (r *runtimeResolver170) Dubbo(ctx context.Context, ref nav.ReviewRef170) ([]nav.Relation170, []nav.Issue170, error) {
	n, err := r.navigator(ref)
	if err != nil {
		return nil, nil, err
	}
	return reviewcontext.ResolveDubbo170(ctx, n.RepoRoot, ref, r.providers)
}

func loadRuleDispatch170(path string) (reviewrules.Manifest, error) {
	raw, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		return reviewrules.Manifest{}, fmt.Errorf("REVIEW_CONTEXT_RULES_DISPATCH_UNAVAILABLE: %w", err)
	}
	var out reviewrules.Manifest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return out, fmt.Errorf("REVIEW_CONTEXT_RULES_DISPATCH_INVALID: %w", err)
	}
	return out, nil
}

func seedsFromAnalysis170(a analysisruntime.ChangeAnalysis) []nav.ReviewRef170 {
	refs := []nav.ReviewRef170{}
	seen := map[string]bool{}
	add := func(workspaceID, pathValue, symbol string) {
		if workspaceID == "" {
			workspaceID = "current"
		}
		if workspaceID != "current" || !strings.HasSuffix(strings.ToLower(pathValue), ".java") {
			return
		}
		ref, ok := methodRefFromSymbol170(workspaceID, pathValue, symbol)
		if !ok {
			return
		}
		key, err := nav.ReviewRefKey170(ref)
		if err != nil || seen[key] {
			return
		}
		seen[key] = true
		refs = append(refs, ref)
	}
	for _, chain := range a.CallChains {
		for i, symbol := range chain.Chain {
			if i < len(chain.ChainRefs) {
				add(chain.ChainRefs[i].Workspace, chain.ChainRefs[i].Path, symbol)
			}
		}
	}
	for _, loc := range a.SymbolLocations {
		add(loc.Workspace, loc.Path, loc.Symbol)
	}
	sort.Slice(refs, func(i, j int) bool {
		ki, _ := nav.ReviewRefKey170(refs[i])
		kj, _ := nav.ReviewRefKey170(refs[j])
		return ki < kj
	})
	return refs
}

var package170 = regexp.MustCompile(`(?m)^\s*package\s+([A-Za-z_$][A-Za-z0-9_$.]*)\s*;`)

func methodRefFromSymbol170(workspaceID, p, symbol string) (nav.ReviewRef170, bool) {
	i := strings.LastIndex(strings.TrimSpace(symbol), ".")
	if i <= 0 || i+1 >= len(symbol) {
		return nav.ReviewRef170{}, false
	}
	owner, name := symbol[:i], symbol[i+1:]
	ownerSimple := owner
	if j := strings.LastIndex(ownerSimple, "."); j >= 0 {
		ownerSimple = ownerSimple[j+1:]
	}
	ownerFQCN := owner
	if workspaceID == "current" && !strings.Contains(owner, ".") {
		if raw, err := os.ReadFile(filepath.FromSlash(p)); err == nil {
			if m := package170.FindSubmatch(raw); len(m) == 2 {
				ownerFQCN = string(m[1]) + "." + ownerSimple
			}
		}
	}
	return nav.ReviewRef170{Workspace: workspaceID, Path: p, Side: "CURRENT", Kind: "METHOD", OwnerFQCN: ownerFQCN, Name: name, ParameterTypes: []string{}}, true
}

func needsFromDispatch170(units reviewunit.Manifest, dispatch reviewrules.Manifest, a analysisruntime.ChangeAnalysis) []reviewcontext.Need170 {
	unitByID := map[string]reviewunit.Unit{}
	for _, u := range units.Units {
		unitByID[u.ID] = u
	}
	needs := []reviewcontext.Need170{}
	for _, d := range dispatch.Dispatches {
		u, ok := unitByID[d.ReviewUnitID]
		if !ok {
			continue
		}
		needs = append(needs, reviewcontext.Need170{ReviewUnitID: d.ReviewUnitID, RuleID: d.RuleID, Seeds: seedsForUnit170(u, a), RequiredKinds: requiredKindsForDispatch170(d)})
	}
	return needs
}

func seedsForUnit170(u reviewunit.Unit, a analysisruntime.ChangeAnalysis) []nav.ReviewRef170 {
	wantedSymbols := map[string]bool{}
	for _, symbol := range u.Chain {
		wantedSymbols[strings.TrimSpace(symbol)] = true
	}
	if strings.TrimSpace(u.EntryPoint) != "" {
		wantedSymbols[strings.TrimSpace(u.EntryPoint)] = true
	}
	for _, symbol := range u.ContextSymbols {
		wantedSymbols[strings.TrimSpace(symbol)] = true
	}
	unitFiles := map[string]bool{}
	for _, f := range u.Files {
		unitFiles[strings.ToLower(filepath.ToSlash(f.Path))] = true
	}
	refs := []nav.ReviewRef170{}
	seen := map[string]bool{}
	add := func(workspaceID, p, symbol string) {
		if workspaceID == "" {
			workspaceID = "current"
		}
		if workspaceID != "current" || !strings.HasSuffix(strings.ToLower(p), ".java") {
			return
		}
		ref, ok := methodRefFromSymbol170(workspaceID, p, symbol)
		if !ok {
			return
		}
		key, err := nav.ReviewRefKey170(ref)
		if err != nil || seen[key] {
			return
		}
		seen[key] = true
		refs = append(refs, ref)
	}
	for _, chain := range a.CallChains {
		for i, symbol := range chain.Chain {
			if !wantedSymbols[strings.TrimSpace(symbol)] || i >= len(chain.ChainRefs) {
				continue
			}
			ref := chain.ChainRefs[i]
			add(ref.Workspace, ref.Path, symbol)
		}
	}
	for _, loc := range a.SymbolLocations {
		if wantedSymbols[strings.TrimSpace(loc.Symbol)] || unitFiles[strings.ToLower(filepath.ToSlash(loc.Path))] {
			add(loc.Workspace, loc.Path, loc.Symbol)
		}
	}
	// Mapper XML ReviewUnits may contain no Java chain. Use certified resource
	// relations to reach the mapper symbol; never bind Chain[i] to Files[i].
	for _, rr := range a.ResourceRelations {
		if !unitFiles[strings.ToLower(filepath.ToSlash(rr.Path))] {
			continue
		}
		for _, loc := range a.SymbolLocations {
			if strings.TrimSpace(loc.Symbol) == strings.TrimSpace(rr.FromSymbol) {
				add(loc.Workspace, loc.Path, loc.Symbol)
			}
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		a, _ := nav.ReviewRefKey170(refs[i])
		b, _ := nav.ReviewRefKey170(refs[j])
		return a < b
	})
	return refs
}

func requiredKindsForDispatch170(d reviewrules.Dispatch) []string {
	roles := map[string]bool{}
	for _, reason := range d.DispatchReason {
		const prefix = "CHANGED_ROLE:"
		if strings.HasPrefix(reason, prefix) {
			roles[strings.TrimSpace(strings.TrimPrefix(reason, prefix))] = true
		}
	}
	evidence := map[string]bool{}
	for _, raw := range d.RequiredEvidence {
		evidence[strings.ToUpper(strings.TrimSpace(raw))] = true
	}
	set := map[string]bool{}
	known := map[string]bool{"CHANGED_RANGE": true, "RESOURCE_RELATION": true, "SYMBOL": true, "CHAIN": true, "PROVIDER_SOURCE": true, "DUBBO_CONTRACT": true}
	for e := range evidence {
		if !known[e] {
			set["UNSUPPORTED_CONTEXT_RELATION"] = true
		}
	}
	if evidence["RESOURCE_RELATION"] {
		if roles["Mapper"] || roles["MapperXml"] {
			set["MYBATIS_STATEMENT"] = true
		} else {
			set["UNSUPPORTED_CONTEXT_RELATION"] = true
		}
	}
	if evidence["CHAIN"] {
		set["JAVA_CALL"] = true
		if strings.HasPrefix(d.RuleID, "SPRING-TX-") && roles["Service"] && evidence["SYMBOL"] {
			set["SPRING_BINDING"] = true
		}
	}
	if evidence["PROVIDER_SOURCE"] || evidence["DUBBO_CONTRACT"] {
		set["DUBBO_CONTRACT"] = true
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func requiredKinds170(evidence []string) []string {
	return requiredKindsForDispatch170(reviewrules.Dispatch{RequiredEvidence: evidence})
}

type baseSourceContext170 struct {
	Root      string
	Navigator nav.Navigator
}

func prepareBaseSource170(ctx context.Context, repoRoot string, snap changeset.Snapshot, paths []string, astPath string) (baseSourceContext170, func(), error) {
	clean := []string{}
	seen := map[string]bool{}
	for _, raw := range paths {
		p := filepath.ToSlash(filepath.Clean(raw))
		if p == "." || strings.HasPrefix(p, "../") || seen[strings.ToLower(p)] {
			continue
		}
		seen[strings.ToLower(p)] = true
		clean = append(clean, p)
	}
	sort.Strings(clean)
	sources, err := analysisruntime.ReadBaseSources170(ctx, repoRoot, snap.MergeBase, clean)
	if err != nil {
		return baseSourceContext170{}, func() {}, err
	}
	root, err := os.MkdirTemp("", "codea-harness-review-base-170-")
	if err != nil {
		return baseSourceContext170{}, func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(root) }
	for p, data := range sources {
		target := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			cleanup()
			return baseSourceContext170{}, func() {}, err
		}
		if err := os.WriteFile(target, data, 0o600); err != nil {
			cleanup()
			return baseSourceContext170{}, func() {}, err
		}
	}
	return baseSourceContext170{Root: root, Navigator: nav.Navigator{RepoRoot: root, AstGrepPath: astPath}}, cleanup, nil
}

func basePaths170(snap changeset.Snapshot, currentSeeds []nav.ReviewRef170) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(p string) {
		p = filepath.ToSlash(filepath.Clean(p))
		if p == "." || strings.HasPrefix(p, "../") || seen[strings.ToLower(p)] {
			return
		}
		seen[strings.ToLower(p)] = true
		out = append(out, p)
	}
	for _, f := range snap.Files {
		add(f.Path)
	}
	for _, s := range currentSeeds {
		add(s.Path)
	}
	sort.Strings(out)
	return out
}

func addBaseInputs170(ctx context.Context, snap changeset.Snapshot, resolver *runtimeResolver170, input *reviewcontext.BuildInput170) (func(), error) {
	paths := basePaths170(snap, input.Seeds)
	baseCtx, cleanup, err := prepareBaseSource170(ctx, resolver.root, snap, paths, resolver.astGrepPath)
	if err != nil {
		return func() {}, err
	}
	resolver.navigators["base"] = baseCtx.Navigator
	javaPaths := []string{}
	for _, p := range paths {
		if strings.HasSuffix(strings.ToLower(p), ".java") {
			javaPaths = append(javaPaths, p)
		}
	}
	baseSeeds, baseRanges, err := baseCtx.Navigator.DiscoverMethodsForSide170(ctx, javaPaths, "current", "BASE")
	if err != nil && !errors.Is(err, nav.ErrSymbolNotFound) {
		cleanup()
		return func() {}, err
	}
	input.Seeds = append(input.Seeds, baseSeeds...)
	input.Resources = append(input.Resources, baseRanges...)
	return cleanup, nil
}

func loadReviewContextArtifact170(runID, phase string) (reviewcontext.Context170, error) {
	path := reviewContextArtifactPath170(runID, phase)
	raw, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		return reviewcontext.Context170{}, fmt.Errorf("CONTEXT_RELATION_NOT_VERIFIED: read: %w", err)
	}
	var out reviewcontext.Context170
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return out, fmt.Errorf("CONTEXT_RELATION_NOT_VERIFIED: decode: %w", err)
	}
	for _, rel := range out.Relations {
		if err := nav.ValidateRelation170(rel); err != nil {
			return out, fmt.Errorf("CONTEXT_RELATION_NOT_VERIFIED: %w", err)
		}
	}
	return out, nil
}

func verifyReviewContextUse170(ctx context.Context, claimed reviewcontext.Context170, input reviewcontext.BuildInput170, resolver reviewcontext.Resolver170) error {
	return reviewcontext.VerifyRelations170(ctx, claimed, input, resolver)
}

func verifyReviewContextArtifactUse170(runID string, a analysisruntime.ChangeAnalysis, units reviewunit.Manifest, dispatch reviewrules.Manifest) error {
	state, err := reviewprogress.Read(".", runID)
	if err != nil {
		// Pre-1.7 Reviewer certification has no Runtime-owned progress artifact.
		// Only that absence is legacy. Existing-but-invalid progress must remain
		// fail-closed so a 1.7 run can never downgrade through corruption.
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if state.ProtocolVersion != reviewprogress.Protocol170 {
		return nil
	}
	claimed, err := loadReviewContextArtifact170(runID, "RULES")
	if err != nil {
		return err
	}
	resolver, err := newRuntimeResolver170(".")
	if err != nil {
		return err
	}
	input := reviewcontext.BuildInput170{RunID: runID, Phase: "RULES", Budget: reviewcontext.DefaultBudget170(), Seeds: seedsFromAnalysis170(a)}
	input.Resources, err = resourcesForSeeds170(context.Background(), resolver.navigators["current"], input.Seeds)
	if err != nil {
		return err
	}
	input.Needs = needsFromDispatch170(units, dispatch, a)
	snap, err := loadFreshChangeSet170(runID)
	if err != nil {
		return err
	}
	cleanup, err := addBaseInputs170(context.Background(), snap, resolver, &input)
	if err != nil {
		return err
	}
	defer cleanup()
	if err := verifyReviewContextUse170(context.Background(), claimed, input, resolver); err != nil {
		return err
	}
	return nil
}
