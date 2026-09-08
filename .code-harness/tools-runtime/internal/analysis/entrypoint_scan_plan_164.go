package analysis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codea-harness-tools/internal/changeset"
	"codea-harness-tools/internal/projectpath"
)

const (
	entrypointScanSideCurrent164 = "CURRENT"
	entrypointScanSideBase164    = "BASE"
)

type EntrypointScanPlan struct {
	RunID              string
	SnapshotSHA256     string
	MergeBase          string
	CurrentPaths       []string
	BasePaths          []string
	CurrentScopeSHA256 string
	BaseScopeSHA256    string
}

type entrypointBaseSourceReader164 interface {
	ReadBaseSources(context.Context, string, string, []string) (map[string][]byte, error)
}

func buildEntrypointScanPlan164(ctx context.Context, repoRoot, runID string, snapshot changeset.Snapshot, reader entrypointBaseSourceReader164) (EntrypointScanPlan, map[string][]byte, error) {
	if strings.TrimSpace(runID) == "" {
		return EntrypointScanPlan{}, nil, fmt.Errorf("ENTRYPOINT_INVENTORY_RUN_ID_REQUIRED")
	}
	if strings.TrimSpace(snapshot.MergeBase) == "" {
		return EntrypointScanPlan{}, nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_OUT_OF_SCOPE: snapshot mergeBase is required")
	}
	if reader == nil {
		return EntrypointScanPlan{}, nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_OUT_OF_SCOPE: base source reader is required")
	}
	candidateSet := map[string]bool{}
	for _, changed := range snapshot.Files {
		if err := validateEntrypointBatchProtocolPath164(changed.Path); err != nil {
			return EntrypointScanPlan{}, nil, err
		}
		p, ok := projectpath.Normalize(changed.Path)
		if !ok || !projectpath.IsMainJava(p) {
			continue
		}
		candidateSet[p] = true
	}
	candidates := sortedEntrypointPaths164(candidateSet)
	currentSet := map[string]bool{}
	for _, p := range candidates {
		full := filepath.Join(repoRoot, filepath.FromSlash(p))
		info, err := os.Stat(full)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return EntrypointScanPlan{}, nil, fmt.Errorf("ENTRYPOINT_CURRENT_SOURCE_STAT_FAILED: %s: %w", p, err)
		}
		if !info.Mode().IsRegular() {
			return EntrypointScanPlan{}, nil, fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: current source is not a regular file: %s", p)
		}
		currentSet[p] = true
	}
	baseSources, err := reader.ReadBaseSources(ctx, repoRoot, snapshot.MergeBase, candidates)
	if err != nil {
		return EntrypointScanPlan{}, nil, err
	}
	baseSet := map[string]bool{}
	for raw := range baseSources {
		if err := validateEntrypointBatchProtocolPath164(raw); err != nil {
			return EntrypointScanPlan{}, nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_OUT_OF_SCOPE: %w", err)
		}
		p, ok := projectpath.Normalize(raw)
		if !ok || p != filepath.ToSlash(raw) || !candidateSet[p] || !projectpath.IsMainJava(p) {
			return EntrypointScanPlan{}, nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_OUT_OF_SCOPE: %s", raw)
		}
		baseSet[p] = true
	}
	snapshotSHA := strings.TrimSpace(snapshot.SnapshotSHA256)
	if snapshotSHA == "" {
		snapshotSHA = strings.TrimSpace(snapshot.SHA256)
	}
	plan := EntrypointScanPlan{RunID: runID, SnapshotSHA256: snapshotSHA, MergeBase: snapshot.MergeBase, CurrentPaths: sortedEntrypointPaths164(currentSet), BasePaths: sortedEntrypointPaths164(baseSet)}
	plan.CurrentScopeSHA256 = hashEntrypointScope164(plan, entrypointScanSideCurrent164, plan.CurrentPaths)
	plan.BaseScopeSHA256 = hashEntrypointScope164(plan, entrypointScanSideBase164, plan.BasePaths)
	return plan, baseSources, nil
}

func validateEntrypointScanPaths164(plan EntrypointScanPlan, side string, requested []string) error {
	var expected []string
	switch side {
	case entrypointScanSideCurrent164:
		expected = plan.CurrentPaths
	case entrypointScanSideBase164:
		expected = plan.BasePaths
	default:
		return fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: unknown scan side %q", side)
	}
	clean, ok := exactEntrypointPathSet164(requested)
	if !ok || !equalStringSlices164(clean, expected) {
		return fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: side=%s requested=%v expected=%v", side, clean, expected)
	}
	return nil
}

func validateEntrypointScanResults164(requested []string, results []ControllerEndpoint) error {
	allowed, ok := exactEntrypointPathSet164(requested)
	if !ok {
		return fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: invalid requested paths %v", requested)
	}
	set := make(map[string]bool, len(allowed))
	for _, p := range allowed {
		set[p] = true
	}
	for _, result := range results {
		if validateEntrypointBatchProtocolPath164(result.Path) != nil {
			return fmt.Errorf("ENTRYPOINT_SCAN_RESULT_OUT_OF_SCOPE: %s", result.Path)
		}
		p, valid := projectpath.Normalize(result.Path)
		if !valid || !set[p] {
			return fmt.Errorf("ENTRYPOINT_SCAN_RESULT_OUT_OF_SCOPE: %s", result.Path)
		}
	}
	return nil
}

func exactEntrypointPathSet164(paths []string) ([]string, bool) {
	set := map[string]bool{}
	for _, raw := range paths {
		if validateEntrypointBatchProtocolPath164(raw) != nil {
			return nil, false
		}
		p, ok := projectpath.Normalize(raw)
		if !ok || p != filepath.ToSlash(raw) || !projectpath.IsMainJava(p) || set[p] {
			return nil, false
		}
		set[p] = true
	}
	return sortedEntrypointPaths164(set), true
}

func validateEntrypointBatchProtocolPath164(raw string) error {
	if strings.ContainsAny(raw, "\x00\r\n") {
		return fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: batch protocol path contains NUL/CR/LF")
	}
	return nil
}

func sortedEntrypointPaths164(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func equalStringSlices164(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func hashEntrypointScope164(plan EntrypointScanPlan, side string, paths []string) string {
	h := sha256.New()
	_, _ = io.WriteString(h, plan.RunID+"\n"+plan.SnapshotSHA256+"\n"+plan.MergeBase+"\n"+side+"\n")
	for _, p := range paths {
		_, _ = io.WriteString(h, p+"\n")
	}
	return hex.EncodeToString(h.Sum(nil))
}
