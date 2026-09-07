package analysis

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"codea-harness-tools/internal/changeset"
	"codea-harness-tools/internal/nav"
)

type entrypointBatchStats164 struct {
	CurrentRequestedFiles  int
	BaseRequestedFiles     int
	CurrentScannedFiles    int
	BaseScannedFiles       int
	CurrentASTProcesses    int
	BaseASTProcesses       int
	BaseGitBatchProcesses  int
	MergeBaseProcesses     int
}

type countedEntrypointRunner164 struct {
	inner nav.Runner
	count *int
}

func (r countedEntrypointRunner164) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	*r.count++
	return r.inner.Run(ctx, name, args...)
}

type exactBatchEntrypointRunner164 struct {
	dir     string
	allowed map[string]bool
}

func (r exactBatchEntrypointRunner164) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if len(r.allowed) == 0 || len(args) < len(r.allowed) {
		return nil, errors.New("ENTRYPOINT_SCAN_SCOPE_WIDENED: exact targets missing")
	}
	targets := args[len(args)-len(r.allowed):]
	seen := map[string]bool{}
	for _, raw := range targets {
		p, ok := exactProductionJavaPath164(raw)
		if !ok || !r.allowed[p] || seen[p] {
			return nil, fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: target=%q", raw)
		}
		full := filepath.Join(r.dir, filepath.FromSlash(p))
		info, err := os.Stat(full)
		if err != nil || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: target=%q is not an exact regular file", raw)
		}
		seen[p] = true
	}
	if len(seen) != len(r.allowed) {
		return nil, fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: requested=%d allowed=%d", len(seen), len(r.allowed))
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = r.dir
	return cmd.Output()
}

