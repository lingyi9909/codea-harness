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

# Correct the bounded-caller contract to the permanent acceptance API.
p = read('internal/reviewcontext/model_170.go')
p = p.replace('CallersBounded(context.Context, nav.ReviewRef170, int)', 'CallersBounded170(context.Context, nav.ReviewRef170, int)')
write('internal/reviewcontext/model_170.go', p)

# Keep the public side-aware discovery name fixed by the regression contract.
p = read('internal/nav/context_discovery_170.go')
p = p.replace('DiscoverMethodsAtSide170', 'DiscoverMethodsForSide170')
write('internal/nav/context_discovery_170.go', p)

# Build170: BASE is comparison input only, never normal CURRENT traversal.
p = read('internal/reviewcontext/build_170.go')
p = replace_once(p,
'''\tfor _, seed := range input.Seeds {\n\t\tif (seed.Side != "CURRENT" && seed.Side != "BASE") || seed.Kind != "METHOD" {\n\t\t\tcontinue\n\t\t}\n\t\tqueue = append(queue, queue170{ref: seed})\n\t}''',
'''\tfor _, seed := range input.Seeds {\n\t\tif seed.Side != "CURRENT" || seed.Kind != "METHOD" {\n\t\t\tcontinue\n\t\t}\n\t\tqueue = append(queue, queue170{ref: seed})\n\t}''',
'BASE must not enter current traversal')
p = p.replace('bounded.CallersBounded(ctx, item.ref, remaining)', 'bounded.CallersBounded170(ctx, item.ref, remaining)')
p = replace_once(p,
'''\t\t\tcallerRelations, examined, exhausted, err = bounded.CallersBounded170(ctx, item.ref, remaining)\n\t\t\tstate.candidates += examined\n\t\t\tif exhausted {\n\t\t\t\tat := nav.SourceRange170{Ref: item.ref, StartLine: 1, EndLine: 1, StartColumn: 1, EndColumn: 2}\n\t\t\t\taddBudgetIssue("JAVA_CALL", at)\n\t\t\t}''',
'''\t\t\tcallerRelations, examined, exhausted, err = bounded.CallersBounded170(ctx, item.ref, remaining)\n\t\t\tcredited := examined\n\t\t\tif remaining > 0 && credited > remaining {\n\t\t\t\tcredited = remaining\n\t\t\t}\n\t\t\tif credited > 0 {\n\t\t\t\tstate.candidates += credited\n\t\t\t}\n\t\t\tif exhausted || examined > credited {\n\t\t\t\tat := nav.SourceRange170{Ref: item.ref, StartLine: 1, EndLine: 1, StartColumn: 1, EndColumn: 2}\n\t\t\t\taddBudgetIssue("JAVA_CALL", at)\n\t\t\t}''',
'candidate credit capped')
# Caller relations were produced by candidates already counted above. Add the
# relation without consuming a second semantic-candidate slot.
p = replace_once(p,
'''\t\tfor _, relation := range callerRelations {\n\t\t\tif err := addRelation(relation); err != nil {\n\t\t\t\treturn Context170{}, err\n\t\t\t}\n\t\t\t_, accepted := relationByKey[relationKey170(relation)]''',
'''\t\tfor _, relation := range callerRelations {\n\t\t\tbeforeCandidates := state.candidates\n\t\t\tif beforeCandidates > 0 {\n\t\t\t\tstate.candidates--\n\t\t\t}\n\t\t\taddErr := addRelation(relation)\n\t\t\tstate.candidates = beforeCandidates\n\t\t\tif addErr != nil {\n\t\t\t\treturn Context170{}, addErr\n\t\t\t}\n\t\t\t_, accepted := relationByKey[relationKey170(relation)]''',
'caller relation not double counted')
# A Spring binding may target the seed's declaring TYPE rather than the exact
# METHOD identity. Treat it as seed-owned only when workspace/path/side/FQCN
# all match; this does not weaken arbitrary relation matching.
p = replace_once(p,
'''\t\tfor _, target := range relation.Targets {\n\t\t\ttargetKey, err := nav.ReviewRefKey170(target)\n\t\t\tif err == nil && targetKey == seedKey {\n\t\t\t\treturn true\n\t\t\t}\n\t\t}\n''',
'''\t\tfor _, target := range relation.Targets {\n\t\t\ttargetKey, err := nav.ReviewRefKey170(target)\n\t\t\tif err == nil && targetKey == seedKey {\n\t\t\t\treturn true\n\t\t\t}\n\t\t\tif relation.Kind == "SPRING_BINDING" && target.Kind == "TYPE" && seed.Kind == "METHOD" &&\n\t\t\t\tstrings.TrimSpace(target.Workspace) == strings.TrimSpace(seed.Workspace) &&\n\t\t\t\tstrings.EqualFold(strings.ReplaceAll(strings.TrimSpace(target.Path), "\\\\", "/"), strings.ReplaceAll(strings.TrimSpace(seed.Path), "\\\\", "/")) &&\n\t\t\t\ttarget.Side == seed.Side && strings.TrimSpace(target.OwnerFQCN) == strings.TrimSpace(seed.OwnerFQCN) {\n\t\t\t\treturn true\n\t\t\t}\n\t\t}\n''',
'Spring type target owns method seed')
write('internal/reviewcontext/build_170.go', p)

