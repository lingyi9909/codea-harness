package changeset

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var hunkHeader153 = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

var sourceOrder153 = map[Source]int{
	SourceCommitted: 0,
	SourceStaged:    1,
	SourceUnstaged:  2,
	SourceUntracked: 3,
}

var changeSetScopes153 = []string{
	"src/main/java",
	"src/test/java",
	"src/main/resources",
}

// Compute independently recomputes the Harness Review Change Set from local Git state.
// It never fetches or substitutes a missing baseRef.
func Compute(repoRoot, baseRef string, includeWorkingTree bool) (Snapshot, error) {
	baseRef = strings.TrimSpace(baseRef)
	if baseRef == "" {
		return Snapshot{}, fmt.Errorf("CHANGE_SET_BASE_REF_REQUIRED")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mergeBase, err := git153(ctx, repoRoot, "merge-base", baseRef, "HEAD")
	if err != nil {
		return Snapshot{}, fmt.Errorf("CHANGE_SET_BASE_REF_NOT_FOUND: %s: %w", baseRef, err)
	}
	head, err := git153(ctx, repoRoot, "rev-parse", "HEAD")
	if err != nil {
		return Snapshot{}, fmt.Errorf("CHANGE_SET_HEAD_UNAVAILABLE: %w", err)
	}

	files := map[string]*File{}
	committedArgs := []string{"diff", "--unified=0", "--no-ext-diff", strings.TrimSpace(mergeBase) + "..HEAD", "--"}
	committedArgs = append(committedArgs, changeSetScopes153...)
	if err := collectDiff153(ctx, repoRoot, files, SourceCommitted, committedArgs...); err != nil {
		return Snapshot{}, fmt.Errorf("CHANGE_SET_COMMITTED_DIFF_FAILED: %w", err)
	}

	if includeWorkingTree {
		stagedArgs := []string{"diff", "--cached", "--unified=0", "--no-ext-diff", "--"}
		stagedArgs = append(stagedArgs, changeSetScopes153...)
		if err := collectDiff153(ctx, repoRoot, files, SourceStaged, stagedArgs...); err != nil {
			return Snapshot{}, fmt.Errorf("CHANGE_SET_STAGED_DIFF_FAILED: %w", err)
		}
		unstagedArgs := []string{"diff", "--unified=0", "--no-ext-diff", "--"}
		unstagedArgs = append(unstagedArgs, changeSetScopes153...)
		if err := collectDiff153(ctx, repoRoot, files, SourceUnstaged, unstagedArgs...); err != nil {
			return Snapshot{}, fmt.Errorf("CHANGE_SET_UNSTAGED_DIFF_FAILED: %w", err)
		}
		untracked, err := git153(ctx, repoRoot, "ls-files", "--others", "--exclude-standard")
		if err != nil {
			return Snapshot{}, fmt.Errorf("CHANGE_SET_UNTRACKED_LIST_FAILED: %w", err)
		}
		for _, raw := range strings.Split(strings.ReplaceAll(untracked, "\r\n", "\n"), "\n") {
			p := normalize153Path(raw)
			if p == "" || !inHarnessScope153(p) {
				continue
			}
			mergeFile153(files, File{Path: p, Status: "A", Sources: []Source{SourceUntracked}})
		}
	}

	out := make([]File, 0, len(files))
	for _, f := range files {
		sort.Slice(f.Sources, func(i, j int) bool { return sourceOrder153[f.Sources[i]] < sourceOrder153[f.Sources[j]] })
		f.Sources = dedupeSources153(f.Sources)
		sort.Slice(f.Hunks, func(i, j int) bool {
			a, b := f.Hunks[i], f.Hunks[j]
			if a.NewStart != b.NewStart { return a.NewStart < b.NewStart }
			if a.OldStart != b.OldStart { return a.OldStart < b.OldStart }
			if a.NewLines != b.NewLines { return a.NewLines < b.NewLines }
			return a.OldLines < b.OldLines
		})
		f.Hunks = dedupeHunks153(f.Hunks)
		out = append(out, *f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })

	snap := Snapshot{BaseRef: baseRef, Head: strings.TrimSpace(head), Files: out}
	canonical, err := json.Marshal(struct {
		BaseRef string `json:"baseRef"`
		Head    string `json:"head"`
		Files   []File `json:"files"`
	}{BaseRef: snap.BaseRef, Head: snap.Head, Files: snap.Files})
	if err != nil {
		return Snapshot{}, fmt.Errorf("CHANGE_SET_CANONICALIZE_FAILED: %w", err)
	}
	snap.SHA256 = fmt.Sprintf("%x", sha256.Sum256(canonical))
	return snap, nil
}

func git153(ctx context.Context, repoRoot string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func collectDiff153(ctx context.Context, repoRoot string, files map[string]*File, source Source, args ...string) error {
	out, err := git153(ctx, repoRoot, args...)
	if err != nil {
		return err
	}
	for _, f := range parseUnifiedDiff153([]byte(out), source) {
		if inHarnessScope153(f.Path) {
			mergeFile153(files, f)
		}
	}
	return nil
}

func parseUnifiedDiff153(data []byte, source Source) []File {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var current *File
	flush := func(out *[]File) {
		if current != nil && current.Path != "" {
			*out = append(*out, *current)
		}
		current = nil
	}
	var out []File
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "diff --git ") {
			flush(&out)
			current = &File{Status: "M", Sources: []Source{source}}
			continue
		}
		if current == nil {
			continue
		}
		switch {
		case strings.HasPrefix(line, "new file mode "):
			current.Status = "A"
		case strings.HasPrefix(line, "deleted file mode "):
			current.Status = "D"
		case strings.HasPrefix(line, "+++ "):
			p := diffHeaderPath153(strings.TrimPrefix(line, "+++ "))
			if p != "" {
				current.Path = p
			}
		case strings.HasPrefix(line, "--- ") && current.Path == "":
			p := diffHeaderPath153(strings.TrimPrefix(line, "--- "))
			if p != "" {
				current.Path = p
			}
		case strings.HasPrefix(line, "@@ "):
			if h, ok := parseHunk153(line); ok {
				current.Hunks = append(current.Hunks, h)
			}
		}
	}
	flush(&out)
	return out
}