func buildEntrypointInventoryBatch164(ctx context.Context, repoRoot, runID string, snapshot changeset.Snapshot, intent Intent, astGrepPath string) (inventory EntrypointInventory, plan EntrypointScanPlan, stats entrypointBatchStats164, err error) {
	if strings.TrimSpace(runID) == "" {
		err = errors.New("ENTRYPOINT_INVENTORY_RUN_ID_REQUIRED")
		return
	}
	mode := strings.ToUpper(strings.TrimSpace(intent.Mode))
	switch mode {
	case "FULL", "LIST", "TARGETED", "CHAIN_MAINTENANCE":
	default:
		err = fmt.Errorf("ENTRYPOINT_INVENTORY_INTENT_INVALID: %s", intent.Mode)
		return
	}
	if (mode == "TARGETED" || mode == "CHAIN_MAINTENANCE") && strings.TrimSpace(intent.Target) == "" {
		err = errors.New("ENTRYPOINT_INVENTORY_TARGET_REQUIRED")
		return
	}

	currentPaths, baseCandidates, classifyErr := classifyEntrypointPaths164(repoRoot, snapshot)
	if classifyErr != nil {
		err = classifyErr
		return
	}
	baseSources, basePaths, loadErr := loadEntrypointBaseSources164(ctx, repoRoot, snapshot, baseCandidates, &stats)
	if loadErr != nil {
		err = loadErr
		return
	}
	plan = newEntrypointScanPlan164(runID, snapshot, currentPaths, basePaths)
	stats.CurrentRequestedFiles = len(plan.CurrentPaths)
	stats.BaseRequestedFiles = len(plan.BasePaths)

	tmpRoot, tmpErr := os.MkdirTemp("", "codea-harness-entrypoint-batch-")
	if tmpErr != nil {
		err = fmt.Errorf("ENTRYPOINT_BATCH_TEMP_CREATE_FAILED: %w", tmpErr)
		return
	}
	if insideRepository164(repoRoot, tmpRoot) {
		_ = os.RemoveAll(tmpRoot)
		err = errors.New("ENTRYPOINT_BATCH_TEMP_SCOPE_INVALID: temp workspace must be outside repository")
		return
	}
	defer func() {
		if cleanupErr := os.RemoveAll(tmpRoot); cleanupErr != nil && err == nil {
			err = fmt.Errorf("ENTRYPOINT_BATCH_TEMP_CLEANUP_FAILED: %w", cleanupErr)
		}
	}()

	currentRoot := filepath.Join(tmpRoot, "current")
	baseRoot := filepath.Join(tmpRoot, "base")
	if materializeErr := materializeCurrentEntrypointSources164(repoRoot, currentRoot, plan.CurrentPaths); materializeErr != nil {
		err = materializeErr
		return
	}
	if materializeErr := materializeBaseEntrypointSources164(baseRoot, plan.BasePaths, baseSources); materializeErr != nil {
		err = materializeErr
		return
	}

	current, scanErr := scanEntrypointSide164(ctx, currentRoot, astGrepPath, plan.CurrentPaths, &stats.CurrentASTProcesses)
	if scanErr != nil {
		err = fmt.Errorf("ENTRYPOINT_CURRENT_SCAN_FAILED: %w", scanErr)
		return
	}
	if len(plan.CurrentPaths) > 0 {
		stats.CurrentScannedFiles = len(plan.CurrentPaths)
	}
	base, scanErr := scanEntrypointSide164(ctx, baseRoot, astGrepPath, plan.BasePaths, &stats.BaseASTProcesses)
	if scanErr != nil {
		err = fmt.Errorf("ENTRYPOINT_BASE_SCAN_FAILED: %w", scanErr)
		return
	}
	if len(plan.BasePaths) > 0 {
		stats.BaseScannedFiles = len(plan.BasePaths)
	}

	byKey := map[string]ExpectedEntrypoint{}
	for _, changed := range snapshot.Files {
		p, ok := exactProductionJavaPath164(changed.Path)
		if !ok {
			if isProductionJava153(changed.Path) {
				err = fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: invalid canonical path %q", changed.Path)
				return
			}
			continue
		}
		currentEndpoints := current[p]
		baseEndpoints := base[p]
		status := strings.ToUpper(strings.TrimSpace(changed.Status))
		switch status {
		case "A":
			for _, ep := range currentEndpoints { addExpected153(byKey, ep, "") }
		case "D":
			for _, ep := range baseEndpoints { addExpected153(byKey, ep, DispositionRemoved) }
		default:
			collectModifiedEntrypoints153(byKey, changed, currentEndpoints, baseEndpoints)
		}
	}

	items := make([]ExpectedEntrypoint, 0, len(byKey))
	for _, ep := range byKey {
		if targetAllowsEntrypoint153(intent, ep.Symbol) {
			items = append(items, ep)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Symbol != items[j].Symbol { return items[i].Symbol < items[j].Symbol }
		if items[i].Path != items[j].Path { return items[i].Path < items[j].Path }
		return items[i].Disposition < items[j].Disposition
	})
	inventory = EntrypointInventory{
		RunID: runID,
		Status: inventoryComplete153,
		ExpectedEntrypoints: items,
		ChangeSetSHA256: snapshot.SHA256,
	}
	return
}

func classifyEntrypointPaths164(repoRoot string, snapshot changeset.Snapshot) ([]string, []string, error) {
	current := make([]string, 0, len(snapshot.Files))
	baseCandidates := make([]string, 0, len(snapshot.Files))
	for _, changed := range snapshot.Files {
		p, ok := exactProductionJavaPath164(changed.Path)
		if !ok {
			if isProductionJava153(changed.Path) {
				return nil, nil, fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: invalid canonical path %q", changed.Path)
			}
			continue
		}
		full := filepath.Join(repoRoot, filepath.FromSlash(p))
		if info, statErr := os.Stat(full); statErr == nil {
			if !info.Mode().IsRegular() {
				return nil, nil, fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: current source is not regular file %s", p)
			}
			current = append(current, p)
		} else if !os.IsNotExist(statErr) {
			return nil, nil, fmt.Errorf("ENTRYPOINT_CURRENT_SOURCE_STAT_FAILED: %s: %w", p, statErr)
		}
		if strings.ToUpper(strings.TrimSpace(changed.Status)) != "A" {
			baseCandidates = append(baseCandidates, p)
		}
	}
	sort.Strings(current)
	sort.Strings(baseCandidates)
	return current, baseCandidates, nil
}

