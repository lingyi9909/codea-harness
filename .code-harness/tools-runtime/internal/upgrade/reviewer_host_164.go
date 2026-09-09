package upgrade

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const reviewerHostUpgradeRoot = "host"

var reviewerHostFiles164 = []string{
	".opencode/agents/reviewer.md",
	".opencode/commands/harness-review-reviewer.md",
}

// reviewerHostInstallHook is intentionally package-private test plumbing used
// to prove rollback after a partial Host commit. Production leaves it nil.
var reviewerHostInstallHook func(index int, rel string) error

type reviewerHostSnapshot struct {
	rel    string
	exists bool
	data   []byte
	mode   fs.FileMode
}

type reviewerHostTransaction struct {
	projectRoot string
	sourceRoot  string
	snapshots   []reviewerHostSnapshot
}

func crossesReviewerHostBoundary(oldV, newV [3]int) bool {
	boundary := [3]int{1, 6, 4}
	return cmp(oldV, boundary) < 0 && cmp(newV, boundary) >= 0
}

func prepareReviewerHostTransaction(o Options, oldV, newV [3]int) (*reviewerHostTransaction, error) {
	if !crossesReviewerHostBoundary(oldV, newV) {
		return nil, nil
	}
	projectRoot := filepath.Dir(filepath.Clean(o.TargetDir))
	sourceRoot := filepath.Join(o.SourceDir, reviewerHostUpgradeRoot)
	if _, err := os.Stat(sourceRoot); os.IsNotExist(err) {
		// Historical test/source trees predate Host resources. The official
		// 1.6.4 package gate requires the staged Host tree; when it is present
		// Runtime includes it in the same transaction.
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("inspect Reviewer Host staged source: %w", err)
	}
	txn := &reviewerHostTransaction{projectRoot: projectRoot, sourceRoot: sourceRoot}
	for _, rel := range reviewerHostFiles164 {
		src := filepath.Join(sourceRoot, filepath.FromSlash(rel))
		sourceInfo, err := os.Stat(src)
		if err != nil || !sourceInfo.Mode().IsRegular() {
			return nil, fmt.Errorf("incomplete upgrade package: %s/%s", reviewerHostUpgradeRoot, rel)
		}
		sourceBytes, err := os.ReadFile(src)
		if err != nil {
			return nil, err
		}
		dst := filepath.Join(projectRoot, filepath.FromSlash(rel))
		info, err := os.Stat(dst)
		if os.IsNotExist(err) {
			txn.snapshots = append(txn.snapshots, reviewerHostSnapshot{rel: rel})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("Reviewer Host target cannot be inspected %s: %w", rel, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("Reviewer Host target conflict %s: existing path is not a regular file", rel)
		}
		current, err := os.ReadFile(dst)
		if err != nil {
			return nil, err
		}
		if string(current) != string(sourceBytes) {
			return nil, fmt.Errorf("Reviewer Host target conflict %s: existing file is not owned by this release; MANUAL_ACTION_REQUIRED", rel)
		}
		txn.snapshots = append(txn.snapshots, reviewerHostSnapshot{rel: rel, exists: true, data: current, mode: info.Mode()})
	}
	return txn, nil
}

func (t *reviewerHostTransaction) apply() ([]string, error) {
	if t == nil {
		return nil, nil
	}
	var updated []string
	for index, snap := range t.snapshots {
		if reviewerHostInstallHook != nil {
			if err := reviewerHostInstallHook(index, snap.rel); err != nil {
				return updated, err
			}
		}
		src := filepath.Join(t.sourceRoot, filepath.FromSlash(snap.rel))
		dst := filepath.Join(t.projectRoot, filepath.FromSlash(snap.rel))
		same := false
		if snap.exists {
			var err error
			same, err = filesEqual(src, dst)
			if err != nil {
				return updated, err
			}
		}
		if same {
			continue
		}
		info, err := os.Stat(src)
		if err != nil {
			return updated, err
		}
		if err := stagedReplaceFile(src, dst, info.Mode(), false, ""); err != nil {
			return updated, fmt.Errorf("install Reviewer Host %s: %w", snap.rel, err)
		}
		updated = append(updated, snap.rel)
	}
	return updated, nil
}

func (t *reviewerHostTransaction) rollback() error {
	if t == nil {
		return nil
	}
	var first error
	for i := len(t.snapshots) - 1; i >= 0; i-- {
		snap := t.snapshots[i]
		dst := filepath.Join(t.projectRoot, filepath.FromSlash(snap.rel))
		if snap.exists {
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err == nil {
				err = os.WriteFile(dst, snap.data, snap.mode.Perm())
				if err != nil && first == nil {
					first = err
				}
			} else if first == nil {
				first = err
			}
			continue
		}
		if err := os.Remove(dst); err != nil && !os.IsNotExist(err) && first == nil {
			first = err
		}
		removeEmptyHostParents(t.projectRoot, snap.rel)
	}
	return first
}

func removeEmptyHostParents(projectRoot, rel string) {
	stop := filepath.Join(projectRoot, ".opencode")
	dir := filepath.Dir(filepath.Join(projectRoot, filepath.FromSlash(rel)))
	for filepath.Clean(dir) != filepath.Clean(projectRoot) {
		if err := os.Remove(dir); err != nil {
			return
		}
		if filepath.Clean(dir) == filepath.Clean(stop) {
			return
		}
		dir = filepath.Dir(dir)
	}
}

func failAndRollbackWithReviewerHost(r Result, cause error, host *reviewerHostTransaction, stage, backup, target, running string, delta ...[]string) Result {
	r = failAndRollback(r, cause, stage, backup, target, running, delta...)
	if err := host.rollback(); err != nil {
		r.RollbackPerformed = false
		r.Errors = append(r.Errors, "Reviewer Host rollback: "+err.Error())
	}
	return r
}
