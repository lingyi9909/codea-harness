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
	"sort"
	"strings"

	"codea-harness-tools/internal/changeset"
	"codea-harness-tools/internal/nav"
)

// EntrypointScanPlan is Runtime-owned scope authority derived from the
// freshness-validated canonical ChangeSet. It is internal execution evidence,
// not Agent/Reviewer input and not part of certification hashes.
type EntrypointScanPlan struct {
	RunID               string
	SnapshotSHA256      string
	MergeBase           string
	CurrentPaths        []string
	BasePaths           []string
	CurrentScopeSHA256  string
	BaseScopeSHA256     string
}

type snapshotScopedEntrypointScanner164 struct {
	repoRoot      string
	astGrepPath   string
	plan          EntrypointScanPlan
	snapshotPaths map[string]bool
	current       map[string]bool
	base          map[string]bool
}

type exactEntrypointRunner164 struct {
	dir     string
	allowed map[string]bool
}

func buildEntrypointScanPlan164(ctx context.Context, repoRoot, runID string, snapshot changeset.Snapshot) (EntrypointScanPlan, error) {
	if strings.TrimSpace(runID) == "" {
		return EntrypointScanPlan{}, errors.New("ENTRYPOINT_INVENTORY_RUN_ID_REQUIRED")
	}
	current := make([]string, 0, len(snapshot.Files))
	baseCandidates := make([]string, 0, len(snapshot.Files))
	for _, changed := range snapshot.Files {
		p, ok := exactProductionJavaPath164(changed.Path)
		if !ok {
			if isProductionJava153(changed.Path) {
				return EntrypointScanPlan{}, fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: invalid snapshot path %q", changed.Path)
			}
			continue
		}
		full := filepath.Join(repoRoot, filepath.FromSlash(p))
		if info, err := os.Stat(full); err == nil {
			if !info.Mode().IsRegular() {
				return EntrypointScanPlan{}, fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: current source is not regular file %s", p)
			}
			current = append(current, p)
		} else if !os.IsNotExist(err) {
			return EntrypointScanPlan{}, fmt.Errorf("ENTRYPOINT_CURRENT_SOURCE_STAT_FAILED: %s: %w", p, err)
		}
		if strings.ToUpper(strings.TrimSpace(changed.Status)) != "A" {
			baseCandidates = append(baseCandidates, p)
		}
	}
	sort.Strings(current)
	sort.Strings(baseCandidates)
	base := baseCandidates
	if strings.TrimSpace(snapshot.MergeBase) != "" && len(baseCandidates) > 0 {
		var err error
		base, err = probeBaseExistingPaths164(ctx, repoRoot, snapshot.MergeBase, baseCandidates)
		if err != nil {
			return EntrypointScanPlan{}, err
		}
	}
	snapshotHash := strings.TrimSpace(snapshot.SnapshotSHA256)
	if snapshotHash == "" {
		snapshotHash = strings.TrimSpace(snapshot.SHA256)
	}
	return EntrypointScanPlan{
		RunID:              runID,
		SnapshotSHA256:     snapshotHash,
		MergeBase:          strings.TrimSpace(snapshot.MergeBase),
		CurrentPaths:       current,
		BasePaths:          base,
		CurrentScopeSHA256: hashEntrypointScope164(current),
		BaseScopeSHA256:    hashEntrypointScope164(base),
	}, nil
}

func probeBaseExistingPaths164(ctx context.Context, repoRoot, mergeBase string, candidates []string) ([]string, error) {
	if strings.TrimSpace(mergeBase) == "" || len(candidates) == 0 {
		return append([]string(nil), candidates...), nil
	}
	var request strings.Builder
	for _, p := range candidates {
		if _, ok := exactProductionJavaPath164(p); !ok || strings.ContainsAny(p, "\x00\r\n") {
			return nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_OUT_OF_SCOPE: %q", p)
		}
		request.WriteString(mergeBase)
		request.WriteByte(':')
		request.WriteString(p)
		request.WriteByte('\n')
	}
	cmd := exec.CommandContext(ctx, "git", "cat-file", "--batch-check")
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(request.String())
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_PROBE_FAILED: %w: %s", err, strings.TrimSpace(string(out)))
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != len(candidates) {
		return nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_PROBE_FAILED: responses=%d requests=%d", len(lines), len(candidates))
	}
	base := make([]string, 0, len(candidates))
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasSuffix(line, " missing") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[1] != "blob" {
			return nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_PROBE_FAILED: unexpected response for %s: %q", candidates[i], line)
		}
		base = append(base, candidates[i])
	}
	return base, nil
}