# Exact 1.7/legacy routing contract. Runtime progress identity is authority;
# artifact existence is never used to downgrade a 1.7 run.
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
			state, err := reviewDispatchState170(args[1:])
			if err != nil { return err }
			if reviewDispatchProtocol170(state) == reviewprogress.Protocol170 {
				return runReviewDispatch170(args[1:])
			}
		}
	}
	return runReview160(args)
}

func isReviewProtocol170(state reviewprogress.State) bool {
	return state.ProtocolVersion == reviewprogress.Protocol170
}

func reviewDispatchProtocol170(state reviewprogress.State) string {
	if isReviewProtocol170(state) { return reviewprogress.Protocol170 }
	return "LEGACY"
}

func reviewDispatchState170(args []string) (reviewprogress.State, error) {
	fs := flag.NewFlagSet("review dispatch protocol", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	runID := fs.String("run-id", "", "")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 || strings.TrimSpace(*runID) == "" {
		return reviewprogress.State{}, errors.New("review dispatch requires --run-id")
	}
	return reviewprogress.Read(".", strings.TrimSpace(*runID))
}

func runReviewBegin170(args []string) error {
	if len(args) != 0 { return errors.New("review begin takes no arguments") }
	runsRoot := filepath.Join(".code-harness", "runs")
	if err := os.MkdirAll(runsRoot, 0o755); err != nil { return fmt.Errorf("REVIEW_BEGIN_RUNS_DIR_FAILED: %w", err) }
	for attempt := 0; attempt < 32; attempt++ {
		entropy := make([]byte, 16)
		if _, err := rand.Read(entropy); err != nil { return fmt.Errorf("REVIEW_BEGIN_RANDOM_FAILED: %w", err) }
		runID := "review-" + hex.EncodeToString(entropy)
		runPath := filepath.Join(runsRoot, runID)
		if err := os.Mkdir(runPath, 0o755); err != nil {
			if errors.Is(err, os.ErrExist) { continue }
			return err
		}
		if _, err := reviewprogress.Begin170(".", runID); err != nil {
			_ = os.RemoveAll(runPath)
			return fmt.Errorf("REVIEW_BEGIN_PROGRESS_FAILED: %w", err)
		}
		progressPath, _ := reviewprogress.Path(runID)
		return writeJSONAndStatus(map[string]any{
			"status":"READY", "runId":runID, "runPath":filepath.ToSlash(runPath),
			"progressPath":filepath.ToSlash(progressPath), "protocolVersion":reviewprogress.Protocol170,
		}, true)
	}
	return errors.New("REVIEW_BEGIN_RUN_ID_EXHAUSTED")
}

func requireReviewDiscovery170(runID string) error {
	ctx, err := loadReviewContextArtifact170(runID, "DISCOVERY")
	if err != nil { return fmt.Errorf("REVIEW_CONTEXT_DISCOVERY_REQUIRED: %w", err) }
	if ctx.RunID != runID || ctx.Phase != "DISCOVERY" {
		return fmt.Errorf("REVIEW_CONTEXT_DISCOVERY_REQUIRED: context identity mismatch")
	}
	return nil
}

func runReviewDispatch170(args []string) error {
	fs := flag.NewFlagSet("review dispatch", flag.ContinueOnError)
	runID := fs.String("run-id", "", "same-run Runtime-owned ReviewUnit run id")
	if err := fs.Parse(args); err != nil { return err }
	if fs.NArg() != 0 || *runID == "" { return errors.New("review dispatch requires --run-id") }
	canonicalRunID := strings.TrimSpace(*runID)
	if canonicalRunID != *runID { return fmt.Errorf("RULE_DISPATCH_RUN_ID_INVALID: %q", *runID) }
	state, err := reviewprogress.Read(".", canonicalRunID)
	if err != nil { return err }
	if !isReviewProtocol170(state) { return fmt.Errorf("REVIEW_PROTOCOL_MISMATCH: run %s is not 1.7", canonicalRunID) }
	if err := requireReviewDiscovery170(canonicalRunID); err != nil { return err }
	if state.Status != reviewprogress.StatusRunning || state.CurrentStage != reviewprogress.StageReviewPlanning {
		return fmt.Errorf("RULE_DISPATCH_PROGRESS_INVALID: current=%s status=%s", state.CurrentStage, state.Status)
	}
	failPlanning := func(err error) error {
		return failReviewProgressStage164(canonicalRunID, reviewprogress.StageReviewPlanning, "REVIEW_PLANNING_FAILED", err)
	}
	units, err := reviewunit.Load(reviewunit.BuildInput{RunID:canonicalRunID, CertifiedRunID:canonicalRunID, RepoRoot:"."})
	if err != nil { return failPlanning(fmt.Errorf("RULE_DISPATCH_STALE: %w", err)) }
	rules, catalogSHA, err := reviewrules.LoadCatalog(filepath.Join(".code-harness", "review-rules", "spring-v1.yaml"))
	if err != nil { return failPlanning(err) }
	manifest, err := reviewrules.BuildDispatch(units, rules, catalogSHA)
	if err != nil { return failPlanning(err) }
	encoded, err := reviewrules.CanonicalBytes(manifest)
	if err != nil { return failPlanning(err) }
	if err := validateReviewContract153("rule-dispatch.schema.json", encoded); err != nil {
		return failPlanning(fmt.Errorf("RULE_DISPATCH_SCHEMA_INVALID: %w", err))
	}
	artifactPath := filepath.Join(".code-harness", "runs", canonicalRunID, "analysis", "rule-dispatch.json")
	if err := atomicReviewWrite153(artifactPath, encoded); err != nil {
		return failPlanning(fmt.Errorf("RULE_DISPATCH_WRITE_FAILED: %w", err))
	}
	// 1.7 planning finalizes only after RULES context succeeds.
	return writeJSONAndStatus(map[string]any{
		"status":"READY", "artifactPath":filepath.ToSlash(artifactPath), "manifest":manifest, "planningFinalized":false,
	}, true)
}
'''
write('cmd/codea-dcep-tools/review_context_entry_170.go', entry)

# BASE adapter has the exact acceptance signature and reuses existing batch Git
# source extraction. It returns an ephemeral Navigator, not a new snapshot.
p = read('cmd/codea-dcep-tools/review_context_command_170.go')
p = p.replace('DiscoverMethodsAtSide170', 'DiscoverMethodsForSide170')
pattern = re.compile(r'func prepareBaseSource170\([\s\S]*?\nfunc basePaths170', re.M)
m = pattern.search(p)
if not m:
    raise SystemExit('prepareBaseSource170 block not found')
base_block = r'''type baseSourceContext170 struct {
	Root      string
	Navigator nav.Navigator
}

func prepareBaseSource170(ctx context.Context, repoRoot string, snap changeset.Snapshot, paths []string, astPath string) (baseSourceContext170, func(), error) {
	clean := []string{}
	seen := map[string]bool{}
	for _, raw := range paths {
		p := filepath.ToSlash(filepath.Clean(raw))
		if p == "." || strings.HasPrefix(p, "../") || seen[strings.ToLower(p)] { continue }
		seen[strings.ToLower(p)] = true
		clean = append(clean, p)
	}
	sort.Strings(clean)
	sources, err := analysisruntime.ReadBaseSources170(ctx, repoRoot, snap.MergeBase, clean)
	if err != nil { return baseSourceContext170{}, func(){}, err }
	root, err := os.MkdirTemp("", "codea-harness-review-base-170-")
	if err != nil { return baseSourceContext170{}, func(){}, err }
	cleanup := func(){ _ = os.RemoveAll(root) }
	for p, data := range sources {
		target := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil { cleanup(); return baseSourceContext170{}, func(){}, err }
		if err := os.WriteFile(target, data, 0o600); err != nil { cleanup(); return baseSourceContext170{}, func(){}, err }
	}
	return baseSourceContext170{Root:root, Navigator:nav.Navigator{RepoRoot:root, AstGrepPath:astPath}}, cleanup, nil
}

func basePaths170'''
p = p[:m.start()] + base_block + p[m.end():]
pattern = re.compile(r'func addBaseInputs170\([\s\S]*?\nfunc loadReviewContextArtifact170', re.M)
m = pattern.search(p)
if not m:
    raise SystemExit('addBaseInputs170 block not found')
add_base = r'''func addBaseInputs170(ctx context.Context, snap changeset.Snapshot, resolver *runtimeResolver170, input *reviewcontext.BuildInput170) (func(), error) {
	paths := basePaths170(snap, input.Seeds)
	baseCtx, cleanup, err := prepareBaseSource170(ctx, resolver.root, snap, paths, resolver.astGrepPath)
	if err != nil { return func(){}, err }
	resolver.navigators["base"] = baseCtx.Navigator
	javaPaths := []string{}
	for _, p := range paths { if strings.HasSuffix(strings.ToLower(p), ".java") { javaPaths = append(javaPaths, p) } }
	baseSeeds, baseRanges, err := baseCtx.Navigator.DiscoverMethodsForSide170(ctx, javaPaths, "current", "BASE")
	if err != nil && !errors.Is(err, nav.ErrSymbolNotFound) { cleanup(); return func(){}, err }
	input.Seeds = append(input.Seeds, baseSeeds...)
	input.Resources = append(input.Resources, baseRanges...)
	return cleanup, nil
}

func loadReviewContextArtifact170'''
p = p[:m.start()] + add_base + p[m.end():]
# BASE side is authoritative even though its workspace id remains "current".
p = replace_once(p,
'''func (r *runtimeResolver170) navigator(ref nav.ReviewRef170) (nav.Navigator, error) {\n\tn, ok := r.navigators[ref.Workspace]''',
'''func (r *runtimeResolver170) navigator(ref nav.ReviewRef170) (nav.Navigator, error) {\n\tif ref.Side == "BASE" {\n\t\tif n, ok := r.navigators["base"]; ok { return n, nil }\n\t\treturn nav.Navigator{}, fmt.Errorf("BASE_SOURCE_UNAVAILABLE: base source adapter is not prepared")\n\t}\n\tn, ok := r.navigators[ref.Workspace]''',
'BASE side navigator')
# Split low-level use-time relation revalidation from the finding-certification
# artifact loader/wiring so the verifier is directly regression-testable.
p = p.replace('func verifyReviewContextUse170(runID string, a analysisruntime.ChangeAnalysis, units reviewunit.Manifest, dispatch reviewrules.Manifest) error {',
'''func verifyReviewContextUse170(ctx context.Context, claimed reviewcontext.Context170, input reviewcontext.BuildInput170, resolver reviewcontext.Resolver170) error {\n\treturn reviewcontext.VerifyRelations170(ctx, claimed, input, resolver)\n}\n\nfunc verifyReviewContextArtifactUse170(runID string, a analysisruntime.ChangeAnalysis, units reviewunit.Manifest, dispatch reviewrules.Manifest) error {''')
p = p.replace('if err := reviewcontext.VerifyRelations170(context.Background(), claimed, input, resolver); err != nil { return err }',
              'if err := verifyReviewContextUse170(context.Background(), claimed, input, resolver); err != nil { return err }')
write('cmd/codea-dcep-tools/review_context_command_170.go', p)

# Finding certification is the real consumption boundary: revalidate the RULES
# context immediately before findings are certified.
p = read('cmd/codea-dcep-tools/review_precision_command.go')
p = p.replace('verifyReviewContextUse170(runID, analysisValue, units, dispatch)', 'verifyReviewContextArtifactUse170(runID, analysisValue, units, dispatch)')
write('cmd/codea-dcep-tools/review_precision_command.go', p)

print('TASK170_T5_BLOCKER_PATCH2_APPLIED')