func newEntrypointScanPlan164(runID string, snapshot changeset.Snapshot, currentPaths, basePaths []string) EntrypointScanPlan {
	current := append([]string(nil), currentPaths...)
	base := append([]string(nil), basePaths...)
	sort.Strings(current)
	sort.Strings(base)
	snapshotHash := strings.TrimSpace(snapshot.SnapshotSHA256)
	if snapshotHash == "" { snapshotHash = strings.TrimSpace(snapshot.SHA256) }
	return EntrypointScanPlan{
		RunID: runID,
		SnapshotSHA256: snapshotHash,
		MergeBase: strings.TrimSpace(snapshot.MergeBase),
		CurrentPaths: current,
		BasePaths: base,
		CurrentScopeSHA256: hashEntrypointScope164(current),
		BaseScopeSHA256: hashEntrypointScope164(base),
	}
}

func loadEntrypointBaseSources164(ctx context.Context, repoRoot string, snapshot changeset.Snapshot, candidates []string, stats *entrypointBatchStats164) (map[string][]byte, []string, error) {
	sources := map[string][]byte{}
	if len(candidates) == 0 {
		return sources, nil, nil
	}
	mergeBase := strings.TrimSpace(snapshot.MergeBase)
	if mergeBase == "" {
		cmd := exec.CommandContext(ctx, "git", "merge-base", snapshot.BaseRef, "HEAD")
		cmd.Dir = repoRoot
		out, cmdErr := cmd.CombinedOutput()
		stats.MergeBaseProcesses++
		if cmdErr != nil {
			return nil, nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_LOAD_FAILED: merge-base: %w: %s", cmdErr, strings.TrimSpace(string(out)))
		}
		mergeBase = strings.TrimSpace(string(out))
	}
	if mergeBase == "" {
		return nil, nil, errors.New("ENTRYPOINT_BASE_SOURCE_LOAD_FAILED: mergeBase unavailable")
	}

	var request strings.Builder
	for _, p := range candidates {
		clean, ok := exactProductionJavaPath164(p)
		if !ok || clean != p || strings.ContainsAny(p, "\x00\r\n") {
			return nil, nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_OUT_OF_SCOPE: %q", p)
		}
		request.WriteString(mergeBase)
		request.WriteByte(':')
		request.WriteString(p)
		request.WriteByte('\n')
	}
	cmd := exec.CommandContext(ctx, "git", "cat-file", "--batch")
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(request.String())
	out, cmdErr := cmd.Output()
	stats.BaseGitBatchProcesses++
	if cmdErr != nil {
		return nil, nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_LOAD_FAILED: git cat-file --batch: %w", cmdErr)
	}

	reader := bufio.NewReader(bytes.NewReader(out))
	basePaths := make([]string, 0, len(candidates))
	for _, p := range candidates {
		headerBytes, readErr := reader.ReadBytes('\n')
		if readErr != nil {
			return nil, nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_LOAD_FAILED: header for %s: %w", p, readErr)
		}
		header := strings.TrimSpace(string(headerBytes))
		if strings.HasSuffix(header, " missing") {
			continue
		}
		fields := strings.Fields(header)
		if len(fields) != 3 || fields[1] != "blob" {
			return nil, nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_LOAD_FAILED: unexpected header for %s: %q", p, header)
		}
		size, sizeErr := strconv.Atoi(fields[2])
		if sizeErr != nil || size < 0 {
			return nil, nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_LOAD_FAILED: invalid size for %s: %q", p, fields[2])
		}
		content := make([]byte, size)
		if _, readErr := io.ReadFull(reader, content); readErr != nil {
			return nil, nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_LOAD_FAILED: content for %s: %w", p, readErr)
		}
		separator, readErr := reader.ReadByte()
		if readErr != nil || separator != '\n' {
			return nil, nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_LOAD_FAILED: malformed separator for %s", p)
		}
		sources[p] = content
		basePaths = append(basePaths, p)
	}
	if rest, _ := io.ReadAll(reader); len(bytes.TrimSpace(rest)) != 0 {
		return nil, nil, errors.New("ENTRYPOINT_BASE_SOURCE_LOAD_FAILED: unexpected trailing batch output")
	}
	return sources, basePaths, nil
}

