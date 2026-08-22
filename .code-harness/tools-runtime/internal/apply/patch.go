package apply

import (
	"fmt"
	"strconv"
	"strings"
)

type filePatch struct {
	Path   string
	Create bool
	Delete bool
	Hunks  []hunk
}

type hunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []string
}

func parseUnifiedDiff(diff string) ([]filePatch, error) {
	if strings.TrimSpace(diff) == "" {
		return nil, fmt.Errorf("EMPTY_DIFF")
	}
	if strings.ContainsRune(diff, '\x00') || strings.Contains(diff, "GIT binary patch") || strings.Contains(diff, "Binary files ") {
		return nil, fmt.Errorf("BINARY_PATCH_NOT_SUPPORTED")
	}
	lines := strings.Split(diff, "\n")
	var patches []filePatch
	for i := 0; i < len(lines); {
		line := lines[i]
		if line == "" || strings.HasPrefix(line, "diff --git ") || strings.HasPrefix(line, "index ") || strings.HasPrefix(line, "new file mode ") || strings.HasPrefix(line, "deleted file mode ") {
			i++
			continue
		}
		if !strings.HasPrefix(line, "--- ") {
			return nil, fmt.Errorf("INVALID_UNIFIED_DIFF: expected --- header at line %d", i+1)
		}
		oldName := headerPath(strings.TrimPrefix(line, "--- "))
		i++
		if i >= len(lines) || !strings.HasPrefix(lines[i], "+++ ") {
			return nil, fmt.Errorf("INVALID_UNIFIED_DIFF: missing +++ header")
		}
		newName := headerPath(strings.TrimPrefix(lines[i], "+++ "))
		i++
		create := oldName == "/dev/null"
		deleteFile := newName == "/dev/null"
		if create && deleteFile {
			return nil, fmt.Errorf("INVALID_UNIFIED_DIFF: both paths are /dev/null")
		}
		var target string
		switch {
		case create:
			target = stripDiffPrefix(newName, "b/")
		case deleteFile:
			target = stripDiffPrefix(oldName, "a/")
		default:
			oldPath := stripDiffPrefix(oldName, "a/")
			newPath := stripDiffPrefix(newName, "b/")
			if oldPath != newPath {
				return nil, fmt.Errorf("RENAME_NOT_SUPPORTED: %q -> %q", oldPath, newPath)
			}
			target = oldPath
		}
		clean, err := safeRepoPath(target)
		if err != nil {
			return nil, err
		}
		fp := filePatch{Path: clean, Create: create, Delete: deleteFile}
		for i < len(lines) {
			if strings.HasPrefix(lines[i], "--- ") || strings.HasPrefix(lines[i], "diff --git ") {
				break
			}
			if lines[i] == "" {
				i++
				continue
			}
			if !strings.HasPrefix(lines[i], "@@ ") {
				return nil, fmt.Errorf("INVALID_UNIFIED_DIFF: expected hunk for %q at line %d", clean, i+1)
			}
			h, err := parseHunkHeader(lines[i])
			if err != nil {
				return nil, err
			}
			i++
			for i < len(lines) {
				if strings.HasPrefix(lines[i], "@@ ") || strings.HasPrefix(lines[i], "--- ") || strings.HasPrefix(lines[i], "diff --git ") {
					break
				}
				if lines[i] == "" && i == len(lines)-1 {
					i++
					break
				}
				if strings.HasPrefix(lines[i], "\\ No newline at end of file") {
					return nil, fmt.Errorf("NO_NEWLINE_PATCH_NOT_SUPPORTED")
				}
				if lines[i] == "" {
					return nil, fmt.Errorf("INVALID_UNIFIED_DIFF: unprefixed empty hunk line")
				}
				prefix := lines[i][0]
				if prefix != ' ' && prefix != '+' && prefix != '-' {
					break
				}
				h.Lines = append(h.Lines, lines[i])
				i++
			}
			if err := verifyHunkCounts(h); err != nil {
				return nil, fmt.Errorf("%s: %w", clean, err)
			}
			fp.Hunks = append(fp.Hunks, h)
		}
		if len(fp.Hunks) == 0 {
			return nil, fmt.Errorf("INVALID_UNIFIED_DIFF: %q has no hunks", clean)
		}
		patches = append(patches, fp)
	}
	if len(patches) == 0 {
		return nil, fmt.Errorf("INVALID_UNIFIED_DIFF: no file patches")
	}
	return patches, nil
}

func headerPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if idx := strings.IndexAny(raw, "\t "); idx >= 0 {
		raw = raw[:idx]
	}
	return raw
}

func stripDiffPrefix(value, prefix string) string {
	if strings.HasPrefix(value, prefix) {
		return strings.TrimPrefix(value, prefix)
	}
	return value
}

func parseHunkHeader(line string) (hunk, error) {
	end := strings.Index(line[3:], " @@")
	if !strings.HasPrefix(line, "@@ ") || end < 0 {
		return hunk{}, fmt.Errorf("INVALID_HUNK_HEADER: %q", line)
	}
	body := line[3 : 3+end]
	parts := strings.Fields(body)
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "-") || !strings.HasPrefix(parts[1], "+") {
		return hunk{}, fmt.Errorf("INVALID_HUNK_HEADER: %q", line)
	}
	oldStart, oldCount, err := parseRange(strings.TrimPrefix(parts[0], "-"))
	if err != nil {
		return hunk{}, err
	}
	newStart, newCount, err := parseRange(strings.TrimPrefix(parts[1], "+"))
	if err != nil {
		return hunk{}, err
	}
	return hunk{OldStart: oldStart, OldCount: oldCount, NewStart: newStart, NewCount: newCount}, nil
}

func parseRange(s string) (int, int, error) {
	parts := strings.Split(s, ",")
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("INVALID_HUNK_RANGE: %q", s)
	}
	count := 1
	if len(parts) == 2 {
		count, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, fmt.Errorf("INVALID_HUNK_RANGE: %q", s)
		}
	}
	if len(parts) > 2 || start < 0 || count < 0 {
		return 0, 0, fmt.Errorf("INVALID_HUNK_RANGE: %q", s)
	}
	return start, count, nil
}

func verifyHunkCounts(h hunk) error {
	oldCount, newCount := 0, 0
	for _, line := range h.Lines {
		switch line[0] {
		case ' ':
			oldCount++
			newCount++
		case '-':
			oldCount++
		case '+':
			newCount++
		}
	}
	if oldCount != h.OldCount || newCount != h.NewCount {
		return fmt.Errorf("HUNK_COUNT_MISMATCH: old %d/%d new %d/%d", oldCount, h.OldCount, newCount, h.NewCount)
	}
	return nil
}

func applyFilePatch(original []byte, patch filePatch) ([]byte, error) {
	if patch.Create && len(original) != 0 {
		return nil, fmt.Errorf("CREATE_TARGET_EXISTS: %s", patch.Path)
	}
	if patch.Delete && len(original) == 0 {
		return nil, fmt.Errorf("DELETE_TARGET_MISSING: %s", patch.Path)
	}

	lineEnding := "\n"
	text := string(original)
	if strings.Contains(text, "\r\n") {
		lineEnding = "\r\n"
		text = strings.ReplaceAll(text, "\r\n", "\n")
	}
	hadFinalNewline := strings.HasSuffix(text, "\n")
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if len(original) == 0 {
		lines = nil
	}
	out := make([]string, 0, len(lines)+8)
	cursor := 0
	for _, h := range patch.Hunks {
		start := h.OldStart - 1
		if h.OldStart == 0 {
			start = 0
		}
		if start < cursor || start > len(lines) {
			return nil, fmt.Errorf("PATCH_CONTEXT_MISMATCH: %s invalid hunk start", patch.Path)
		}
		out = append(out, lines[cursor:start]...)
		pos := start
		for _, dl := range h.Lines {
			value := dl[1:]
			switch dl[0] {
			case ' ':
				if pos >= len(lines) || lines[pos] != value {
					return nil, fmt.Errorf("PATCH_CONTEXT_MISMATCH: %s", patch.Path)
				}
				out = append(out, value)
				pos++
			case '-':
				if pos >= len(lines) || lines[pos] != value {
					return nil, fmt.Errorf("PATCH_CONTEXT_MISMATCH: %s", patch.Path)
				}
				pos++
			case '+':
				out = append(out, value)
			}
		}
		cursor = pos
	}
	out = append(out, lines[cursor:]...)
	if patch.Delete {
		return nil, nil
	}
	result := strings.Join(out, lineEnding)
	if patch.Create || hadFinalNewline {
		result += lineEnding
	}
	return []byte(result), nil
}