func newSnapshotScopedEntrypointScanner164(repoRoot, astGrepPath string, plan EntrypointScanPlan, snapshot changeset.Snapshot) snapshotScopedEntrypointScanner164 {
	snapshotPaths := map[string]bool{}
	for _, changed := range snapshot.Files {
		if p, ok := exactProductionJavaPath164(changed.Path); ok {
			snapshotPaths[p] = true
		}
	}
	return snapshotScopedEntrypointScanner164{
		repoRoot: repoRoot,
		astGrepPath: astGrepPath,
		plan: plan,
		snapshotPaths: snapshotPaths,
		current: pathSet164(plan.CurrentPaths),
		base: pathSet164(plan.BasePaths),
	}
}

func (s snapshotScopedEntrypointScanner164) Current(ctx context.Context, p string) ([]ControllerEndpoint, error) {
	p, ok := exactProductionJavaPath164(p)
	if !ok || !s.snapshotPaths[p] {
		return nil, fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: current path %q", p)
	}
	if !s.current[p] {
		return nil, nil
	}
	return s.scanExactAtRoot(ctx, s.repoRoot, p)
}

func (s snapshotScopedEntrypointScanner164) Base(ctx context.Context, snapshot changeset.Snapshot, p string) ([]ControllerEndpoint, error) {
	p, ok := exactProductionJavaPath164(p)
	if !ok || !s.snapshotPaths[p] {
		return nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_OUT_OF_SCOPE: %q", p)
	}
	if !s.base[p] {
		return nil, nil
	}
	mergeBase := strings.TrimSpace(snapshot.MergeBase)
	if mergeBase == "" {
		mergeBaseCmd := exec.CommandContext(ctx, "git", "merge-base", snapshot.BaseRef, "HEAD")
		mergeBaseCmd.Dir = s.repoRoot
		mergeBaseBytes, err := mergeBaseCmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("git merge-base %s HEAD: %w: %s", snapshot.BaseRef, err, strings.TrimSpace(string(mergeBaseBytes)))
		}
		mergeBase = strings.TrimSpace(string(mergeBaseBytes))
	}
	object := mergeBase + ":" + p
	show := exec.CommandContext(ctx, "git", "show", object)
	show.Dir = s.repoRoot
	content, err := show.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, nil
		}
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "codea-harness-entrypoint-base-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	tmpFile := filepath.Join(tmp, filepath.FromSlash(p))
	if err := os.MkdirAll(filepath.Dir(tmpFile), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(tmpFile, content, 0o600); err != nil {
		return nil, err
	}
	return s.scanExactAtRoot(ctx, tmp, p)
}

func (s snapshotScopedEntrypointScanner164) scanExactAtRoot(ctx context.Context, scanRoot, scope string) ([]ControllerEndpoint, error) {
	if _, ok := exactProductionJavaPath164(scope); !ok {
		return nil, fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: %q", scope)
	}
	full := filepath.Join(scanRoot, filepath.FromSlash(scope))
	info, err := os.Stat(full)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: non-file scope %s", scope)
	}
	allowed := map[string]bool{scope: true}
	n := nav.Navigator{
		RepoRoot: scanRoot,
		AstGrepPath: s.astGrepPath,
		Runner: exactEntrypointRunner164{dir: scanRoot, allowed: allowed},
	}
	matches, err := n.FindControllerEndpoints(ctx, scope)
	if err != nil {
		return nil, err
	}
	out := make([]ControllerEndpoint, 0, len(matches))
	for _, match := range matches {
		matchPath := filepath.ToSlash(match.Path)
		if !allowed[matchPath] {
			return nil, fmt.Errorf("ENTRYPOINT_SCAN_RESULT_OUT_OF_SCOPE: requested=%s result=%s", scope, matchPath)
		}
		out = append(out, ControllerEndpoint{
			Controller: match.Controller,
			Symbol: match.Symbol,
			Path: matchPath,
			ControllerStartLine: match.ControllerStartLine,
			ControllerEndLine: match.ControllerEndLine,
			StartLine: match.StartLine,
			EndLine: match.EndLine,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Symbol != out[j].Symbol { return out[i].Symbol < out[j].Symbol }
		return out[i].StartLine < out[j].StartLine
	})
	return out, nil
}

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
