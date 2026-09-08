package changeset

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// VerifyFreshness proves that the live Git source identity still matches a
// previously sealed canonical Snapshot. It intentionally does not rebuild
// Files/Hunks or mint a replacement Snapshot; Compute remains the sole
// canonical Snapshot producer.
func VerifyFreshness(repoRoot string, snapshot Snapshot) error {
	if err := validateCanonicalSnapshot162(snapshot); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resolvedBase, head, err := resolveFreshnessCommits164(ctx, repoRoot, snapshot.RequestedBaseRef)
	if err != nil {
		return err
	}
	if resolvedBase != snapshot.ResolvedBaseCommit {
		return fmt.Errorf("CHANGE_SET_SNAPSHOT_STALE: resolved base moved")
	}
	if head != snapshot.HeadCommit {
		return fmt.Errorf("CHANGE_SET_SNAPSHOT_STALE: HEAD moved")
	}

	mergeBase, err := runGit153(ctx, repoRoot, "merge-base", resolvedBase, head)
	if err != nil {
		return fmt.Errorf("CHANGE_SET_MERGE_BASE_UNAVAILABLE: %w", err)
	}
	if strings.TrimSpace(mergeBase) != snapshot.MergeBase {
		return fmt.Errorf("CHANGE_SET_SNAPSHOT_STALE: merge base changed")
	}

	currentBranch, err := currentBranch153(ctx, repoRoot)
	if err != nil {
		return err
	}
	if currentBranch != snapshot.CurrentBranch {
		return fmt.Errorf("CHANGE_SET_SNAPSHOT_STALE: current branch changed")
	}

	committedPatch, err := freshnessDiff164(ctx, repoRoot, SourceCommitted,
		"diff", "--unified=0", "--no-ext-diff", snapshot.MergeBase+".."+snapshot.HeadCommit, "--")
	if err != nil {
		return fmt.Errorf("CHANGE_SET_COMMITTED_DIFF_FAILED: %w", err)
	}

	stagedPatch := ""
	unstagedPatch := ""
	untrackedState := []untrackedState162{}
	if snapshot.IncludeWorkingTree {
		stagedPatch, err = freshnessDiff164(ctx, repoRoot, SourceStaged,
			"diff", "--cached", "--unified=0", "--no-ext-diff", "--")
		if err != nil {
			return fmt.Errorf("CHANGE_SET_STAGED_DIFF_FAILED: %w", err)
		}
		unstagedPatch, err = freshnessDiff164(ctx, repoRoot, SourceUnstaged,
			"diff", "--unified=0", "--no-ext-diff", "--")
		if err != nil {
			return fmt.Errorf("CHANGE_SET_UNSTAGED_DIFF_FAILED: %w", err)
		}
		untrackedState, err = freshnessUntracked164(ctx, repoRoot)
		if err != nil {
			return err
		}
	}

	gitStateSHA, err := gitStateSHA256162(committedPatch, stagedPatch, unstagedPatch, untrackedState)
	if err != nil {
		return err
	}
	if gitStateSHA != snapshot.GitStateSHA256 {
		return fmt.Errorf("CHANGE_SET_SNAPSHOT_STALE: Git state changed")
	}

	resolvedBaseAfter, headAfter, err := resolveFreshnessCommits164(ctx, repoRoot, snapshot.RequestedBaseRef)
	if err != nil {
		return err
	}
	if resolvedBaseAfter != resolvedBase || headAfter != head {
		return fmt.Errorf("CHANGE_SET_SNAPSHOT_STALE: Git identity changed during freshness verification")
	}
	branchAfter, err := currentBranch153(ctx, repoRoot)
	if err != nil {
		return err
	}
	if branchAfter != currentBranch {
		return fmt.Errorf("CHANGE_SET_SNAPSHOT_STALE: current branch changed during freshness verification")
	}
	return nil
}

func resolveFreshnessCommits164(ctx context.Context, repoRoot, requestedBaseRef string) (string, string, error) {
	out, err := runGit153(ctx, repoRoot, "rev-parse", requestedBaseRef+"^{commit}", "HEAD^{commit}")
	if err != nil {
		return "", "", fmt.Errorf("CHANGE_SET_FRESHNESS_IDENTITY_FAILED: %w", err)
	}
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return "", "", fmt.Errorf("CHANGE_SET_FRESHNESS_IDENTITY_FAILED: expected base and HEAD commit ids")
	}
	return fields[0], fields[1], nil
}

func freshnessDiff164(ctx context.Context, repoRoot string, source Source, args ...string) (string, error) {
	out, err := runGit153(ctx, repoRoot, args...)
	if err != nil {
		return "", err
	}
	return reviewScopedDiff153(out, source), nil
}

func freshnessUntracked164(ctx context.Context, repoRoot string) ([]untrackedState162, error) {
	untracked, err := runGit153(ctx, repoRoot, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, fmt.Errorf("CHANGE_SET_UNTRACKED_LIST_FAILED: %w", err)
	}
	state := []untrackedState162{}
	for _, raw := range strings.Split(strings.ReplaceAll(untracked, "\r\n", "\n"), "\n") {
		p := normalize153Path(raw)
		if p == "" || !inHarnessScope153(p) {
			continue
		}
		content, readErr := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(p)))
		if readErr != nil {
			return nil, fmt.Errorf("CHANGE_SET_UNTRACKED_READ_FAILED: %s: %w", p, readErr)
		}
		state = append(state, untrackedState162{Path: p, SHA256: fmt.Sprintf("%x", sha256.Sum256(content))})
	}
	sort.Slice(state, func(i, j int) bool { return state[i].Path < state[j].Path })
	return state, nil
}
