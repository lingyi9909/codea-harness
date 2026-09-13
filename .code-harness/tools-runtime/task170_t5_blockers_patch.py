from pathlib import Path
import re

ROOT = Path('.code-harness/tools-runtime')

def read(rel):
    return (ROOT / rel).read_text(encoding='utf-8')

def write(rel, text):
    p = ROOT / rel
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(text, encoding='utf-8', newline='\n')

def replace_once(text, old, new, label):
    if old not in text:
        raise SystemExit(f'missing replacement anchor: {label}')
    if text.count(old) != 1:
        raise SystemExit(f'non-unique replacement anchor: {label} count={text.count(old)}')
    return text.replace(old, new, 1)

# 1. Runtime-owned protocol identity in review progress.
p = read('internal/reviewprogress/progress.go')
p = replace_once(p, 'const (\n\tStatusPending   = "PENDING"', 'const (\n\tProtocol170 = "1.7"\n)\n\nconst (\n\tStatusPending   = "PENDING"', 'protocol constant')
p = replace_once(p, 'type State struct {\n\tVersion       int', 'type State struct {\n\tVersion       int          `json:"version"`\n\tProtocolVersion string      `json:"protocolVersion,omitempty"`\n\tRunID         string       `json:"runId"`\n\tStatus        string       `json:"status"`\n\tCurrentStage  string       `json:"currentStage"`\n\tTerminalStage string       `json:"terminalStage,omitempty"`\n\tFailureStage  string       `json:"failureStage,omitempty"`\n\tFailureCode   string       `json:"failureCode,omitempty"`\n\tStartedAt     string       `json:"startedAt"`\n\tUpdatedAt     string       `json:"updatedAt"`\n\tStages        []StageState `json:"stages"`\n\tEvents        []Event      `json:"events"`\n}\n\n/*STATE_REPLACED*/\ntype _obsoleteState170 struct {\n\tVersion       int', 'state protocol field')
# Remove the old duplicated state body introduced behind sentinel.
start = p.index('/*STATE_REPLACED*/')
end = p.index('\n}\n\nfunc CanonicalStages', start)
p = p[:start] + p[end+3:]
p = replace_once(p, 'func Begin(repoRoot, runID string) (State, error) {\n\trel, err := Path(runID)', 'func Begin(repoRoot, runID string) (State, error) {\n\treturn beginWithProtocol(repoRoot, runID, "")\n}\n\nfunc Begin170(repoRoot, runID string) (State, error) {\n\treturn beginWithProtocol(repoRoot, runID, Protocol170)\n}\n\nfunc beginWithProtocol(repoRoot, runID, protocolVersion string) (State, error) {\n\trel, err := Path(runID)', 'begin protocol')
p = replace_once(p, 'state := State{\n\t\tVersion:      1,', 'state := State{\n\t\tVersion:         1,\n\t\tProtocolVersion: protocolVersion,', 'state protocol assignment')
p = replace_once(p, 'if state.Version != 1 || !runIDPattern.MatchString(state.RunID) {', 'if state.Version != 1 || !runIDPattern.MatchString(state.RunID) || (state.ProtocolVersion != "" && state.ProtocolVersion != Protocol170) {', 'validate protocol')
write('internal/reviewprogress/progress.go', p)

# 2. Review context model: Spring resolver and bounded caller optional contract.
p = read('internal/reviewcontext/model_170.go')
p = replace_once(p, '\tDubbo(context.Context, nav.ReviewRef170) ([]nav.Relation170, []nav.Issue170, error)\n}', '\tDubbo(context.Context, nav.ReviewRef170) ([]nav.Relation170, []nav.Issue170, error)\n\tSpring(context.Context, nav.ReviewRef170) ([]nav.Relation170, []nav.Issue170, error)\n}\n\ntype BoundedCallerResolver170 interface {\n\tCallersBounded(context.Context, nav.ReviewRef170, int) ([]nav.Relation170, int, bool, error)\n}', 'resolver spring/bounded')
write('internal/reviewcontext/model_170.go', p)

