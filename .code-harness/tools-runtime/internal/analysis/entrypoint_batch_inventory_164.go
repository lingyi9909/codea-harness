package analysis

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codea-harness-tools/internal/changeset"
	"codea-harness-tools/internal/nav"
)

type entrypointExecutionMetrics164 struct {
	CurrentRequestedFiles     []string
	BaseRequestedFiles        []string
	CurrentScannedFiles       []string
	BaseScannedFiles          []string
	AstGrepProcessCount       int
	BaseGitBatchProcessCount  int
	PerFileGitMergeBaseCount  int
	PerFileGitShowCount       int
	FullProjectBaseScanCount  int
}

type countingEntrypointRunner164 struct {
	dir     string
	metrics *entrypointExecutionMetrics164
}

func (r countingEntrypointRunner164) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if r.metrics != nil {
		r.metrics.AstGrepProcessCount++
	}
	return rootedRunner153{dir: r.dir}.Run(ctx, name, args...)
}

type precomputedEntrypointScanner164 struct {
	plan    EntrypointScanPlan
	current map[string][]ControllerEndpoint
	base    map[string][]ControllerEndpoint
}

func (s precomputedEntrypointScanner164) Current(_ context.Context, p string) ([]ControllerEndpoint, error) {
	if err := validateEntrypointMember164(s.plan, entrypointScanSideCurrent164, p); err != nil {
		return nil, err
	}
	return append([]ControllerEndpoint(nil), s.current[filepath.ToSlash(p)]...), nil
}

func (s precomputedEntrypointScanner164) Base(_ context.Context, snapshot changeset.Snapshot, p string) ([]ControllerEndpoint, error) {
	if strings.TrimSpace(snapshot.MergeBase) != s.plan.MergeBase {
		return nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_OUT_OF_SCOPE: snapshot mergeBase changed")
	}
	if err := validateEntrypointMember164(s.plan, entrypointScanSideBase164, p); err != nil {
		return nil, err
	}
	return append([]ControllerEndpoint(nil), s.base[filepath.ToSlash(p)]...), nil
}

func scanEntrypointPlan164(ctx context.Context, repoRoot, astGrepPath string, plan EntrypointScanPlan, baseSources map[string][]byte, metrics *entrypointExecutionMetrics164) (precomputedEntrypointScanner164, error) {
	if err := validateEntrypointPlanBinding164(plan); err != nil {
		return precomputedEntrypointScanner164{}, err
	}
	if err := validateEntrypointScanPaths164(plan, entrypointScanSideCurrent164, plan.CurrentPaths); err != nil {
		return precomputedEntrypointScanner164{}, err
	}
	if err := validateEntrypointScanPaths164(plan, entrypointScanSideBase164, plan.BasePaths); err != nil {
		return precomputedEntrypointScanner164{}, err
	}
	if metrics != nil {
		metrics.CurrentRequestedFiles = append([]string(nil), plan.CurrentPaths...)
		metrics.BaseRequestedFiles = append([]string(nil), plan.BasePaths...)
		metrics.CurrentScannedFiles = append([]string(nil), plan.CurrentPaths...)
		metrics.BaseScannedFiles = append([]string(nil), plan.BasePaths...)
	}

	currentByPath := map[string][]ControllerEndpoint{}
	if len(plan.CurrentPaths) > 0 {
		n := nav.Navigator{
			RepoRoot: repoRoot, AstGrepPath: astGrepPath,
			Runner: countingEntrypointRunner164{dir: repoRoot, metrics: metrics},
		}
		matches, err := n.FindControllerEndpointsBatch(ctx, plan.CurrentPaths)
		if err != nil {
			return precomputedEntrypointScanner164{}, fmt.Errorf("ENTRYPOINT_CURRENT_SCAN_FAILED: %w", err)
		}
		currentEndpoints := controllerEndpointMatches164(matches)
		if err := validateEntrypointScanResults164(plan.CurrentPaths, currentEndpoints); err != nil {
			return precomputedEntrypointScanner164{}, err
		}
		currentByPath = groupControllerEndpoints164(currentEndpoints)
	}

	baseByPath := map[string][]ControllerEndpoint{}
	if len(plan.BasePaths) > 0 {
		baseRoot, cleanup, err := materializeEntrypointBaseTree164(plan, baseSources)
		if err != nil {
			return precomputedEntrypointScanner164{}, err
		}
		defer cleanup()
		n := nav.Navigator{
			RepoRoot: baseRoot, AstGrepPath: astGrepPath,
			Runner: countingEntrypointRunner164{dir: baseRoot, metrics: metrics},
		}
		matches, err := n.FindControllerEndpointsBatch(ctx, plan.BasePaths)
		if err != nil {
			return precomputedEntrypointScanner164{}, fmt.Errorf("ENTRYPOINT_BASE_SCAN_FAILED: %w", err)
		}
		baseEndpoints := controllerEndpointMatches164(matches)
		if err := validateEntrypointScanResults164(plan.BasePaths, baseEndpoints); err != nil {
			return precomputedEntrypointScanner164{}, err
		}
		baseByPath = groupControllerEndpoints164(baseEndpoints)
	}

	return precomputedEntrypointScanner164{plan: plan, current: currentByPath, base: baseByPath}, nil
}