func materializeCurrentEntrypointSources164(repoRoot, currentRoot string, paths []string) error {
	for _, p := range paths {
		if _, ok := exactProductionJavaPath164(p); !ok {
			return fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: current source %q", p)
		}
		content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(p)))
		if err != nil {
			return fmt.Errorf("ENTRYPOINT_CURRENT_SOURCE_READ_FAILED: %s: %w", p, err)
		}
		if err := writeExactEntrypointSource164(currentRoot, p, content); err != nil { return err }
	}
	return nil
}

func materializeBaseEntrypointSources164(baseRoot string, paths []string, sources map[string][]byte) error {
	allowed := pathSet164(paths)
	for sourcePath := range sources {
		if !allowed[sourcePath] {
			return fmt.Errorf("ENTRYPOINT_BASE_SOURCE_OUT_OF_SCOPE: %s", sourcePath)
		}
	}
	for _, p := range paths {
		content, ok := sources[p]
		if !ok {
			return fmt.Errorf("ENTRYPOINT_BASE_SOURCE_OUT_OF_SCOPE: planned source missing %s", p)
		}
		if err := writeExactEntrypointSource164(baseRoot, p, content); err != nil { return err }
	}
	return nil
}

func writeExactEntrypointSource164(root, p string, content []byte) error {
	clean, ok := exactProductionJavaPath164(p)
	if !ok || clean != p {
		return fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: materialize path %q", p)
	}
	full := filepath.Join(root, filepath.FromSlash(clean))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil { return err }
	if err := os.WriteFile(full, content, 0o600); err != nil { return err }
	return nil
}

func scanEntrypointSide164(ctx context.Context, root, astGrepPath string, paths []string, processCount *int) (map[string][]ControllerEndpoint, error) {
	result := make(map[string][]ControllerEndpoint, len(paths))
	if len(paths) == 0 {
		return result, nil
	}
	allowed := pathSet164(paths)
	runner := countedEntrypointRunner164{
		inner: exactBatchEntrypointRunner164{dir: root, allowed: allowed},
		count: processCount,
	}
	n := nav.Navigator{RepoRoot: root, AstGrepPath: astGrepPath, Runner: runner}
	matches, err := n.FindControllerEndpointsBatch(ctx, paths)
	if err != nil {
		return nil, err
	}
	for _, p := range paths { result[p] = nil }
	for _, match := range matches {
		p := filepath.ToSlash(match.Path)
		if !allowed[p] {
			return nil, fmt.Errorf("ENTRYPOINT_SCAN_RESULT_OUT_OF_SCOPE: result=%s", p)
		}
		result[p] = append(result[p], ControllerEndpoint{
			Controller: match.Controller,
			Symbol: match.Symbol,
			Path: p,
			ControllerStartLine: match.ControllerStartLine,
			ControllerEndLine: match.ControllerEndLine,
			StartLine: match.StartLine,
			EndLine: match.EndLine,
		})
	}
	for p := range result {
		sort.Slice(result[p], func(i, j int) bool {
			if result[p][i].Symbol != result[p][j].Symbol { return result[p][i].Symbol < result[p][j].Symbol }
			return result[p][i].StartLine < result[p][j].StartLine
		})
	}
	return result, nil
}

func insideRepository164(repoRoot, candidate string) bool {
	repoAbs, repoErr := filepath.Abs(repoRoot)
	candidateAbs, candidateErr := filepath.Abs(candidate)
	if repoErr != nil || candidateErr != nil { return true }
	rel, err := filepath.Rel(repoAbs, candidateAbs)
	if err != nil { return true }
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
