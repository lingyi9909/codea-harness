package upgrade

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const manifestPath = "RELEASE-MANIFEST.json"

type releaseInventory struct {
	Version       string            `json:"version"`
	BuildCommit   string            `json:"buildCommit"`
	RuntimeSHA256 string            `json:"runtimeSha256"`
	ManagedFiles  map[string]string `json:"managedFiles"`
}

// Installed inventories establish ownership, not the current bytes of files users
// may have edited. Source inventories additionally attest every shipped byte.
func readInventory(root string, source bool) (*releaseInventory, error) {
	b, err := os.ReadFile(filepath.Join(root, manifestPath))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m releaseInventory
	if err = json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("invalid release manifest: %w", err)
	}
	if source && m.ManagedFiles == nil {
		return nil, fmt.Errorf("source manifest missing managedFiles")
	}
	for rel, hash := range m.ManagedFiles {
		if rel == manifestPath || rel == "." || path.Clean(rel) != rel || strings.ContainsAny(rel, "\\:\x00") || strings.HasPrefix(rel, "/") || !isManaged(rel) || isProjectState(rel) {
			return nil, fmt.Errorf("invalid inventory path %q", rel)
		}
		decoded, e := hex.DecodeString(hash)
		if e != nil || len(decoded) != sha256.Size {
			return nil, fmt.Errorf("invalid inventory hash for %s", rel)
		}
	}
	if !source {
		return &m, nil
	}
	version, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		return nil, err
	}
	if m.Version != strings.TrimSpace(string(version)) {
		return nil, fmt.Errorf("manifest version differs from VERSION")
	}
	files, err := listManagedFiles(root)
	if err != nil {
		return nil, err
	}
	seen := 0
	for _, rel := range files {
		if rel == manifestPath {
			continue
		}
		seen++
		want, ok := m.ManagedFiles[rel]
		if !ok {
			return nil, fmt.Errorf("source inventory missing %s", rel)
		}
		actual, err := hashFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(actual, want) {
			return nil, fmt.Errorf("source inventory hash mismatch: %s", rel)
		}
	}
	if seen != len(m.ManagedFiles) {
		return nil, fmt.Errorf("source inventory contains absent files")
	}
	runtimeHash, err := hashFile(filepath.Join(root, "bin", "codea-dcep-tools.exe"))
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(runtimeHash, m.RuntimeSHA256) {
		return nil, fmt.Errorf("manifest runtimeSha256 mismatch")
	}
	return &m, nil
}
func hashFile(p string) (string, error) {
	f, e := os.Open(p)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return "", e
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func removeInventory(root string, inventory *releaseInventory) error {
	if inventory == nil {
		return nil
	}
	for rel := range inventory.ManagedFiles {
		p := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Lstat(p)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		// A user-created directory at a formerly owned file path is not owned.
		if info.IsDir() {
			continue
		}
		if err = os.Remove(p); err != nil {
			return err
		}
		removeEmptyParents(root, rel)
	}
	return nil
}

// Rollback derives its complete file journal from pretransaction backup bytes,
// independently of whichever release manifest was installed before failure.
func listSnapshotFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel != "harness.yaml" {
			files = append(files, rel)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

// Only paths in the apply journal (including file/directory topology beneath
// those paths) belong to rollback. Concurrent or preserved Project State is
// outside the transaction and must never be rewritten from the backup copy.
func listAffectedSnapshotFiles(root string, updated, removed []string) ([]string, error) {
	files, err := listSnapshotFiles(root)
	if err != nil {
		return nil, err
	}
	touched := append(append([]string(nil), updated...), removed...)
	filtered := make([]string, 0, len(files))
	for _, rel := range files {
		if isAffectedTransactionPath(rel, touched) {
			filtered = append(filtered, rel)
		}
	}
	return filtered, nil
}

func restoreAffectedDirectoryModes(backup, target string, updated, removed []string) error {
	touched := append(append([]string(nil), updated...), removed...)
	return filepath.WalkDir(backup, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(backup, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." || !isAffectedTransactionPath(rel, touched) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		destination := filepath.Join(target, filepath.FromSlash(rel))
		if err := os.MkdirAll(destination, info.Mode().Perm()); err != nil {
			return err
		}
		return os.Chmod(destination, info.Mode().Perm())
	})
}

func isAffectedTransactionPath(rel string, touched []string) bool {
	for _, transactionPath := range touched {
		if rel == transactionPath || strings.HasPrefix(rel, transactionPath+"/") || strings.HasPrefix(transactionPath, rel+"/") {
			return true
		}
	}
	return false
}

func validateInventoryCollisions(source, target string, inventory *releaseInventory) error {
	if inventory == nil || inventory.ManagedFiles == nil {
		return nil
	}
	files, err := listManagedFiles(source)
	if err != nil {
		return err
	}
	installed, err := listManagedFiles(target)
	if err != nil {
		return err
	}
	existing := map[string]bool{}
	for _, rel := range installed {
		existing[inventoryPathKey(rel)] = true
	}
	owned := map[string]bool{}
	for rel := range inventory.ManagedFiles {
		owned[inventoryPathKey(rel)] = true
	}
	for _, rel := range files {
		if rel == manifestPath {
			continue
		}
		key := inventoryPathKey(rel)
		if owned[key] {
			continue
		}
		if existing[key] {
			return fmt.Errorf("source file collides with unowned installed path: %s", rel)
		}
	}
	return nil
}

func inventoryPathKey(rel string) string {
	rel = filepath.ToSlash(rel)
	if runtime.GOOS == "windows" {
		return strings.ToLower(rel)
	}
	return rel
}