func diffHeaderPath153(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "/dev/null" || raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "\"") {
		if decoded, err := strconv.Unquote(raw); err == nil {
			raw = decoded
		}
	}
	raw = strings.TrimPrefix(raw, "a/")
	raw = strings.TrimPrefix(raw, "b/")
	return normalize153Path(raw)
}

func parseHunk153(line string) (Hunk, bool) {
	m := hunkHeader153.FindStringSubmatch(line)
	if len(m) != 5 {
		return Hunk{}, false
	}
	oldStart, _ := strconv.Atoi(m[1])
	oldLines := 1
	if m[2] != "" { oldLines, _ = strconv.Atoi(m[2]) }
	newStart, _ := strconv.Atoi(m[3])
	newLines := 1
	if m[4] != "" { newLines, _ = strconv.Atoi(m[4]) }
	return Hunk{OldStart: oldStart, OldLines: oldLines, NewStart: newStart, NewLines: newLines}, true
}

func mergeFile153(files map[string]*File, incoming File) {
	incoming.Path = normalize153Path(incoming.Path)
	if incoming.Path == "" {
		return
	}
	if current, ok := files[incoming.Path]; ok {
		current.Sources = append(current.Sources, incoming.Sources...)
		current.Hunks = append(current.Hunks, incoming.Hunks...)
		current.Status = mergeStatus153(current.Status, incoming.Status)
		return
	}
	clone := incoming
	clone.Sources = append([]Source(nil), incoming.Sources...)
	clone.Hunks = append([]Hunk(nil), incoming.Hunks...)
	files[incoming.Path] = &clone
}

func mergeStatus153(current, incoming string) string {
	if current == "" { return incoming }
	if incoming == "" || incoming == current { return current }
	if incoming == "D" { return "D" }
	if current == "D" && incoming == "A" { return "M" }
	if current == "A" { return "A" }
	if incoming == "A" { return "A" }
	return "M"
}

func dedupeSources153(in []Source) []Source {
	seen := map[Source]bool{}
	out := make([]Source, 0, len(in))
	for _, s := range in {
		if !seen[s] { seen[s] = true; out = append(out, s) }
	}
	return out
}

func dedupeHunks153(in []Hunk) []Hunk {
	seen := map[Hunk]bool{}
	out := make([]Hunk, 0, len(in))
	for _, h := range in {
		if !seen[h] { seen[h] = true; out = append(out, h) }
	}
	return out
}

func normalize153Path(p string) string {
	p = strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
	if p == "" { return "" }
	p = path.Clean(p)
	if p == "." || p == ".." || strings.HasPrefix(p, "../") || path.IsAbs(p) {
		return ""
	}
	return p
}

func inHarnessScope153(p string) bool {
	p = normalize153Path(p)
	switch {
	case strings.HasPrefix(p, "src/main/java/") && strings.HasSuffix(p, ".java"):
		return true
	case strings.HasPrefix(p, "src/test/java/") && strings.HasSuffix(p, ".java"):
		return true
	case strings.HasPrefix(p, "src/main/resources/") && strings.HasSuffix(p, "Mapper.xml"):
		return true
	case strings.HasPrefix(p, "src/main/resources/") && strings.HasSuffix(p, ".yml"):
		return true
	default:
		return false
	}
}