# 3. Bound reverse caller semantic candidates in nav.
p = read('internal/nav/context_discovery_170.go')
old = '''func (n Navigator) ExactCallers170(ctx context.Context, target ReviewRef170) ([]Relation170, error) {
\tif target.Kind != "METHOD" || target.Side != "CURRENT" {
\t\treturn []Relation170{}, nil
\t}
\ttargetKey, err := ReviewRefKey170(target)
\tif err != nil {
\t\treturn nil, err
\t}
\tidx, err := n.buildJavaIndex170(ctx)
\tif err != nil {
\t\treturn nil, err
\t}
\tif _, err := idx.findMethod170(target); err != nil {
\t\treturn nil, err
\t}
\tseen := map[string]bool{}
\tout := []Relation170{}
\tfor _, method := range idx.Methods {
\t\tif err := ctx.Err(); err != nil {
\t\t\treturn nil, err
\t\t}
\t\tfrom := idx.methodRef170(method, target.Workspace, "CURRENT")
\t\tcalls, err := n.resolveMethodCalls170(ctx, idx, method, from)
\t\tif err != nil {
\t\t\treturn nil, err
\t\t}
\t\tfor _, call := range calls {
\t\t\trel := call.Relation
\t\t\tif rel.Resolution != "EXACT" || rel.Kind != "JAVA_CALL" || len(rel.Targets) != 1 {
\t\t\t\tcontinue
\t\t\t}
\t\t\tkey, err := ReviewRefKey170(rel.Targets[0])
\t\t\tif err != nil {
\t\t\t\tcontinue
\t\t\t}
\t\t\tif key != targetKey || seen[rel.ID] {
\t\t\t\tcontinue
\t\t\t}
\t\t\tseen[rel.ID] = true
\t\t\tout = append(out, rel)
\t\t}
\t}
\tsort.Slice(out, func(i, j int) bool {
\t\treturn relationSortKeyForCallers170(out[i]) < relationSortKeyForCallers170(out[j])
\t})
\treturn out, nil
}'''
new = '''func (n Navigator) ExactCallers170(ctx context.Context, target ReviewRef170) ([]Relation170, error) {
\tout, _, _, err := n.ExactCallersBounded170(ctx, target, 0)
\treturn out, err
}

// ExactCallersBounded170 counts each method that enters forward semantic
// resolution. maxCandidates<=0 preserves the historical unlimited helper; T5
// passes the remaining Runtime budget so reverse discovery cannot scan the
// whole Java index outside the candidate budget.
func (n Navigator) ExactCallersBounded170(ctx context.Context, target ReviewRef170, maxCandidates int) ([]Relation170, int, bool, error) {
\tif target.Kind != "METHOD" || target.Side != "CURRENT" {
\t\treturn []Relation170{}, 0, false, nil
\t}
\ttargetKey, err := ReviewRefKey170(target)
\tif err != nil {
\t\treturn nil, 0, false, err
\t}
\tidx, err := n.buildJavaIndex170(ctx)
\tif err != nil {
\t\treturn nil, 0, false, err
\t}
\tif _, err := idx.findMethod170(target); err != nil {
\t\treturn nil, 0, false, err
\t}
\tseen := map[string]bool{}
\tout := []Relation170{}
\texamined := 0
\tfor _, method := range idx.Methods {
\t\tif err := ctx.Err(); err != nil {
\t\t\treturn nil, examined, false, err
\t\t}
\t\tif maxCandidates > 0 && examined >= maxCandidates {
\t\t\tsort.Slice(out, func(i, j int) bool { return relationSortKeyForCallers170(out[i]) < relationSortKeyForCallers170(out[j]) })
\t\t\treturn out, examined, true, nil
\t\t}
\t\texamined++
\t\tfrom := idx.methodRef170(method, target.Workspace, "CURRENT")
\t\tcalls, err := n.resolveMethodCalls170(ctx, idx, method, from)
\t\tif err != nil {
\t\t\treturn nil, examined, false, err
\t\t}
\t\tfor _, call := range calls {
\t\t\trel := call.Relation
\t\t\tif rel.Resolution != "EXACT" || rel.Kind != "JAVA_CALL" || len(rel.Targets) != 1 {
\t\t\t\tcontinue
\t\t\t}
\t\t\tkey, err := ReviewRefKey170(rel.Targets[0])
\t\t\tif err != nil || key != targetKey || seen[rel.ID] {
\t\t\t\tcontinue
\t\t\t}
\t\t\tseen[rel.ID] = true
\t\t\tout = append(out, rel)
\t\t}
\t}
\tsort.Slice(out, func(i, j int) bool { return relationSortKeyForCallers170(out[i]) < relationSortKeyForCallers170(out[j]) })
\treturn out, examined, false, nil
}'''
p = replace_once(p, old, new, 'bounded exact callers')
# Side-aware discovery for BASE adapter.
p = replace_once(p, 'func (n Navigator) DiscoverMethods170(ctx context.Context, paths []string) ([]ReviewRef170, []SourceRange170, error) {\n\tidx, err := n.buildJavaIndex170(ctx)', 'func (n Navigator) DiscoverMethods170(ctx context.Context, paths []string) ([]ReviewRef170, []SourceRange170, error) {\n\treturn n.DiscoverMethodsAtSide170(ctx, paths, "current", "CURRENT")\n}\n\nfunc (n Navigator) DiscoverMethodsAtSide170(ctx context.Context, paths []string, workspace, side string) ([]ReviewRef170, []SourceRange170, error) {\n\tidx, err := n.buildJavaIndex170(ctx)', 'side discovery signature')
p = replace_once(p, 'ref := idx.methodRef170(method, "current", "CURRENT")', 'ref := idx.methodRef170(method, workspace, side)', 'side discovery ref')
write('internal/nav/context_discovery_170.go', p)

