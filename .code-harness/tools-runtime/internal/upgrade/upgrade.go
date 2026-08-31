package upgrade

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"codea-harness-tools/internal/schema"
)

const (
	StatusUpgraded             = "UPGRADED"
	StatusAlreadyUpToDate      = "ALREADY_UP_TO_DATE"
	StatusManualActionRequired = "MANUAL_ACTION_REQUIRED"
	StatusUpgradeFailed        = "UPGRADE_FAILED"
)

type Result struct {
	Status            string   `json:"status"`
	FromVersion       string   `json:"fromVersion"`
	ToVersion         string   `json:"toVersion"`
	UpdatedFiles      []string `json:"updatedFiles,omitempty"`
	RemovedFiles      []string `json:"removedFiles,omitempty"`
	PreservedFiles    []string `json:"preservedFiles,omitempty"`
	RollbackPerformed bool     `json:"rollbackPerformed"`
	Errors            []string `json:"errors"`
	Migrations        []string `json:"migrations,omitempty"`
}

type RefProvider interface{ DetectBaseRef() (string, bool) }
type StaticRefs struct {
	OriginHead                    string
	RemoteBranches, LocalBranches []string
}

func (s StaticRefs) DetectBaseRef() (string, bool) {
	if s.OriginHead != "" {
		return s.OriginHead, true
	}
	has := func(xs []string, v string) bool {
		for _, x := range xs {
			if x == v {
				return true
			}
		}
		return false
	}
	for _, v := range []string{"origin/master", "origin/main", "origin/develop"} {
		if has(s.RemoteBranches, v) {
			return v, true
		}
	}
	for _, v := range []string{"master", "main", "develop"} {
		if has(s.LocalBranches, v) {
			return v, true
		}
	}
	return "", false
}

type Options struct {
	SourceDir, TargetDir string
	Refs                 RefProvider
	// RunningExecutable is test-only/integration plumbing. Empty means os.Executable().
	RunningExecutable string
}

