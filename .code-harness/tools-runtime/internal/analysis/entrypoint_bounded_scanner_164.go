package analysis

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"codea-harness-tools/internal/changeset"
)

type gitBatchBaseSourceReader164 struct{}

func (gitBatchBaseSourceReader164) ReadBaseSources(ctx context.Context, repoRoot, mergeBase string, paths []string) (map[string][]byte, error) {
	if strings.TrimSpace(mergeBase) == "" {
		return nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_OUT_OF_SCOPE: mergeBase is required")
	}
	clean, ok := exactEntrypointPathSet164(paths)
	if !ok {
		return nil, fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: invalid base source request %v", paths)
	}
	if len(clean) == 0 {
		return map[string][]byte{}, nil
	}

	var input strings.Builder
	for _, p := range clean {
		input.WriteString(mergeBase)
		input.WriteString(":")
		input.WriteString(p)
		input.WriteByte('\n')
	}
	cmd := exec.CommandContext(ctx, "git", "cat-file", "--batch")
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(input.String())
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_BATCH_FAILED: %w", err)
	}

	reader := bufio.NewReader(bytes.NewReader(out))
	result := make(map[string][]byte, len(clean))
	for _, p := range clean {
		header, readErr := reader.ReadString('\n')
		if readErr != nil {
			return nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_BATCH_FAILED: %s header: %w", p, readErr)
		}
		header = strings.TrimSuffix(header, "\n")
		header = strings.TrimSuffix(header, "\r")
		if strings.HasSuffix(header, " missing") {
			continue
		}
		fields := strings.Fields(header)
		if len(fields) != 3 || fields[1] != "blob" {
			return nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_OUT_OF_SCOPE: unexpected cat-file header for %s: %q", p, header)
		}
		size, sizeErr := strconv.Atoi(fields[2])
		if sizeErr != nil || size < 0 {
			return nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_BATCH_FAILED: invalid size for %s: %q", p, fields[2])
		}
		data := make([]byte, size)
		if _, readErr := io.ReadFull(reader, data); readErr != nil {
			return nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_BATCH_FAILED: %s body: %w", p, readErr)
		}
		terminator, readErr := reader.ReadByte()
		if readErr != nil || terminator != '\n' {
			return nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_BATCH_FAILED: %s terminator", p)
		}
		result[p] = data
	}
	if extra, _ := io.ReadAll(reader); len(bytes.TrimSpace(extra)) != 0 {
		return nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_OUT_OF_SCOPE: unexpected cat-file output")
	}
	return result, nil
}

type plannedEntrypointScanner164 struct {
	plan        EntrypointScanPlan
	repoRoot    string
	astGrepPath string
	baseSources map[string][]byte
}

func (s plannedEntrypointScanner164) Current(ctx context.Context, p string) ([]ControllerEndpoint, error) {
	if err := validateEntrypointMember164(s.plan, entrypointScanSideCurrent164, p); err != nil {
		return nil, err
	}
	delegate := navigationEntrypointScanner153{repoRoot: s.repoRoot, astGrepPath: s.astGrepPath}
	out, err := delegate.scanAtRoot(ctx, s.repoRoot, filepath.ToSlash(p))
	if err != nil {
		return nil, err
	}
	if err := validateEntrypointScanResults164([]string{filepath.ToSlash(p)}, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s plannedEntrypointScanner164) Base(ctx context.Context, snapshot changeset.Snapshot, p string) ([]ControllerEndpoint, error) {
	if strings.TrimSpace(snapshot.MergeBase) != s.plan.MergeBase {
		return nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_OUT_OF_SCOPE: snapshot mergeBase changed")
	}
	if err := validateEntrypointMember164(s.plan, entrypointScanSideBase164, p); err != nil {
		return nil, err
	}
	clean := filepath.ToSlash(p)
	content, ok := s.baseSources[clean]
	if !ok {
		return nil, fmt.Errorf("ENTRYPOINT_BASE_SOURCE_OUT_OF_SCOPE: missing planned base source %s", clean)
	}
	tmp, err := os.MkdirTemp("", "codea-harness-entrypoint-base-164-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	tmpFile := filepath.Join(tmp, filepath.FromSlash(clean))
	if err := os.MkdirAll(filepath.Dir(tmpFile), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(tmpFile, content, 0o600); err != nil {
		return nil, err
	}
	delegate := navigationEntrypointScanner153{repoRoot: s.repoRoot, astGrepPath: s.astGrepPath}
	out, err := delegate.scanAtRoot(ctx, tmp, clean)
	if err != nil {
		return nil, err
	}
	if err := validateEntrypointScanResults164([]string{clean}, out); err != nil {
		return nil, err
	}
	return out, nil
}

func validateEntrypointMember164(plan EntrypointScanPlan, side, raw string) error {
	clean := filepath.ToSlash(raw)
	var expected []string
	switch side {
	case entrypointScanSideCurrent164:
		expected = plan.CurrentPaths
	case entrypointScanSideBase164:
		expected = plan.BasePaths
	default:
		return fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: unknown side %q", side)
	}
	for _, p := range expected {
		if p == clean {
			return nil
		}
	}
	return fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: side=%s path=%s", side, clean)
}