# 4. Spring method context resolver: explicit managed-bean + @Transactional evidence.
p = read('internal/nav/spring_resolution_170.go')
p = replace_once(p, '\t"Autowired":     {"org.springframework.beans.factory.annotation.Autowired"},', '\t"Autowired":     {"org.springframework.beans.factory.annotation.Autowired"},\n\t"Transactional": {"org.springframework.transaction.annotation.Transactional"},', 'transactional annotation')
p += r'''

// ResolveSpringMethodContext170 proves the source-verifiable part of Spring
// transaction proxy context for a concrete method. It does not claim that a
// runtime invocation traversed a proxy; it proves that the owner is a managed
// Spring bean and that @Transactional is present on the method or owner.
func (n Navigator) ResolveSpringMethodContext170(ctx context.Context, ref ReviewRef170) ([]Relation170, []Issue170, error) {
	if ref.Kind != "METHOD" {
		return []Relation170{}, []Issue170{}, nil
	}
	idx, err := n.buildJavaIndex170(ctx)
	if err != nil {
		return nil, nil, err
	}
	method, err := idx.findMethod170(ref)
	if err != nil {
		return nil, nil, err
	}
	ownerAnns := annotations170(method.Owner.Text)
	methodAnns := annotations170(method.Text)
	managed := springIsStereotype170(method.Owner.Imports, ownerAnns) || springHasRecognizedAnnotation170(method.Owner.Imports, ownerAnns, "Configuration")
	transactional := springHasRecognizedAnnotation170(method.Owner.Imports, methodAnns, "Transactional") || springHasRecognizedAnnotation170(method.Owner.Imports, ownerAnns, "Transactional")
	target := ReviewRef170{Workspace: ref.Workspace, Path: method.Owner.Path, Side: ref.Side, Kind: "TYPE", OwnerFQCN: method.Owner.FQCN, Name: method.Owner.Name, ParameterTypes: []string{}}
	evidence, evidenceErr := n.springTextEvidence170(ref, method.Raw.Path, method.Text, method.Raw.StartLine, method.Raw.StartColumn, "@Transactional", method.Name)
	if evidenceErr != nil {
		return nil, nil, evidenceErr
	}
	rel := Relation170{Kind: "SPRING_BINDING", From: ref, Targets: []ReviewRef170{}, Evidence: []SourceRange170{evidence}, Assumptions: []string{}}
	if !managed {
		rel.Resolution = "UNRESOLVED"
		rel.Reason = "SPRING_PROXY_CONTEXT_UNRESOLVED: owner is not a source-verifiable managed Spring bean"
	} else if !transactional {
		rel.Resolution = "UNRESOLVED"
		rel.Reason = "SPRING_PROXY_CONTEXT_UNRESOLVED: @Transactional is not source-verifiable on method or owner"
	} else if springHasCondition170(method.Owner.Imports, ownerAnns) {
		rel.Targets = []ReviewRef170{target}
		rel.Resolution = "CONDITIONAL"
		rel.Reason = "SPRING_CONDITION_UNRESOLVED: bean registration has unresolved profile/condition"
	} else {
		rel.Targets = []ReviewRef170{target}
		rel.Resolution = "EXACT"
	}
	rel.ID = springRelationID170(rel)
	return []Relation170{rel}, []Issue170{}, nil
}
'''
write('internal/nav/spring_resolution_170.go', p)

# 5. Build170: bounded callers, Spring, BASE-only resolution, timeout as limitation.
p = read('internal/reviewcontext/build_170.go')
p = replace_once(p, 'import (\n\t"context"', 'import (\n\t"context"\n\t"errors"', 'build errors import')
# Mapper deadline handling.
p = replace_once(p, '''\t\trelations, issues, err := resolver.Mapper(ctx, resource)
\t\tif err != nil {
\t\t\treturn Context170{}, fmt.Errorf("REVIEW_CONTEXT_MAPPER_FAILED: %w", err)
\t\t}''', '''\t\trelations, issues, err := resolver.Mapper(ctx, resource)
\t\tif err != nil {
\t\t\tif errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
\t\t\t\taddBudgetIssue("TIME", resource)
\t\t\t\tbreak
\t\t\t}
\t\t\treturn Context170{}, fmt.Errorf("REVIEW_CONTEXT_MAPPER_FAILED: %w", err)
\t\t}''', 'mapper timeout')
# Queue BASE seeds too.
p = replace_once(p, '''\tfor _, seed := range input.Seeds {
\t\tif seed.Side != "CURRENT" || seed.Kind != "METHOD" {
\t\t\tcontinue
\t\t}
\t\tqueue = append(queue, queue170{ref: seed})
\t}''', '''\tfor _, seed := range input.Seeds {
\t\tif (seed.Side != "CURRENT" && seed.Side != "BASE") || seed.Kind != "METHOD" {
\t\t\tcontinue
\t\t}
\t\tqueue = append(queue, queue170{ref: seed})
\t}''', 'base seeds queue')
# Method timeout.
p = replace_once(p, '''\t\tfacts, err := resolver.Method(ctx, item.ref)
\t\tif err != nil {
\t\t\treturn Context170{}, fmt.Errorf("REVIEW_CONTEXT_METHOD_FAILED: %w", err)
\t\t}''', '''\t\tfacts, err := resolver.Method(ctx, item.ref)
\t\tif err != nil {
\t\t\tif errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
\t\t\t\tat := nav.SourceRange170{Ref: item.ref, StartLine: 1, EndLine: 1, StartColumn: 1, EndColumn: 2}
\t\t\t\taddBudgetIssue("TIME", at)
\t\t\t\tbreak
\t\t\t}
\t\t\treturn Context170{}, fmt.Errorf("REVIEW_CONTEXT_METHOD_FAILED: %w", err)
\t\t}''', 'method timeout')
# Insert Spring resolution after calls, before Dubbo.
anchor = '''\t\tdubboRelations, dubboIssues, err := resolver.Dubbo(ctx, item.ref)'''
insert = '''\t\tspringRelations, springIssues, err := resolver.Spring(ctx, item.ref)
\t\tif err != nil {
\t\t\tif errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
\t\t\t\tat := nav.SourceRange170{Ref: item.ref, StartLine: 1, EndLine: 1, StartColumn: 1, EndColumn: 2}
\t\t\t\taddBudgetIssue("TIME", at)
\t\t\t\tbreak
\t\t\t}
\t\t\treturn Context170{}, fmt.Errorf("REVIEW_CONTEXT_SPRING_FAILED: %w", err)
\t\t}
\t\tout.Issues = append(out.Issues, springIssues...)
\t\tfor _, relation := range springRelations {
\t\t\tif err := addRelation(relation); err != nil {
\t\t\t\treturn Context170{}, err
\t\t\t}
\t\t}

\t\t// BASE evidence is comparison-only. Never traverse it as CURRENT caller,
\t\t// Dubbo provider, or dependency target.
\t\tif item.ref.Side == "BASE" {
\t\t\tcontinue
\t\t}

\t\tdubboRelations, dubboIssues, err := resolver.Dubbo(ctx, item.ref)'''
p = replace_once(p, anchor, insert, 'spring/base insertion')
# Dubbo timeout.
p = replace_once(p, '''\t\tif err != nil {
\t\t\treturn Context170{}, fmt.Errorf("REVIEW_CONTEXT_DUBBO_FAILED: %w", err)
\t\t}
\t\tout.Issues = append(out.Issues, dubboIssues...)''', '''\t\tif err != nil {
\t\t\tif errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
\t\t\t\tat := nav.SourceRange170{Ref: item.ref, StartLine: 1, EndLine: 1, StartColumn: 1, EndColumn: 2}
\t\t\t\taddBudgetIssue("TIME", at)
\t\t\t\tbreak
\t\t\t}
\t\t\treturn Context170{}, fmt.Errorf("REVIEW_CONTEXT_DUBBO_FAILED: %w", err)
\t\t}
\t\tout.Issues = append(out.Issues, dubboIssues...)''', 'dubbo timeout')
# Replace callers block with bounded path.
old = '''\t\tcallerRelations, err := resolver.Callers(ctx, item.ref)
\t\tif err != nil {
\t\t\treturn Context170{}, fmt.Errorf("REVIEW_CONTEXT_CALLERS_FAILED: %w", err)
\t\t}
\t\tfor _, relation := range callerRelations {'''
new = '''\t\tcallerRelations := []nav.Relation170{}
\t\tif bounded, ok := resolver.(BoundedCallerResolver170); ok {
\t\t\tremaining := 0
\t\t\tif budget.MaxCandidates > 0 {
\t\t\t\tremaining = budget.MaxCandidates - state.candidates
\t\t\t\tif remaining <= 0 {
\t\t\t\t\tat := nav.SourceRange170{Ref: item.ref, StartLine: 1, EndLine: 1, StartColumn: 1, EndColumn: 2}
\t\t\t\t\taddBudgetIssue("JAVA_CALL", at)
\t\t\t\t\tcontinue
\t\t\t\t}
\t\t\t}
\t\t\tvar examined int
\t\t\tvar exhausted bool
\t\t\tcallerRelations, examined, exhausted, err = bounded.CallersBounded(ctx, item.ref, remaining)
\t\t\tstate.candidates += examined
\t\t\tif exhausted {
\t\t\t\tat := nav.SourceRange170{Ref: item.ref, StartLine: 1, EndLine: 1, StartColumn: 1, EndColumn: 2}
\t\t\t\taddBudgetIssue("JAVA_CALL", at)
\t\t\t}
\t\t} else {
\t\t\tcallerRelations, err = resolver.Callers(ctx, item.ref)
\t\t}
\t\tif err != nil {
\t\t\tif errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
\t\t\t\tat := nav.SourceRange170{Ref: item.ref, StartLine: 1, EndLine: 1, StartColumn: 1, EndColumn: 2}
\t\t\t\taddBudgetIssue("TIME", at)
\t\t\t\tbreak
\t\t\t}
\t\t\treturn Context170{}, fmt.Errorf("REVIEW_CONTEXT_CALLERS_FAILED: %w", err)
\t\t}
\t\tfor _, relation := range callerRelations {'''
p = replace_once(p, old, new, 'bounded caller build')
# Global timeout/candidate limitation should block missing kinds.
p = replace_once(p, 'if budgetBlocked[kind] {', 'if budgetBlocked[kind] || budgetBlocked["TIME"] {', 'timeout blocked checks')
write('internal/reviewcontext/build_170.go', p)

