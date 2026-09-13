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

	analysisPath := filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json"))
	analysisValue, _, err := analysisruntime.LoadCertified(".", analysisPath)
	if err != nil {
		return fmt.Errorf("REVIEW_CONTEXT_ANALYSIS_UNAVAILABLE: %w", err)
	}
	resolver, err := newRuntimeResolver170(".")
	if err != nil {
		return err
	}
	buildInput := reviewcontext.BuildInput170{RunID: runID, Phase: req.Phase, Budget: reviewcontext.DefaultBudget170()}
	buildInput.Seeds = seedsFromAnalysis170(analysisValue)

	var unitsPath, dispatchPath string
	if req.Phase == "RULES" {
		discoveryPath := filepath.Join(".code-harness", "runs", runID, "analysis", "review-context-discovery.json")
		if _, err := os.Stat(discoveryPath); err != nil {
			return fmt.Errorf("REVIEW_CONTEXT_DISCOVERY_UNAVAILABLE: %w", err)
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
	artifactPath := filepath.Join(".code-harness", "runs", runID, "analysis", "review-context-"+strings.ToLower(req.Phase)+".json")
	if err := atomicReviewWrite153(artifactPath, encoded); err != nil {
		return fmt.Errorf("REVIEW_CONTEXT_WRITE_FAILED: %w", err)
	}
	if req.Phase == "RULES" {
		if err := advanceReviewProgressStage164(runID, reviewprogress.StageReviewPlanning, unitsPath, dispatchPath, filepath.ToSlash(artifactPath)); err != nil {
			return err
		}
	}
	return writeJSONAndStatus(map[string]any{"status": "READY", "runId": runID, "phase": req.Phase, "artifactPath": filepath.ToSlash(artifactPath), "context": built}, true)
}

type runtimeResolver170 struct {
	root       string
	navigators map[string]nav.Navigator
	providers  []reviewcontext.ProviderRoot170
}

func newRuntimeResolver170(root string) (*runtimeResolver170, error) {
	ast := filepath.Join(root, ".code-harness", "bin", "ast-grep.exe")
	r := &runtimeResolver170{root: root, navigators: map[string]nav.Navigator{"current": {RepoRoot: root, AstGrepPath: ast}}, providers: []reviewcontext.ProviderRoot170{}}
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
func (r *runtimeResolver170) Callers(context.Context, nav.ReviewRef170) ([]nav.Relation170, error) {
	// T2 exposes exact forward call resolution. Upstream expansion remains
	// fail-closed until a caller candidate can be re-proven as an exact forward
	// edge; never manufacture a caller relation from name-only legacy search.
	return []nav.Relation170{}, nil
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
		seeds := []nav.ReviewRef170{}
		for i, symbol := range u.Chain {
			p := ""
			workspaceID := "current"
			if i < len(u.Files) {
				p = u.Files[i].Path
				workspaceID = u.Files[i].Workspace
			}
			if p == "" {
				for _, loc := range a.SymbolLocations {
					if loc.Symbol == symbol {
						p = loc.Path
						workspaceID = loc.Workspace
						break
					}
				}
			}
			if ref, ok := methodRefFromSymbol170(workspaceID, p, symbol); ok {
				seeds = append(seeds, ref)
			}
		}
		needs = append(needs, reviewcontext.Need170{ReviewUnitID: d.ReviewUnitID, RuleID: d.RuleID, Seeds: seeds, RequiredKinds: requiredKinds170(d.RequiredEvidence)})
	}
	return needs
}

func requiredKinds170(evidence []string) []string {
	set := map[string]bool{}
	for _, raw := range evidence {
		s := strings.ToUpper(strings.TrimSpace(raw))
		switch {
		case strings.Contains(s, "DUBBO") || strings.Contains(s, "PROVIDER"):
			set["DUBBO_CONTRACT"] = true
		case strings.Contains(s, "SQL") || strings.Contains(s, "MAPPER"):
			set["MYBATIS_STATEMENT"] = true
		case strings.Contains(s, "SPRING") || strings.Contains(s, "BEAN") || strings.Contains(s, "PROXY"):
			set["SPRING_BINDING"] = true
		case strings.Contains(s, "CALL"):
			set["JAVA_CALL"] = true
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
