package analysis

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"codea-harness-tools/internal/changeset"
)

// EntrypointScanPlan is Runtime-owned scope authority derived from the
// freshness-validated canonical ChangeSet. Agent/Reviewer/Orchestrator input
// cannot supply or widen these paths.
type EntrypointScanPlan struct {
	RunID              string
	SnapshotSHA256     string
	MergeBase          string
	CurrentPaths       []string
	BasePaths          []string
	CurrentScopeSHA256 string
	BaseScopeSHA256    string

	baseSources           map[string][]byte
	baseGitBatchProcesses int
	mergeBaseProcesses    int
}

type snapshotScopedEntrypointScanner164 struct {
	repoRoot      string
	astGrepPath   string
	plan          EntrypointScanPlan
	snapshotPaths map[string]bool
	current       map[string]bool
	base          map[string]bool

	once           sync.Once
	initErr        error
	currentResults map[string][]ControllerEndpoint
	baseResults    map[string][]ControllerEndpoint
	stats          entrypointBatchStats164
}

type exactEntrypointRunner164 struct {
	dir     string
	allowed map[string]bool
}

func buildEntrypointScanPlan164(ctx context.Context, repoRoot, runID string, snapshot changeset.Snapshot) (EntrypointScanPlan, error) {
	if strings.TrimSpace(runID) == "" {
		return EntrypointScanPlan{}, errors.New("ENTRYPOINT_INVENTORY_RUN_ID_REQUIRED")
	}
	current, baseCandidates, err := classifyEntrypointPaths164(repoRoot, snapshot)
	if err != nil {
		return EntrypointScanPlan{}, err
	}
	stats := entrypointBatchStats164{}
	baseSources, basePaths, err := loadEntrypointBaseSources164(ctx, repoRoot, snapshot, baseCandidates, &stats)
	if err != nil {
		return EntrypointScanPlan{}, err
	}
	plan := newEntrypointScanPlan164(runID, snapshot, current, basePaths)
	plan.baseSources = cloneBaseSources164(baseSources)
	plan.baseGitBatchProcesses = stats.BaseGitBatchProcesses
	plan.mergeBaseProcesses = stats.MergeBaseProcesses
	return plan, nil
}

func newSnapshotScopedEntrypointScanner164(repoRoot, astGrepPath string, plan EntrypointScanPlan, snapshot changeset.Snapshot) *snapshotScopedEntrypointScanner164 {
	snapshotPaths := map[string]bool{}
	for _, changed := range snapshot.Files {
		if p, ok := exactProductionJavaPath164(changed.Path); ok {
			snapshotPaths[p] = true
		}
	}
	return &snapshotScopedEntrypointScanner164{
		repoRoot: repoRoot,
		astGrepPath: astGrepPath,
		plan: plan,
		snapshotPaths: snapshotPaths,
		current: pathSet164(plan.CurrentPaths),
		base: pathSet164(plan.BasePaths),
		stats: entrypointBatchStats164{
			CurrentRequestedFiles: len(plan.CurrentPaths),
			BaseRequestedFiles: len(plan.BasePaths),
			BaseGitBatchProcesses: plan.baseGitBatchProcesses,
			MergeBaseProcesses: plan.mergeBaseProcesses,
		},
	}
}

func (s *snapshotScopedEntrypointScanner164) Current(ctx context.Context, p string) ([]ControllerEndpoint, error) {
	clean, ok := exactProductionJavaPath164(p)
	if !ok || !s.snapshotPaths[clean] {
		return nil, fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: current path %q", p)
	}
	if !s.current[clean] {
		return nil, nil
	}
	if err := s.ensureBatch164(ctx); err != nil {
		return nil, err
	}
	return append([]ControllerEndpoint(nil), s.currentResults[clean]...), nil
}

func (s *snapshotScopedEntrypointScanner164) Base(ctx context.Context, snapshot changeset.Snapshot, p string) ([]ControllerEndpoint, error) {
	clean, ok := exactProductionJavaPath164(p)
	if !ok || !s.snapshotPaths[clean] {
		return nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_OUT_OF_SCOPE: %q", p)
	}
	if strings.TrimSpace(snapshot.MergeBase) != "" && strings.TrimSpace(s.plan.MergeBase) != "" && strings.TrimSpace(snapshot.MergeBase) != strings.TrimSpace(s.plan.MergeBase) {
		return nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_OUT_OF_SCOPE: mergeBase changed from %s to %s", s.plan.MergeBase, snapshot.MergeBase)
	}
	if !s.base[clean] {
		return nil, nil
	}
	if err := s.ensureBatch164(ctx); err != nil {
		return nil, err
	}
	return append([]ControllerEndpoint(nil), s.baseResults[clean]...), nil
}

