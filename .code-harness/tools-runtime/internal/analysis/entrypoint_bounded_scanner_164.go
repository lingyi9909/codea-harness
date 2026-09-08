package analysis

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type gitBatchBaseSourceReader164 struct {
	metrics *entrypointExecutionMetrics164
}

func (r gitBatchBaseSourceReader164) ReadBaseSources(ctx context.Context, repoRoot, mergeBase string, paths []string) (map[string][]byte, error) {
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
	if r.metrics != nil {
		r.metrics.BaseGitBatchProcessCount++
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
		header = strings.TrimSuffix(strings.TrimSuffix(header, "\n"), "\r")
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
