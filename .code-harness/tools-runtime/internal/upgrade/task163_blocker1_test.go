package upgrade

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type rollbackTreeEntry struct {
	Mode os.FileMode
	Data string
}

func snapshotRollbackTree(t *testing.T, root string) map[string]rollbackTreeEntry {
	t.Helper()
	state := map[string]rollbackTreeEntry{}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		item := rollbackTreeEntry{Mode: info.Mode()}
		if !entry.IsDir() {
			contents, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			item.Data = string(contents)
		}
		state[filepath.ToSlash(rel)] = item
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return state
}

func Test163Blocker1ValidLongManagedFilenameUpgrades(t *testing.T) {
	source, target := makeDeltaPair(t)
	name := strings.Repeat("x", 250)
	write(t, source, "agents/"+name, "new\n")

	result := Run(Options{SourceDir: source, TargetDir: target, Refs: StaticRefs{RemoteBranches: []string{"origin/develop"}}})
	if result.Status != StatusUpgraded {
		t.Fatalf("result=%+v", result)
	}
	if got, err := os.ReadFile(filepath.Join(target, "agents", name)); err != nil || string(got) != "new\n" {
		t.Fatalf("long managed file=%q err=%v", got, err)
	}
}

func Test163Blocker1RollbackRestoresRemoveUpdateAndAddAfterPartialApply(t *testing.T) {
	source, target := makeDeltaPair(t)
	write(t, target, "tools/retired/removed.txt", "restore me\n")
	if err := os.Chmod(filepath.Join(target, "tools", "retired"), 0o750); err != nil {
		t.Fatal(err)
	}
	write(t, target, "agents/b-update.md", "old update\n")
	write(t, source, "agents/a-add.md", "remove on rollback\n")
	write(t, source, "agents/b-update.md", "new update\n")
	if err := os.MkdirAll(filepath.Join(target, "tools", "company-empty", "nested"), 0o750); err != nil {
		t.Fatal(err)
	}
	writeInventory(t, target)
	writeInventory(t, source)
	wantInventory, err := os.ReadFile(filepath.Join(target, "RELEASE-MANIFEST.json"))
	if err != nil {
		t.Fatal(err)
	}
	wantState := snapshotRollbackTree(t, target)
	backup := filepath.Join(t.TempDir(), "backup")
	if err := copyTree(target, backup, nil); err != nil {
		t.Fatal(err)
	}
	updated := []string{"RELEASE-MANIFEST.json", "agents/a-add.md", "agents/b-update.md", "agents/z-missing.md"}
	removed := []string{"tools/retired/removed.txt"}
	err = applyStagedDelta(source, target, "", updated, removed)
	if err == nil {
		t.Fatal("expected real missing staged file failure after partial apply")
	}
	if _, err := os.Stat(filepath.Join(target, "tools", "retired", "removed.txt")); !os.IsNotExist(err) {
		t.Fatalf("REMOVE was not applied before failure: %v", err)
	}
	for rel, want := range map[string]string{"agents/a-add.md": "remove on rollback\n", "agents/b-update.md": "new update\n"} {
		got, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(rel)))
		if err != nil || string(got) != want {
			t.Fatalf("partial apply did not reach %s: got=%q err=%v", rel, got, err)
		}
	}
	if got, err := os.ReadFile(filepath.Join(target, "RELEASE-MANIFEST.json")); err != nil || string(got) == string(wantInventory) {
		t.Fatalf("partial apply did not update release manifest: err=%v", err)
	}
	if err := restoreFromBackup(backup, target, "", updated, removed); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	gotState := snapshotRollbackTree(t, target)
	if !reflect.DeepEqual(gotState, wantState) {
		t.Fatalf("rollback did not restore complete pretransaction tree:\nwant=%s\ngot=%s", fmt.Sprint(wantState), fmt.Sprint(gotState))
	}
	if gotState["RELEASE-MANIFEST.json"].Data != string(wantInventory) {
		t.Fatal("complete-state comparison omitted installed inventory")
	}
}

func Test163Blocker1RollbackFailureRetainsRecoveryStageAndBackup(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, ".code-harness")
	backup := filepath.Join(root, ".code-harness-backup-test")
	stage := filepath.Join(root, ".code-harness-stage-test")
	write(t, target, "harness.yaml", "installed\n")
	write(t, backup, "AGENTS.md", "backup\n") // Missing backup harness.yaml forces rollback failure.
	write(t, stage, "AGENTS.md", "stage\n")

	result := failAndRollback(Result{}, errors.New("forced apply failure"), stage, backup, target, "", []string{"AGENTS.md"}, nil)
	if result.Status != StatusUpgradeFailed || result.RollbackPerformed {
		t.Fatalf("result=%+v", result)
	}
	for _, path := range []string{stage, backup} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("recovery artifact removed %s: %v", path, err)
		}
		if !strings.Contains(strings.Join(result.Errors, "\n"), path) {
			t.Fatalf("recovery path missing from errors: path=%s errors=%v", path, result.Errors)
		}
	}
}