# 6. Existing fake resolvers/tests need Spring method to satisfy interface.
for rel in ['internal/reviewcontext/build_170_test.go', 'internal/reviewcontext/build_budget_170_test.go']:
    path = ROOT / rel
    if not path.exists():
        continue
    t = path.read_text(encoding='utf-8')
    # Add Spring to concrete fake if missing.
    if 'func (f *fakeResolver170) Spring' not in t and 'type fakeResolver170' in t:
        anchor = 'func (f *fakeResolver170) Dubbo'
        idx = t.find(anchor)
        # append a standalone method after the Dubbo method by locating next blank line after function body.
        m = re.search(r'func \(f \*fakeResolver170\) Dubbo[\s\S]*?\n}\n', t[idx:])
        if m:
            pos = idx + m.end()
            t = t[:pos] + 'func (f *fakeResolver170) Spring(context.Context, nav.ReviewRef170) ([]nav.Relation170, []nav.Issue170, error) { return []nav.Relation170{}, []nav.Issue170{}, nil }\n' + t[pos:]
    if 'type errorResolver170 struct{}' in t and 'func (errorResolver170) Spring' not in t:
        t += '\nfunc (errorResolver170) Spring(context.Context, nav.ReviewRef170)([]nav.Relation170,[]nav.Issue170,error){ return nil,nil,errors.New("boom") }\n'
    path.write_text(t, encoding='utf-8', newline='\n')

# 7. Export the existing batch base source reader instead of creating a new snapshot system.
write('internal/analysis/base_source_170.go', '''package analysis\n\nimport "context"\n\n// ReadBaseSources170 reuses the 1.6.4 exact-path git cat-file --batch reader.\n// T5 does not create a persistent snapshot or broaden the requested paths.\nfunc ReadBaseSources170(ctx context.Context, repoRoot, mergeBase string, paths []string) (map[string][]byte, error) {\n\treturn (gitBatchBaseSourceReader164{}).ReadBaseSources(ctx, repoRoot, mergeBase, paths)\n}\n''')

