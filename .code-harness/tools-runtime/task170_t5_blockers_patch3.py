from pathlib import Path

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

# A resolver may deliberately report "no Spring context". Treat a zero-value
# placeholder as absence so the rule becomes BLOCKED via missing relation,
# rather than turning absence into a malformed-relation product error.
p = read('internal/reviewcontext/build_170.go')
p = replace_once(p,
'''\t\tfor _, relation := range springRelations {\n\t\t\tif err := addRelation(relation); err != nil {\n\t\t\t\treturn Context170{}, err\n\t\t\t}\n\t\t}''',
'''\t\tfor _, relation := range springRelations {\n\t\t\tif strings.TrimSpace(relation.ID) == "" && strings.TrimSpace(relation.Kind) == "" {\n\t\t\t\tcontinue\n\t\t\t}\n\t\t\tif err := addRelation(relation); err != nil {\n\t\t\t\treturn Context170{}, err\n\t\t\t}\n\t\t}''',
'empty Spring relation means missing context')
write('internal/reviewcontext/build_170.go', p)

# Reuse the same bounded `git cat-file --batch` mechanism as 1.6.4, but bind
# it to exact Runtime-requested Review-scope paths instead of the old
# entrypoint-only main-Java validator. This adds no snapshot and performs no
# directory discovery or glob expansion.
write('internal/analysis/base_source_170.go', r'''package analysis

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"codea-harness-tools/internal/projectpath"
)

// ReadBaseSources170 reads exactly the requested review-scope paths from one
// already-resolved merge-base commit using one local git cat-file --batch
// process. No ref resolution, globbing, directory walk, fetch, or persistent
// snapshot is introduced by T5.
func ReadBaseSources170(ctx context.Context, repoRoot, mergeBase string, paths []string) (map[string][]byte, error) {
	mergeBase = strings.TrimSpace(mergeBase)
	if mergeBase == "" {
		return nil, fmt.Errorf("REVIEW_CONTEXT_BASE_SOURCE_INVALID: mergeBase is required")
	}
	clean, err := exactBaseSourcePaths170(paths)
	if err != nil {
		return nil, err
	}
	if len(clean) == 0 {
		return map[string][]byte{}, nil
	}

	var input strings.Builder
	for _, p := range clean {
		input.WriteString(mergeBase)
		input.WriteByte(':')
		input.WriteString(p)
		input.WriteByte('\n')
	}
	cmd := exec.CommandContext(ctx, "git", "cat-file", "--batch")
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(input.String())
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("REVIEW_CONTEXT_BASE_SOURCE_BATCH_FAILED: %w", err)
	}

	reader := bufio.NewReader(bytes.NewReader(out))
	result := make(map[string][]byte, len(clean))
	for _, p := range clean {
		header, readErr := reader.ReadString('\n')
		if readErr != nil {
			return nil, fmt.Errorf("REVIEW_CONTEXT_BASE_SOURCE_BATCH_FAILED: %s header: %w", p, readErr)
		}
		header = strings.TrimSuffix(strings.TrimSuffix(header, "\n"), "\r")
		if strings.HasSuffix(header, " missing") {
			continue
		}
		fields := strings.Fields(header)
		if len(fields) != 3 || fields[1] != "blob" {
			return nil, fmt.Errorf("REVIEW_CONTEXT_BASE_SOURCE_INVALID: unexpected cat-file header for %s: %q", p, header)
		}
		size, sizeErr := strconv.Atoi(fields[2])
		if sizeErr != nil || size < 0 {
			return nil, fmt.Errorf("REVIEW_CONTEXT_BASE_SOURCE_BATCH_FAILED: invalid size for %s: %q", p, fields[2])
		}
		data := make([]byte, size)
		if _, readErr := io.ReadFull(reader, data); readErr != nil {
			return nil, fmt.Errorf("REVIEW_CONTEXT_BASE_SOURCE_BATCH_FAILED: %s body: %w", p, readErr)
		}
		terminator, readErr := reader.ReadByte()
		if readErr != nil || terminator != '\n' {
			return nil, fmt.Errorf("REVIEW_CONTEXT_BASE_SOURCE_BATCH_FAILED: %s terminator", p)
		}
		result[p] = data
	}
	if extra, _ := io.ReadAll(reader); len(bytes.TrimSpace(extra)) != 0 {
		return nil, fmt.Errorf("REVIEW_CONTEXT_BASE_SOURCE_INVALID: unexpected cat-file output")
	}
	return result, nil
}

func exactBaseSourcePaths170(paths []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(paths))
	for _, raw := range paths {
		if strings.ContainsAny(raw, "\x00\r\n") {
			return nil, fmt.Errorf("REVIEW_CONTEXT_BASE_SOURCE_SCOPE_WIDENED: path contains NUL/CR/LF")
		}
		p, ok := projectpath.Normalize(raw)
		if !ok || p != strings.ReplaceAll(raw, "\\", "/") || !projectpath.IsReviewPath(p) {
			return nil, fmt.Errorf("REVIEW_CONTEXT_BASE_SOURCE_SCOPE_WIDENED: invalid review path %q", raw)
		}
		key := strings.ToLower(p)
		if seen[key] {
			return nil, fmt.Errorf("REVIEW_CONTEXT_BASE_SOURCE_SCOPE_WIDENED: duplicate path %q", raw)
		}
		seen[key] = true
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}
''')

print('TASK170_T5_BLOCKER_PATCH3_APPLIED')
