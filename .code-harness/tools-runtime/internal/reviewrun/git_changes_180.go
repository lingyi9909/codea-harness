package reviewrun

import (
    "bytes"
    "context"
    "errors"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "sort"
    "strconv"
    "strings"

    "codea-harness-tools/internal/changeset"
    "gopkg.in/yaml.v3"
)

// reviewGitConfig180 uses the same review.baseRef and includeWorkingTree
// configuration as the existing canonical ChangeSet. Without a configured
// baseRef, CHANGES retains its legacy working-tree-only behavior (HEAD).
// A configured but unavailable baseRef must NOT silently fall back to HEAD.
type reviewGitConfig180 struct {
    BaseRef string
    IncludeWorkingTree bool
}

func loadReviewGitConfig180(root string) (reviewGitConfig180, error) {
    config := reviewGitConfig180{BaseRef: "HEAD", IncludeWorkingTree: true}
    data, err := os.ReadFile(filepath.Join(root, ".code-harness", "harness.yaml"))
    if errors.Is(err, os.ErrNotExist) {
        return config, nil
    }
    if err != nil {
        return config, fmt.Errorf("REVIEW_CONFIG_READ_FAILED: %w", err)
    }
    var document struct {
        Review struct {
            BaseRef string `yaml:"baseRef"`
            IncludeWorkingTree *bool `yaml:"includeWorkingTree"`
        } `yaml:"review"`
    }
    if err := yaml.Unmarshal(data, &document); err != nil {
        return config, fmt.Errorf("REVIEW_CONFIG_INVALID: %w", err)
    }
    if value := strings.TrimSpace(document.Review.BaseRef); value != "" {
        config.BaseRef = value
    }
    if document.Review.IncludeWorkingTree != nil {
        config.IncludeWorkingTree = *document.Review.IncludeWorkingTree
    }
    return config, nil
}

// reviewChangesSnapshot180 is the source of truth for BOTH prepare impact
// and finish's introducedByChange attribution. Canonical ChangeSet includes
// committed changes since merge-base, plus working tree changes when enabled.
func reviewChangesSnapshot180(root string) (changeset.Snapshot, error) {
    config, err := loadReviewGitConfig180(root)
    if err != nil {
        return changeset.Snapshot{}, err
    }
    return changeset.Compute(root, config.BaseRef, config.IncludeWorkingTree)
}

func changedSourceFiles180(_ context.Context, root string) ([]string, error) {
    snapshot, err := reviewChangesSnapshot180(root)
    if err != nil {
        return nil, err
    }
    paths := make([]string, 0, len(snapshot.Files))
    for _, file := range snapshot.Files {
        ext := filepath.Ext(file.Path)
        if strings.EqualFold(ext, ".java") || strings.EqualFold(ext, ".xml") {
            paths = append(paths, filepath.ToSlash(file.Path))
        }
    }
    sort.Strings(paths)
    return paths, nil
}

// changedLineRangesForPath180 recomputes the exact *current line coordinates*
// against the same merge-base as prepare. Computing one combined diff (rather
// than merging individual staged/unstaged hunk positions) keeps attribution
// correct even when line offsets shift across multiple edit stages.
func changedLineRangesForPath180(ctx context.Context, root, rel string) ([]ReadRef, error) {
    clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(rel)))
    if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || filepath.IsAbs(clean) {
        return nil, fmt.Errorf("invalid change path %q", rel)
    }
    snapshot, err := reviewChangesSnapshot180(root)
    if err != nil {
        return nil, err
    }
    if snapshot.IncludeWorkingTree {
        for _, file := range snapshot.Files {
            if file.Path != clean {
                continue
            }
            for _, src := range file.Sources {
                if src == changeset.SourceUntracked {
                    return fullFileReadRange180(root, clean)
                }
            }
        }
    }
    args := []string{"diff", "--unified=0", "--no-ext-diff", "--no-color", snapshot.MergeBase}
    if !snapshot.IncludeWorkingTree {
        args = append(args, snapshot.HeadCommit)
    }
    args = append(args, "--", clean)
    cmd := exec.CommandContext(ctx, "git", args...)
    cmd.Dir = root
    out, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("REVIEW_CHANGE_ATTRIBUTION_DIFF_FAILED: %w", err)
    }
    refs := []ReadRef{}
    for _, line := range strings.Split(string(out), "\n") {
        m := diffHunk180.FindStringSubmatch(line)
        if len(m) == 0 {
            continue
        }
        start, _ := strconv.Atoi(m[1])
        count := 1
        if m[2] != "" {
            count, _ = strconv.Atoi(m[2])
        }
        if count > 0 {
            refs = append(refs, ReadRef{Path: clean, StartLine: start, EndLine: start + count - 1})
        }
    }
    return refs, nil
}

func fullFileReadRange180(root, rel string) ([]ReadRef, error) {
    data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
    if err != nil {
        return nil, err
    }
    lines := bytes.Count(data, []byte("\n"))
    if len(data) == 0 || data[len(data)-1] != '\n' {
        lines++
    }
    if lines < 1 {
        lines = 1
    }
    return []ReadRef{{Path: rel, StartLine: 1, EndLine: lines}}, nil
}

// verifyPreparedChangesSnapshot180 rejects stale CHANGES selections even when
// Java/XML bytes are unchanged (commit, rebase, or review.baseRef update).
// CURRENT_IMPLEMENTATION is independent of the Git change baseline.
func verifyPreparedChangesSnapshot180(root string, prepared preparedOptions180) error {
    if prepared.Intent.Mode != "CHANGES" { return nil }
    if prepared.ChangeSnapshotSHA256 == "" { return fmt.Errorf("CHANGESET_IDENTITY_MISSING") }
    snapshot, err := reviewChangesSnapshot180(root)
    if err != nil { return fmt.Errorf("CHANGESET_SNAPSHOT_FAILED: %w", err) }
    if snapshot.SnapshotSHA256 != prepared.ChangeSnapshotSHA256 { return fmt.Errorf("CHANGESET_CHANGED_SINCE_PREPARE") }
    return nil
}
