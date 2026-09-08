package changeset

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test164VerifyFreshnessAcceptsUnchangedSnapshot(t *testing.T) {
	repo, snapshot := new164FreshnessSnapshot(t, true)
	if err := VerifyFreshness(repo, snapshot); err != nil {
		t.Fatalf("unchanged snapshot must be fresh: %v", err)
	}
}

func Test164VerifyFreshnessIgnoresWorkingTreeWhenSnapshotExcludedIt(t *testing.T) {
	repo, snapshot := new164FreshnessSnapshot(t, false)
	write153(t, repo, "src/main/java/acme/Tracked.java", "class Tracked { int value = 99; }\n")
	write153(t, repo, "src/main/java/acme/Untracked.java", "class Untracked {}\n")
	if err := VerifyFreshness(repo, snapshot); err != nil {
		t.Fatalf("working tree must not affect includeWorkingTree=false freshness: %v", err)
	}
}

func Test164VerifyFreshnessRejectsMutationMatrix(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T) (string, Snapshot)
		mutate func(*testing.T, string)
	}{
		{
			name: "head_move",
			setup: func(t *testing.T) (string, Snapshot) { return new164FreshnessSnapshot(t, true) },
			mutate: func(t *testing.T, repo string) {
				write153(t, repo, "src/main/java/acme/HeadMove.java", "class HeadMove {}\n")
				git153(t, repo, "add", ".")
				git153(t, repo, "commit", "-m", "head move")
			},
		},
		{
			name: "base_ref_move",
			setup: func(t *testing.T) (string, Snapshot) { return new164FreshnessSnapshot(t, true) },
			mutate: func(t *testing.T, repo string) { git153(t, repo, "branch", "-f", "task164-base", "HEAD") },
		},
		{
			name: "branch_change",
			setup: func(t *testing.T) (string, Snapshot) { return new164FreshnessSnapshot(t, true) },
			mutate: func(t *testing.T, repo string) { git153(t, repo, "switch", "-c", "task164-other-branch") },
		},
		{
			name: "committed_diff_change",
			setup: func(t *testing.T) (string, Snapshot) { return new164FreshnessSnapshot(t, true) },
			mutate: func(t *testing.T, repo string) {
				write153(t, repo, "src/main/java/acme/Committed.java", "class Committed { int value = 3; }\n")
				git153(t, repo, "add", ".")
				git153(t, repo, "commit", "-m", "change committed diff")
			},
		},
		{
			name: "staged_bytes_change",
			setup: func(t *testing.T) (string, Snapshot) {
				repo, _ := new164FreshnessSnapshot(t, true)
				write153(t, repo, "src/main/java/acme/Tracked.java", "class Tracked { int value = 10; }\n")
				git153(t, repo, "add", "src/main/java/acme/Tracked.java")
				return compute164Snapshot(t, repo, true)
			},
			mutate: func(t *testing.T, repo string) {
				write153(t, repo, "src/main/java/acme/Tracked.java", "class Tracked { int value = 11; }\n")
				git153(t, repo, "add", "src/main/java/acme/Tracked.java")
			},
		},
		{
			name: "staged_add",
			setup: func(t *testing.T) (string, Snapshot) { return new164FreshnessSnapshot(t, true) },
			mutate: func(t *testing.T, repo string) {
				write153(t, repo, "src/main/java/acme/StagedAdded.java", "class StagedAdded {}\n")
				git153(t, repo, "add", "src/main/java/acme/StagedAdded.java")
			},
		},
		{
			name: "staged_delete",
			setup: func(t *testing.T) (string, Snapshot) { return new164FreshnessSnapshot(t, true) },
			mutate: func(t *testing.T, repo string) { git153(t, repo, "rm", "src/main/java/acme/Tracked.java") },
		},
		{
			name: "unstaged_bytes_change",
			setup: func(t *testing.T) (string, Snapshot) {
				repo, _ := new164FreshnessSnapshot(t, true)
				write153(t, repo, "src/main/java/acme/Tracked.java", "class Tracked { int value = 20; }\n")
				return compute164Snapshot(t, repo, true)
			},
			mutate: func(t *testing.T, repo string) { write153(t, repo, "src/main/java/acme/Tracked.java", "class Tracked { int value = 21; }\n") },
		},
		{
			name: "unstaged_add",
			setup: func(t *testing.T) (string, Snapshot) { return new164FreshnessSnapshot(t, true) },
			mutate: func(t *testing.T, repo string) { write153(t, repo, "src/main/java/acme/Tracked.java", "class Tracked { int value = 30; }\n") },
		},
		{
			name: "unstaged_restore",
			setup: func(t *testing.T) (string, Snapshot) {
				repo, _ := new164FreshnessSnapshot(t, true)
				write153(t, repo, "src/main/java/acme/Tracked.java", "class Tracked { int value = 40; }\n")
				return compute164Snapshot(t, repo, true)
			},
			mutate: func(t *testing.T, repo string) { git153(t, repo, "restore", "src/main/java/acme/Tracked.java") },
		},
		{
			name: "untracked_add",
			setup: func(t *testing.T) (string, Snapshot) { return new164FreshnessSnapshot(t, true) },
			mutate: func(t *testing.T, repo string) { write153(t, repo, "src/main/java/acme/UntrackedAdded.java", "class UntrackedAdded {}\n") },
		},
		{
			name: "untracked_delete",
			setup: func(t *testing.T) (string, Snapshot) {
				repo, _ := new164FreshnessSnapshot(t, true)
				write153(t, repo, "src/main/java/acme/Untracked.java", "class Untracked {}\n")
				return compute164Snapshot(t, repo, true)
			},
			mutate: func(t *testing.T, repo string) {
				if err := os.Remove(filepath.Join(repo, "src", "main", "java", "acme", "Untracked.java")); err != nil { t.Fatal(err) }
			},
		},
		{
			name: "untracked_same_path_bytes_change",
			setup: func(t *testing.T) (string, Snapshot) {
				repo, _ := new164FreshnessSnapshot(t, true)
				write153(t, repo, "src/main/java/acme/Untracked.java", "AAAA\n")
				return compute164Snapshot(t, repo, true)
			},
			mutate: func(t *testing.T, repo string) { write153(t, repo, "src/main/java/acme/Untracked.java", "BBBB\n") },
		},
		{
			name: "same_path_same_size_different_bytes",
			setup: func(t *testing.T) (string, Snapshot) {
				repo, _ := new164FreshnessSnapshot(t, true)
				write153(t, repo, "src/main/java/acme/Tracked.java", "class Tracked { int value = 51; }\n")
				return compute164Snapshot(t, repo, true)
			},
			mutate: func(t *testing.T, repo string) { write153(t, repo, "src/main/java/acme/Tracked.java", "class Tracked { int value = 52; }\n") },
		},
		{
			name: "staged_and_unstaged_same_path_mixed_state",
			setup: func(t *testing.T) (string, Snapshot) {
				repo, _ := new164FreshnessSnapshot(t, true)
				write153(t, repo, "src/main/java/acme/Tracked.java", "class Tracked { int value = 60; }\n")
				git153(t, repo, "add", "src/main/java/acme/Tracked.java")
				write153(t, repo, "src/main/java/acme/Tracked.java", "class Tracked { int value = 61; }\n")
				return compute164Snapshot(t, repo, true)
			},
			mutate: func(t *testing.T, repo string) { write153(t, repo, "src/main/java/acme/Tracked.java", "class Tracked { int value = 62; }\n") },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo, snapshot := tc.setup(t)
			if err := VerifyFreshness(repo, snapshot); err != nil {
				t.Fatalf("fixture must start fresh: %v", err)
			}
			tc.mutate(t, repo)
			err := VerifyFreshness(repo, snapshot)
			if err == nil {
				t.Fatal("mutation must be rejected as stale")
			}
			if !strings.Contains(err.Error(), "CHANGE_SET_SNAPSHOT_STALE") {
				t.Fatalf("mutation must fail with stale authority, got: %v", err)
			}
		})
	}
}

func new164FreshnessSnapshot(t *testing.T, includeWorkingTree bool) (string, Snapshot) {
	t.Helper()
	repo := new153GitRepo(t)
	write153(t, repo, "src/main/java/acme/Tracked.java", "class Tracked { int value = 1; }\n")
	write153(t, repo, "src/main/java/acme/Committed.java", "class Committed { int value = 1; }\n")
	git153(t, repo, "add", ".")
	git153(t, repo, "commit", "-m", "task164 base")
	git153(t, repo, "branch", "task164-base")
	write153(t, repo, "src/main/java/acme/Committed.java", "class Committed { int value = 2; }\n")
	git153(t, repo, "add", ".")
	git153(t, repo, "commit", "-m", "task164 current")
	snapshot, err := Compute(repo, "task164-base", includeWorkingTree)
	if err != nil { t.Fatal(err) }
	return repo, snapshot
}

func compute164Snapshot(t *testing.T, repo string, includeWorkingTree bool) (string, Snapshot) {
	t.Helper()
	snapshot, err := Compute(repo, "task164-base", includeWorkingTree)
	if err != nil { t.Fatal(err) }
	return repo, snapshot
}