# 8. Runtime command wiring: base source, Spring/bounded resolver, explicit Needs.
p = read('cmd/codea-dcep-tools/review_context_command_170.go')
# Add time only if not present (for cleanup timestamps not needed); no new import.
# runtimeResolver carries ast path.
p = replace_once(p, 'type runtimeResolver170 struct {\n\troot       string', 'type runtimeResolver170 struct {\n\troot       string\n\tastGrepPath string', 'resolver ast field')
p = replace_once(p, 'r := &runtimeResolver170{root: root, navigators: map[string]nav.Navigator{"current": {RepoRoot: root, AstGrepPath: ast}}, providers:', 'r := &runtimeResolver170{root: root, astGrepPath: ast, navigators: map[string]nav.Navigator{"current": {RepoRoot: root, AstGrepPath: ast}}, providers:', 'resolver ast init')
# bounded + Spring methods after Callers.
p = replace_once(p, '''func (r *runtimeResolver170) Callers(ctx context.Context, ref nav.ReviewRef170) ([]nav.Relation170, error) {
\tn, err := r.navigator(ref)
\tif err != nil {
\t\treturn nil, err
\t}
\treturn n.ExactCallers170(ctx, ref)
}''', '''func (r *runtimeResolver170) Callers(ctx context.Context, ref nav.ReviewRef170) ([]nav.Relation170, error) {
\tn, err := r.navigator(ref)
\tif err != nil { return nil, err }
\treturn n.ExactCallers170(ctx, ref)
}
func (r *runtimeResolver170) CallersBounded(ctx context.Context, ref nav.ReviewRef170, maxCandidates int) ([]nav.Relation170, int, bool, error) {
\tn, err := r.navigator(ref)
\tif err != nil { return nil, 0, false, err }
\treturn n.ExactCallersBounded170(ctx, ref, maxCandidates)
}
func (r *runtimeResolver170) Spring(ctx context.Context, ref nav.ReviewRef170) ([]nav.Relation170, []nav.Issue170, error) {
\tn, err := r.navigator(ref)
\tif err != nil { return nil, nil, err }
\treturn n.ResolveSpringMethodContext170(ctx, ref)
}''', 'resolver bounded/spring')
# Replace Needs construction function area from needsFromDispatch to end requiredKinds.
pattern = re.compile(r'func needsFromDispatch170\([\s\S]*?\nfunc requiredKinds170\([\s\S]*?\n}\n\Z')
m = pattern.search(p)
if not m:
    raise SystemExit('needs functions block not found')