func validateEntrypointPlanBinding164(plan EntrypointScanPlan) error {
	current, ok := exactEntrypointPathSet164(plan.CurrentPaths)
	if !ok || !equalStringSlices164(current, plan.CurrentPaths) {
		return fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: invalid current plan paths")
	}
	base, ok := exactEntrypointPathSet164(plan.BasePaths)
	if !ok || !equalStringSlices164(base, plan.BasePaths) {
		return fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: invalid base plan paths")
	}
	if plan.CurrentScopeSHA256 != hashEntrypointScope164(plan, entrypointScanSideCurrent164, plan.CurrentPaths) {
		return fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: current scope hash mismatch")
	}
	if plan.BaseScopeSHA256 != hashEntrypointScope164(plan, entrypointScanSideBase164, plan.BasePaths) {
		return fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: base scope hash mismatch")
	}
	return nil
}

func materializeEntrypointBaseTree164(plan EntrypointScanPlan, baseSources map[string][]byte) (string, func(), error) {
	cleanKeys, ok := exactEntrypointPathSet164(mapKeysSorted164(baseSources))
	if !ok || !equalStringSlices164(cleanKeys, plan.BasePaths) {
		return "", nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_OUT_OF_SCOPE: base source set=%v expected=%v", cleanKeys, plan.BasePaths)
	}
	root, err := os.MkdirTemp("", "codea-harness-entrypoint-base-164-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(root) }
	for _, p := range plan.BasePaths {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			cleanup()
			return "", nil, err
		}
		if err := os.WriteFile(full, baseSources[p], 0o600); err != nil {
			cleanup()
			return "", nil, err
		}
	}
	return root, cleanup, nil
}

func controllerEndpointMatches164(matches []nav.ControllerEndpointMatch) []ControllerEndpoint {
	out := make([]ControllerEndpoint, 0, len(matches))
	for _, match := range matches {
		out = append(out, ControllerEndpoint{
			Controller: match.Controller, Symbol: match.Symbol, Path: filepath.ToSlash(match.Path),
			ControllerStartLine: match.ControllerStartLine, ControllerEndLine: match.ControllerEndLine,
			StartLine: match.StartLine, EndLine: match.EndLine,
		})
	}
	return out
}

func groupControllerEndpoints164(endpoints []ControllerEndpoint) map[string][]ControllerEndpoint {
	out := map[string][]ControllerEndpoint{}
	for _, endpoint := range endpoints {
		p := filepath.ToSlash(endpoint.Path)
		out[p] = append(out[p], endpoint)
	}
	for p := range out {
		sort.Slice(out[p], func(i, j int) bool {
			if out[p][i].Symbol != out[p][j].Symbol {
				return out[p][i].Symbol < out[p][j].Symbol
			}
			return out[p][i].StartLine < out[p][j].StartLine
		})
	}
	return out
}

func mapKeysSorted164(in map[string][]byte) []string {
	out := make([]string, 0, len(in))
	for key := range in {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