func (s *snapshotScopedEntrypointScanner164) ensureBatch164(ctx context.Context) error {
	s.once.Do(func() {
		tmpRoot, err := os.MkdirTemp("", "codea-harness-entrypoint-batch-")
		if err != nil {
			s.initErr = fmt.Errorf("ENTRYPOINT_BATCH_TEMP_CREATE_FAILED: %w", err)
			return
		}
		if insideRepository164(s.repoRoot, tmpRoot) {
			_ = os.RemoveAll(tmpRoot)
			s.initErr = errors.New("ENTRYPOINT_BATCH_TEMP_SCOPE_INVALID: temp workspace must be outside repository")
			return
		}
		defer func() {
			if cleanupErr := os.RemoveAll(tmpRoot); cleanupErr != nil && s.initErr == nil {
				s.initErr = fmt.Errorf("ENTRYPOINT_BATCH_TEMP_CLEANUP_FAILED: %w", cleanupErr)
			}
		}()

		currentRoot := filepath.Join(tmpRoot, "current")
		baseRoot := filepath.Join(tmpRoot, "base")
		if err := materializeCurrentEntrypointSources164(s.repoRoot, currentRoot, s.plan.CurrentPaths); err != nil {
			s.initErr = err
			return
		}
		if err := materializeBaseEntrypointSources164(baseRoot, s.plan.BasePaths, s.plan.baseSources); err != nil {
			s.initErr = err
			return
		}
		s.currentResults, err = scanEntrypointSide164(ctx, currentRoot, s.astGrepPath, s.plan.CurrentPaths, &s.stats.CurrentASTProcesses)
		if err != nil {
			s.initErr = fmt.Errorf("ENTRYPOINT_CURRENT_SCAN_FAILED: %w", err)
			return
		}
		if len(s.plan.CurrentPaths) > 0 {
			s.stats.CurrentScannedFiles = len(s.plan.CurrentPaths)
		}
		s.baseResults, err = scanEntrypointSide164(ctx, baseRoot, s.astGrepPath, s.plan.BasePaths, &s.stats.BaseASTProcesses)
		if err != nil {
			s.initErr = fmt.Errorf("ENTRYPOINT_BASE_SCAN_FAILED: %w", err)
			return
		}
		if len(s.plan.BasePaths) > 0 {
			s.stats.BaseScannedFiles = len(s.plan.BasePaths)
		}
	})
	return s.initErr
}

func (s *snapshotScopedEntrypointScanner164) stats164() entrypointBatchStats164 {
	return s.stats
}

// exactEntrypointRunner164 is retained as the Task 1A single-file runner
// regression. Production Task 1B uses exactBatchEntrypointRunner164.
func (r exactEntrypointRunner164) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if len(args) == 0 {
		return nil, errors.New("ENTRYPOINT_SCAN_SCOPE_WIDENED: missing exact file target")
	}
	target := filepath.ToSlash(args[len(args)-1])
	clean, ok := exactProductionJavaPath164(target)
	if !ok || clean != target || !r.allowed[target] {
		return nil, fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: target=%q", target)
	}
	full := filepath.Join(r.dir, filepath.FromSlash(target))
	info, err := os.Stat(full)
	if err != nil {
		return nil, fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: target=%q: %w", target, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: target=%q is not a regular file", target)
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = r.dir
	return cmd.Output()
}

func validateEntrypointResults164(requested string, endpoints []ControllerEndpoint) error {
	requested = filepath.ToSlash(requested)
	for _, ep := range endpoints {
		if filepath.ToSlash(ep.Path) != requested {
			return fmt.Errorf("ENTRYPOINT_SCAN_RESULT_OUT_OF_SCOPE: requested=%s result=%s", requested, filepath.ToSlash(ep.Path))
		}
	}
	return nil
}

func exactProductionJavaPath164(value string) (string, bool) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" || strings.ContainsAny(value, "\x00\r\n*?[") || path.IsAbs(value) {
		return "", false
	}
	clean := path.Clean(value)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || clean != value {
		return "", false
	}
	if !isProductionJava153(clean) {
		return "", false
	}
	return clean, true
}

func pathSet164(paths []string) map[string]bool {
	out := make(map[string]bool, len(paths))
	for _, p := range paths { out[p] = true }
	return out
}

func hashEntrypointScope164(paths []string) string {
	canonical, _ := json.Marshal(paths)
	return fmt.Sprintf("%x", sha256.Sum256(canonical))
}

func cloneBaseSources164(in map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(in))
	for p, content := range in {
		out[p] = append([]byte(nil), content...)
	}
	return out
}