new_needs = r'''func needsFromDispatch170(units reviewunit.Manifest, dispatch reviewrules.Manifest, a analysisruntime.ChangeAnalysis) []reviewcontext.Need170 {
	unitByID := map[string]reviewunit.Unit{}
	for _, u := range units.Units { unitByID[u.ID] = u }
	needs := []reviewcontext.Need170{}
	for _, d := range dispatch.Dispatches {
		u, ok := unitByID[d.ReviewUnitID]
		if !ok { continue }
		needs = append(needs, reviewcontext.Need170{ReviewUnitID:d.ReviewUnitID, RuleID:d.RuleID, Seeds:seedsForUnit170(u, a), RequiredKinds:requiredKindsForDispatch170(d)})
	}
	return needs
}

func seedsForUnit170(u reviewunit.Unit, a analysisruntime.ChangeAnalysis) []nav.ReviewRef170 {
	wantedSymbols := map[string]bool{}
	for _, symbol := range u.Chain { wantedSymbols[strings.TrimSpace(symbol)] = true }
	if strings.TrimSpace(u.EntryPoint) != "" { wantedSymbols[strings.TrimSpace(u.EntryPoint)] = true }
	for _, symbol := range u.ContextSymbols { wantedSymbols[strings.TrimSpace(symbol)] = true }
	unitFiles := map[string]bool{}
	for _, f := range u.Files { unitFiles[strings.ToLower(filepath.ToSlash(f.Path))] = true }
	refs := []nav.ReviewRef170{}
	seen := map[string]bool{}
	add := func(workspaceID, p, symbol string) {
		if workspaceID == "" { workspaceID = "current" }
		if workspaceID != "current" || !strings.HasSuffix(strings.ToLower(p), ".java") { return }
		ref, ok := methodRefFromSymbol170(workspaceID, p, symbol); if !ok { return }
		key, err := nav.ReviewRefKey170(ref); if err != nil || seen[key] { return }
		seen[key] = true; refs = append(refs, ref)
	}
	for _, chain := range a.CallChains {
		for i, symbol := range chain.Chain {
			if !wantedSymbols[strings.TrimSpace(symbol)] || i >= len(chain.ChainRefs) { continue }
			ref := chain.ChainRefs[i]; add(ref.Workspace, ref.Path, symbol)
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
		if !unitFiles[strings.ToLower(filepath.ToSlash(rr.Path))] { continue }
		for _, loc := range a.SymbolLocations {
			if strings.TrimSpace(loc.Symbol) == strings.TrimSpace(rr.FromSymbol) { add(loc.Workspace, loc.Path, loc.Symbol) }
		}
	}
	sort.Slice(refs, func(i,j int) bool { a,_:=nav.ReviewRefKey170(refs[i]); b,_:=nav.ReviewRefKey170(refs[j]); return a<b })
	return refs
}

func requiredKindsForDispatch170(d reviewrules.Dispatch) []string {
	roles := map[string]bool{}
	for _, reason := range d.DispatchReason {
		const prefix = "CHANGED_ROLE:"
		if strings.HasPrefix(reason, prefix) { roles[strings.TrimSpace(strings.TrimPrefix(reason, prefix))] = true }
	}
	evidence := map[string]bool{}
	for _, raw := range d.RequiredEvidence { evidence[strings.ToUpper(strings.TrimSpace(raw))] = true }
	set := map[string]bool{}
	known := map[string]bool{"CHANGED_RANGE":true, "RESOURCE_RELATION":true, "SYMBOL":true, "CHAIN":true, "PROVIDER_SOURCE":true, "DUBBO_CONTRACT":true}
	for e := range evidence { if !known[e] { set["UNSUPPORTED_CONTEXT_RELATION"] = true } }
	if evidence["RESOURCE_RELATION"] {
		if roles["Mapper"] || roles["MapperXml"] { set["MYBATIS_STATEMENT"] = true } else { set["UNSUPPORTED_CONTEXT_RELATION"] = true }
	}
	if evidence["CHAIN"] {
		set["JAVA_CALL"] = true
		if strings.HasPrefix(d.RuleID, "SPRING-TX-") && roles["Service"] && evidence["SYMBOL"] { set["SPRING_BINDING"] = true }
	}
	if evidence["PROVIDER_SOURCE"] || evidence["DUBBO_CONTRACT"] { set["DUBBO_CONTRACT"] = true }
	out := make([]string,0,len(set)); for k := range set { out=append(out,k) }; sort.Strings(out); return out
}

func requiredKinds170(evidence []string) []string {
	return requiredKindsForDispatch170(reviewrules.Dispatch{RequiredEvidence:evidence})
}
'''
p = p[:m.start()] + new_needs
# Append base source helpers and use-time verifier.
p += r'''

func prepareBaseSource170(ctx context.Context, snap changeset.Snapshot, paths []string) (string, func(), error) {
	clean := []string{}
	seen := map[string]bool{}
	for _, raw := range paths {
		p := filepath.ToSlash(filepath.Clean(raw))
		if p == "." || strings.HasPrefix(p, "../") || seen[strings.ToLower(p)] { continue }
		seen[strings.ToLower(p)] = true; clean = append(clean, p)
	}
	sort.Strings(clean)
	sources, err := analysisruntime.ReadBaseSources170(ctx, ".", snap.MergeBase, clean)
	if err != nil { return "", func(){}, err }
	root, err := os.MkdirTemp("", "codea-harness-review-base-170-")
	if err != nil { return "", func(){}, err }
	cleanup := func(){ _ = os.RemoveAll(root) }
	for p, data := range sources {
		target := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil { cleanup(); return "", func(){}, err }
		if err := os.WriteFile(target, data, 0o600); err != nil { cleanup(); return "", func(){}, err }
	}
	return root, cleanup, nil
}

func basePaths170(snap changeset.Snapshot, currentSeeds []nav.ReviewRef170) []string {
	seen := map[string]bool{}; out := []string{}
	add := func(p string){ p=filepath.ToSlash(filepath.Clean(p)); if p=="." || strings.HasPrefix(p,"../") || seen[strings.ToLower(p)] { return }; seen[strings.ToLower(p)]=true; out=append(out,p) }
	for _, f := range snap.Files { add(f.Path) }
	for _, s := range currentSeeds { add(s.Path) }
	sort.Strings(out); return out
}

func addBaseInputs170(ctx context.Context, snap changeset.Snapshot, resolver *runtimeResolver170, input *reviewcontext.BuildInput170) (func(), error) {
	paths := basePaths170(snap, input.Seeds)
	baseRoot, cleanup, err := prepareBaseSource170(ctx, snap, paths)
	if err != nil { return func(){}, err }
	resolver.navigators["base"] = nav.Navigator{RepoRoot:baseRoot, AstGrepPath:resolver.astGrepPath}
	javaPaths := []string{}; for _, p := range paths { if strings.HasSuffix(strings.ToLower(p), ".java") { javaPaths=append(javaPaths,p) } }
	baseSeeds, baseRanges, err := resolver.navigators["base"].DiscoverMethodsAtSide170(ctx, javaPaths, "base", "BASE")
	if err != nil && !errors.Is(err, nav.ErrSymbolNotFound) { cleanup(); return func(){}, err }
	input.Seeds = append(input.Seeds, baseSeeds...)
	input.Resources = append(input.Resources, baseRanges...)
	return cleanup, nil
}

func loadReviewContextArtifact170(runID, phase string) (reviewcontext.Context170, error) {
	path := reviewContextArtifactPath170(runID, phase)
	raw, err := os.ReadFile(filepath.FromSlash(path)); if err != nil { return reviewcontext.Context170{}, fmt.Errorf("CONTEXT_RELATION_NOT_VERIFIED: read: %w", err) }
	var out reviewcontext.Context170
	dec := json.NewDecoder(bytes.NewReader(raw)); dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil { return out, fmt.Errorf("CONTEXT_RELATION_NOT_VERIFIED: decode: %w", err) }
	for _, rel := range out.Relations { if err := nav.ValidateRelation170(rel); err != nil { return out, fmt.Errorf("CONTEXT_RELATION_NOT_VERIFIED: %w", err) } }
	return out, nil
}

func verifyReviewContextUse170(runID string, a analysisruntime.ChangeAnalysis, units reviewunit.Manifest, dispatch reviewrules.Manifest) error {
	state, err := reviewprogress.Read(".", runID); if err != nil { return err }
	if state.ProtocolVersion != reviewprogress.Protocol170 { return nil }
	claimed, err := loadReviewContextArtifact170(runID, "RULES"); if err != nil { return err }
	resolver, err := newRuntimeResolver170("."); if err != nil { return err }
	input := reviewcontext.BuildInput170{RunID:runID, Phase:"RULES", Budget:reviewcontext.DefaultBudget170(), Seeds:seedsFromAnalysis170(a)}
	input.Resources, err = resourcesForSeeds170(context.Background(), resolver.navigators["current"], input.Seeds); if err != nil { return err }
	input.Needs = needsFromDispatch170(units, dispatch, a)
	snap, err := loadFreshChangeSet170(runID); if err != nil { return err }
	cleanup, err := addBaseInputs170(context.Background(), snap, resolver, &input); if err != nil { return err }; defer cleanup()
	if err := reviewcontext.VerifyRelations170(context.Background(), claimed, input, resolver); err != nil { return err }
	return nil
}
'''
write('cmd/codea-dcep-tools/review_context_command_170.go', p)