var semverRE = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)$`)

func parseVer(s string) ([3]int, error) {
	m := semverRE.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return [3]int{}, fmt.Errorf("invalid semver %q", strings.TrimSpace(s))
	}
	var v [3]int
	for i := 0; i < 3; i++ {
		v[i], _ = strconv.Atoi(m[i+1])
	}
	return v, nil
}
func cmp(a, b [3]int) int {
	for i := 0; i < 3; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

var requiredSource = []string{
	"VERSION", "AGENTS.md", "bootstrap.md", "upgrade.md", "harness.template.yaml", "project.template.md",
	"agents", "skills", "contracts", "tools", "contracts/harness-config.schema.json",
	"bin/codea-dcep-tools.exe", "bin/ast-grep.exe",
}

var managedFiles = map[string]bool{
	"AGENTS.md": true, "bootstrap.md": true, "upgrade.md": true, "VERSION": true, ".gitignore": true,
	"harness.template.yaml": true, "project.template.md": true, "database.template.yaml": true,
}
var managedDirs = []string{"agents", "skills", "contracts", "tools", "bin", "tools-runtime"}

var projectStateFiles = map[string]bool{
	"harness.yaml":  true,
	"project.md":    true,
	"database.yaml": true,
}

func Run(o Options) Result {
	r := Result{PreservedFiles: []string{"harness.yaml", "project.md", "database.yaml", "chains/**", "runs/**"}, Errors: []string{}}
	oldB, err := os.ReadFile(filepath.Join(o.TargetDir, "VERSION"))
	if err != nil {
		return failManual(r, err)
	}
	newB, err := os.ReadFile(filepath.Join(o.SourceDir, "VERSION"))
	if err != nil {
		return failManual(r, err)
	}
	r.FromVersion = strings.TrimSpace(string(oldB))
	r.ToVersion = strings.TrimSpace(string(newB))
	oldV, err := parseVer(r.FromVersion)
	if err != nil {
		return failManual(r, err)
	}
	newV, err := parseVer(r.ToVersion)
	if err != nil {
		return failManual(r, err)
	}
	if cmp(newV, oldV) < 0 {
		return failManual(r, fmt.Errorf("downgrade is not allowed"))
	}
	if cmp(newV, oldV) == 0 {
		r.Status = StatusAlreadyUpToDate
		return r
	}
	for _, rel := range requiredSource {
		if _, err := os.Stat(filepath.Join(o.SourceDir, rel)); err != nil {
			return failManual(r, fmt.Errorf("incomplete upgrade package: %s", rel))
		}
	}

	cfgPath := filepath.Join(o.TargetDir, "harness.yaml")
	cfg, err := os.ReadFile(cfgPath)
	if err != nil {
		return failManual(r, err)
	}
	migrated := append([]byte(nil), cfg...)
	if !hasTopLevelReview(cfg) {
		if o.Refs == nil {
			return failManual(r, fmt.Errorf("cannot detect Review baseline; configure review.baseRef"))
		}
		base, ok := o.Refs.DetectBaseRef()
		if !ok {
			return failManual(r, fmt.Errorf("无法确定 Review 基线，请配置 review.baseRef"))
		}
		migrated = append(migrated, []byte(fmt.Sprintf("\nreview:\n  baseRef: %s\n  includeWorkingTree: true\n", base))...)
		r.Migrations = append(r.Migrations, "add-review-config-v1")
	}
	if cmp(newV, [3]int{1, 4, 0}) >= 0 {
		var changed bool
		migrated, changed, err = migrateConfigV1ToV2ResourceScopes(migrated)
		if err != nil {
			return failManual(r, fmt.Errorf("migrate 1.4 harness config: %w", err))
		}
		if changed {
			r.Migrations = append(r.Migrations, "upgrade-config-v1-to-v2-resource-scopes")
		}
	}

	parent := filepath.Dir(filepath.Clean(o.TargetDir))
	id := strconv.FormatInt(time.Now().UnixNano(), 10)
	backup := filepath.Join(parent, ".code-harness-backup-"+id)
	stage := filepath.Join(parent, ".code-harness-stage-"+id)

	oldManaged, err := listManagedFiles(o.TargetDir)
	if err != nil {
		return failManual(r, err)
	}
	newManaged, err := listManagedFiles(o.SourceDir)
	if err != nil {
		return failManual(r, err)
	}
	r.UpdatedFiles = append([]string(nil), newManaged...)
	r.RemovedFiles = difference(oldManaged, newManaged)

	if err := copyTree(o.TargetDir, backup, nil); err != nil {
		cleanup(stage, backup)
		return failManual(r, fmt.Errorf("backup: %w", err))
	}
	if err := copyTree(o.TargetDir, stage, nil); err != nil {
		cleanup(stage, backup)
		return failManual(r, fmt.Errorf("stage target: %w", err))
	}

	// True replace semantics happen in stage first: remove every framework-owned path,
	// then copy only the new package's framework-owned paths.
	if err := removeManaged(stage); err != nil {
		cleanup(stage, backup)
		return failManual(r, fmt.Errorf("stage remove managed: %w", err))
	}
	if err := copyManaged(o.SourceDir, stage); err != nil {
		cleanup(stage, backup)
		return failManual(r, fmt.Errorf("stage copy managed: %w", err))
	}
	if err := os.WriteFile(filepath.Join(stage, "harness.yaml"), migrated, 0o644); err != nil {
		cleanup(stage, backup)
		return failManual(r, err)
	}

	schemaB, err := os.ReadFile(filepath.Join(stage, "contracts", "harness-config.schema.json"))
	if err != nil {
		cleanup(stage, backup)
		return failManual(r, err)
	}
	if err := schema.ValidateYAML(schemaB, migrated); err != nil {
		cleanup(stage, backup)
		r.Status = StatusUpgradeFailed
		r.RollbackPerformed = true
		r.Errors = []string{fmt.Sprintf("harness.yaml incompatible with new schema: %v", err)}
		return r
	}

	// Apply all framework changes from the fully validated stage. The running
	// Windows executable is replaced last using rename-to-temp + staged rename,
	// never by writing over the live image.
	if err := applyStaged(stage, o.TargetDir, o.RunningExecutable); err != nil {
		rbErr := restoreFromBackup(backup, o.TargetDir, o.RunningExecutable)
		r.Status = StatusUpgradeFailed
		r.RollbackPerformed = rbErr == nil
		r.Errors = []string{err.Error()}
		if rbErr != nil {
			r.Errors = append(r.Errors, "rollback: "+rbErr.Error())
		}
		cleanup(stage, backup)
		return r
	}

	// When the 1.4 upgrade package runtime executes the transaction itself on Windows,
	// move that live executable to a same-volume sibling before removing the consumed
	// source tree. Installed-runtime upgrades do not need this step.
	parkedRuntime, err := parkRunningExecutableOutsideSource(o.SourceDir, o.RunningExecutable)
	if err != nil {
		rbErr := restoreFromBackup(backup, o.TargetDir, o.RunningExecutable)
		r.Status = StatusUpgradeFailed
		r.RollbackPerformed = rbErr == nil
		r.Errors = []string{err.Error()}
		if rbErr != nil {
			r.Errors = append(r.Errors, "rollback: "+rbErr.Error())
		}
		cleanup(stage, backup)
		return r
	}

	// Success cleanup semantics: stage + backup + consumed source package are removed.
	// A renamed live executable may remain in a same-volume sibling temp path until
	// process exit; it is outside the Harness tree and is best-effort cleaned.
	if err := cleanupExact(stage, backup, o.SourceDir); err != nil {
		r.Status = StatusUpgradeFailed
		r.Errors = []string{"upgrade applied but cleanup failed: " + err.Error()}
		return r
	}
	if parkedRuntime != "" {
		_ = os.Remove(parkedRuntime)
	}
	r.Status = StatusUpgraded
	return r
}

func failManual(r Result, e error) Result {
	r.Status = StatusManualActionRequired
	r.Errors = []string{e.Error()}
	return r
}

func hasTopLevelReview(b []byte) bool {
	for _, l := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		if strings.HasPrefix(l, "review:") {
			return true
		}
	}
	return false
}

func isProjectState(rel string) bool {
	rel = filepath.ToSlash(strings.TrimPrefix(rel, "./"))
	return projectStateFiles[rel] || rel == "chains" || strings.HasPrefix(rel, "chains/") || rel == "runs" || strings.HasPrefix(rel, "runs/")
}

func isManaged(rel string) bool {
	rel = filepath.ToSlash(strings.TrimPrefix(rel, "./"))
	if isProjectState(rel) {
		return false
	}
	if managedFiles[rel] {
		return true
	}
	for _, d := range managedDirs {
		if rel == d || strings.HasPrefix(rel, d+"/") {
			return true
		}
	}
	return false
}

func listManagedFiles(root string) ([]string, error) {
	var xs []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if isManaged(rel) {
			xs = append(xs, rel)
		}
		return nil
	})
	sort.Strings(xs)
	return xs, err
}

func difference(old, next []string) []string {
	n := map[string]bool{}
	for _, x := range next {
		n[x] = true
	}
	var out []string
	for _, x := range old {
		if !n[x] {
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}

func removeManaged(root string) error {
	for rel := range managedFiles {
		if err := os.RemoveAll(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			return err
		}
	}
	for _, rel := range managedDirs {
		if err := os.RemoveAll(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			return err
		}
	}
	return nil
}

func copyManaged(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if !isManaged(rel) {
			if d.IsDir() {
				first := strings.Split(rel, "/")[0]
				if !managedFiles[first] {
					ok := false
					for _, md := range managedDirs {
						if first == md {
							ok = true
							break
						}
					}
					if !ok {
						return filepath.SkipDir
					}
				}
			}
			return nil
		}
		to := filepath.Join(dst, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(to, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return copyFile(p, to, info.Mode())
	})
}

func applyStaged(stage, target, running string) error {
	newSet := map[string]bool{}
	newFiles, err := listManagedFiles(stage)
	if err != nil {
		return err
	}
	for _, x := range newFiles {
		newSet[x] = true
	}
	oldFiles, err := listManagedFiles(target)
	if err != nil {
		return err
	}
	for _, rel := range oldFiles {
		if newSet[rel] {
			continue
		}
		if err := os.Remove(filepath.Join(target, filepath.FromSlash(rel))); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove stale %s: %w", rel, err)
		}
	}
	removeEmptyManagedDirs(target)

	selfRel := "bin/codea-dcep-tools.exe"
	for _, rel := range newFiles {
		if rel == selfRel {
			continue
		}
		src := filepath.Join(stage, filepath.FromSlash(rel))
		dst := filepath.Join(target, filepath.FromSlash(rel))
		info, err := os.Stat(src)
		if err != nil {
			return err
		}
		if err := stagedReplaceFile(src, dst, info.Mode(), false, running); err != nil {
			return fmt.Errorf("replace %s: %w", rel, err)
		}
	}
	if newSet[selfRel] {
		src := filepath.Join(stage, filepath.FromSlash(selfRel))
		dst := filepath.Join(target, filepath.FromSlash(selfRel))
		info, err := os.Stat(src)
		if err != nil {
			return err
		}
		if err := stagedReplaceFile(src, dst, info.Mode(), true, running); err != nil {
			return fmt.Errorf("replace %s: %w", selfRel, err)
		}
	}
	cfg, err := os.ReadFile(filepath.Join(stage, "harness.yaml"))
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(target, "harness.yaml"), cfg, 0o644)
}

func stagedReplaceFile(src, dst string, mode fs.FileMode, self bool, running string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp := dst + ".codea-new"
	_ = os.Remove(tmp)
	if err := copyFile(src, tmp, mode); err != nil {
		return err
	}
	if !self {
		if err := os.Rename(tmp, dst); err == nil {
			return nil
		}
		if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
			_ = os.Remove(tmp)
			return err
		}
		if err := os.Rename(tmp, dst); err != nil {
			_ = os.Remove(tmp)
			return err
		}
		return nil
	}
	return replaceRunningExecutable(tmp, dst, running)
}

func runningExecutableParkingPath(dst string) string {
	harnessDir := filepath.Dir(filepath.Dir(dst))
	parent := filepath.Dir(harnessDir)
	return filepath.Join(parent, fmt.Sprintf(".codea-harness-tools-old-%d.exe", time.Now().UnixNano()))
}

func replaceRunningExecutable(staged, dst, running string) error {
	if running == "" {
		if exe, err := os.Executable(); err == nil {
			running = exe
		}
	}
	same := samePath(running, dst)
	if !same {
		if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
			return err
		}
		return os.Rename(staged, dst)
	}
	oldTemp := runningExecutableParkingPath(dst)
	if err := os.Rename(dst, oldTemp); err != nil {
		return fmt.Errorf("move running executable to same-volume temp: %w", err)
	}
	if err := os.Rename(staged, dst); err != nil {
		_ = os.Rename(oldTemp, dst)
		return fmt.Errorf("install staged executable: %w", err)
	}
	_ = os.Remove(oldTemp)
	return nil
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	aa, _ := filepath.Abs(a)
	bb, _ := filepath.Abs(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(aa), filepath.Clean(bb))
	}
	return filepath.Clean(aa) == filepath.Clean(bb)
}

func restoreFromBackup(backup, target, running string) error {
	if _, err := os.Stat(backup); err != nil {
		return err
	}
	return applyStaged(backup, target, running)
}

func removeEmptyManagedDirs(root string) {
	for _, rel := range managedDirs {
		removeEmpty(filepath.Join(root, rel))
	}
}
func removeEmpty(p string) {
	entries, err := os.ReadDir(p)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			removeEmpty(filepath.Join(p, e.Name()))
		}
	}
	entries, _ = os.ReadDir(p)
	if len(entries) == 0 {
		_ = os.Remove(p)
	}
}

func copyTree(src, dst string, allow func(string, fs.DirEntry) bool) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		rel = filepath.ToSlash(rel)
		if allow != nil && !allow(rel, d) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		to := filepath.Join(dst, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(to, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return copyFile(p, to, info.Mode())
	})
}

func copyFile(src, dst string, mode fs.FileMode) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, b, mode.Perm())
}

func cleanup(paths ...string) {
	for _, p := range paths {
		_ = os.RemoveAll(p)
	}
}
func cleanupExact(paths ...string) error {
	var errs []string
	for _, p := range paths {
		if p == "" {
			continue
		}
		if err := os.RemoveAll(p); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", p, err))
		}
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			errs = append(errs, p+": still exists after cleanup")
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}