# Wire base inputs into both DISCOVERY and RULES before Build170.
p = read('cmd/codea-dcep-tools/review_context_command_170.go')
# DISCOVERY: after current methods/ranges assigned.
p = replace_once(p, '''\t\tbuildInput.Seeds = methods
\t\tbuildInput.Resources = ranges
\t} else {''', '''\t\tbuildInput.Seeds = methods
\t\tbuildInput.Resources = ranges
\t\tcleanup, baseErr := addBaseInputs170(context.Background(), snap, resolver, &buildInput)
\t\tif baseErr != nil { return fmt.Errorf("REVIEW_CONTEXT_BASE_SOURCE_UNAVAILABLE: %w", baseErr) }
\t\tdefer cleanup()
\t} else {''', 'discovery base inputs')
# RULES: load snap and add base after needs.
p = replace_once(p, '''\t\tbuildInput.Needs = needsFromDispatch170(units, dispatch, analysisValue)
\t}
\tbuilt, err := reviewcontext.Build170''', '''\t\tbuildInput.Needs = needsFromDispatch170(units, dispatch, analysisValue)
\t\tsnap, snapErr := loadFreshChangeSet170(runID)
\t\tif snapErr != nil { return snapErr }
\t\tcleanup, baseErr := addBaseInputs170(context.Background(), snap, resolver, &buildInput)
\t\tif baseErr != nil { return fmt.Errorf("REVIEW_CONTEXT_BASE_SOURCE_UNAVAILABLE: %w", baseErr) }
\t\tdefer cleanup()
\t}
\tbuilt, err := reviewcontext.Build170''', 'rules base inputs')
write('cmd/codea-dcep-tools/review_context_command_170.go', p)

# 9. 1.7 dispatch authority uses progress protocol, never artifact existence.
entry = r'''package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codea-harness-tools/internal/reviewprogress"
	"codea-harness-tools/internal/reviewrules"
	"codea-harness-tools/internal/reviewunit"
)

func runReview170(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "begin":
			return runReviewBegin170(args[1:])
		case "context":
			return runReviewContext170(args[1:])
		case "dispatch":
			protocol, err := reviewDispatchProtocol170(args[1:])
			if err != nil { return err }
			if protocol == reviewprogress.Protocol170 { return runReviewDispatch170(args[1:]) }
		}
	}
	return runReview160(args)
}

func isReviewProtocol170(state reviewprogress.State) bool { return state.ProtocolVersion == reviewprogress.Protocol170 }

func reviewDispatchProtocol170(args []string) (string, error) {
	fs := flag.NewFlagSet("review dispatch protocol", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	runID := fs.String("run-id", "", "")
	if err := fs.Parse(args); err != nil || fs.NArg()!=0 || strings.TrimSpace(*runID)=="" { return "", errors.New("review dispatch requires --run-id") }
	state, err := reviewprogress.Read(".", strings.TrimSpace(*runID)); if err != nil { return "", err }
	if isReviewProtocol170(state) { return reviewprogress.Protocol170, nil }
	return "legacy", nil
}

func runReviewBegin170(args []string) error {
	if len(args)!=0 { return errors.New("review begin takes no arguments") }
	runsRoot := filepath.Join(".code-harness","runs")
	if err:=os.MkdirAll(runsRoot,0o755); err!=nil { return fmt.Errorf("REVIEW_BEGIN_RUNS_DIR_FAILED: %w",err) }
	for attempt:=0; attempt<32; attempt++ {
		entropy:=make([]byte,16); if _,err:=rand.Read(entropy); err!=nil { return fmt.Errorf("REVIEW_BEGIN_RANDOM_FAILED: %w",err) }
		runID:="review-"+hex.EncodeToString(entropy); runPath:=filepath.Join(runsRoot,runID)
		if err:=os.Mkdir(runPath,0o755); err!=nil { if errors.Is(err,os.ErrExist){continue}; return err }
		if _,err:=reviewprogress.Begin170(".",runID); err!=nil { _=os.RemoveAll(runPath); return fmt.Errorf("REVIEW_BEGIN_PROGRESS_FAILED: %w",err) }
		progressPath,_:=reviewprogress.Path(runID)
		return writeJSONAndStatus(map[string]any{"status":"READY","runId":runID,"runPath":filepath.ToSlash(runPath),"progressPath":filepath.ToSlash(progressPath),"protocolVersion":reviewprogress.Protocol170},true)
	}
	return errors.New("REVIEW_BEGIN_RUN_ID_EXHAUSTED")
}

func requireReviewDiscovery170(runID string) error {
	ctx, err := loadReviewContextArtifact170(runID,"DISCOVERY")
	if err != nil { return fmt.Errorf("REVIEW_CONTEXT_DISCOVERY_REQUIRED: %w",err) }
	if ctx.RunID!=runID || ctx.Phase!="DISCOVERY" { return fmt.Errorf("REVIEW_CONTEXT_DISCOVERY_REQUIRED: context identity mismatch") }
	return nil
}

func runReviewDispatch170(args []string) error {
	fs:=flag.NewFlagSet("review dispatch",flag.ContinueOnError); runID:=fs.String("run-id","","same-run Runtime-owned ReviewUnit run id")
	if err:=fs.Parse(args); err!=nil{return err}; if fs.NArg()!=0||*runID==""{return errors.New("review dispatch requires --run-id")}
	canonicalRunID:=strings.TrimSpace(*runID); if canonicalRunID!=*runID{return fmt.Errorf("RULE_DISPATCH_RUN_ID_INVALID: %q",*runID)}
	state,err:=reviewprogress.Read(".",canonicalRunID); if err!=nil{return err}
	if !isReviewProtocol170(state) { return fmt.Errorf("REVIEW_PROTOCOL_MISMATCH: run %s is not 1.7", canonicalRunID) }
	if err:=requireReviewDiscovery170(canonicalRunID); err!=nil{return err}
	if state.Status!=reviewprogress.StatusRunning||state.CurrentStage!=reviewprogress.StageReviewPlanning{return fmt.Errorf("RULE_DISPATCH_PROGRESS_INVALID: current=%s status=%s",state.CurrentStage,state.Status)}
	failPlanning:=func(err error)error{return failReviewProgressStage164(canonicalRunID,reviewprogress.StageReviewPlanning,"REVIEW_PLANNING_FAILED",err)}
	units,err:=reviewunit.Load(reviewunit.BuildInput{RunID:canonicalRunID,CertifiedRunID:canonicalRunID,RepoRoot:"."}); if err!=nil{return failPlanning(fmt.Errorf("RULE_DISPATCH_STALE: %w",err))}
	rules,catalogSHA,err:=reviewrules.LoadCatalog(filepath.Join(".code-harness","review-rules","spring-v1.yaml")); if err!=nil{return failPlanning(err)}
	manifest,err:=reviewrules.BuildDispatch(units,rules,catalogSHA); if err!=nil{return failPlanning(err)}
	encoded,err:=reviewrules.CanonicalBytes(manifest); if err!=nil{return failPlanning(err)}
	if err:=validateReviewContract153("rule-dispatch.schema.json",encoded); err!=nil{return failPlanning(fmt.Errorf("RULE_DISPATCH_SCHEMA_INVALID: %w",err))}
	artifactPath:=filepath.Join(".code-harness","runs",canonicalRunID,"analysis","rule-dispatch.json"); if err:=atomicReviewWrite153(artifactPath,encoded); err!=nil{return failPlanning(fmt.Errorf("RULE_DISPATCH_WRITE_FAILED: %w",err))}
	return writeJSONAndStatus(map[string]any{"status":"READY","artifactPath":filepath.ToSlash(artifactPath),"manifest":manifest,"planningFinalized":false},true)
}
'''
write('cmd/codea-dcep-tools/review_context_entry_170.go', entry)

# 10. Use-time verification in finding certification.
p = read('cmd/codea-dcep-tools/review_precision_command.go')
p = replace_once(p, '\t_, analysisCert, err := analysisruntime.LoadCertified(".", analysisPath)', '\tanalysisValue, analysisCert, err := analysisruntime.LoadCertified(".", analysisPath)', 'retain analysis value')
p = replace_once(p, '''\tdispatchBytes, err := os.ReadFile(filepath.Join(".code-harness", "runs", runID, "analysis", "rule-dispatch.json"))
\tif err != nil {
\t\treturn failCertification(fmt.Errorf("FINDING_RULE_DISPATCH_READ_FAILED: %w", err))
\t}
\tctx := finding.CertifyContext{''', '''\tdispatchBytes, err := os.ReadFile(filepath.Join(".code-harness", "runs", runID, "analysis", "rule-dispatch.json"))
\tif err != nil {
\t\treturn failCertification(fmt.Errorf("FINDING_RULE_DISPATCH_READ_FAILED: %w", err))
\t}
\tvar dispatch reviewrules.Manifest
\tif err := json.Unmarshal(dispatchBytes, &dispatch); err != nil { return failCertification(fmt.Errorf("FINDING_RULE_DISPATCH_INVALID: %w", err)) }
\tif err := verifyReviewContextUse170(runID, analysisValue, units, dispatch); err != nil { return failCertification(err) }
\tctx := finding.CertifyContext{''', 'use-time verify')
write('cmd/codea-dcep-tools/review_precision_command.go', p)

# 11. Stronger VerifyRelations: equal relation set + exact checks, not a claimed subset.
p = read('internal/reviewcontext/verify_170.go')
p = replace_once(p, '''\tfor _, relation := range claimed.Relations {''', '''\tif len(claimed.Relations) != len(fresh.Relations) { return fmt.Errorf("CONTEXT_RELATION_NOT_VERIFIED: relation set size changed") }
\tfor _, relation := range claimed.Relations {''', 'verify relation set size')
p = replace_once(p, '''\tfor _, check := range claimed.Checks {
\t\tfor _, relationID := range check.RelationIDs {
\t\t\tif _, ok := freshByID[relationID]; !ok {
\t\t\t\treturn fmt.Errorf("CONTEXT_RELATION_NOT_VERIFIED: check %s/%s references unverified relation %s", check.ReviewUnitID, check.RuleID, relationID)
\t\t\t}
\t\t}
\t}
\treturn nil''', '''\tclaimedChecks, err := json.Marshal(claimed.Checks); if err != nil { return err }
\tfreshChecks, err := json.Marshal(fresh.Checks); if err != nil { return err }
\tif string(claimedChecks) != string(freshChecks) { return fmt.Errorf("CONTEXT_RELATION_NOT_VERIFIED: rule ownership/checks changed") }
\treturn nil''', 'verify exact checks')
write('internal/reviewcontext/verify_170.go', p)

print('TASK170_T5_BLOCKER_PATCH_APPLIED')
