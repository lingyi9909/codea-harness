package changeset

import (
	"strings"
	"testing"
)

func Test164VerifyFreshnessRejectsSelfConsistentRehashedProjectionTamper(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(*testing.T) (string, Snapshot)
		tamper func(*testing.T, Snapshot) Snapshot
	}{
		{
			name:  "files_path",
			setup: func(t *testing.T) (string, Snapshot) { return new164FreshnessSnapshot(t, true) },
			tamper: func(t *testing.T, snapshot Snapshot) Snapshot {
				t.Helper()
				snapshot.Files[0].Path = "src/main/java/acme/AAAA.java"
				return rehash164TamperedSnapshot(t, snapshot)
			},
		},
		{
			name:  "status",
			setup: func(t *testing.T) (string, Snapshot) { return new164FreshnessSnapshot(t, true) },
			tamper: func(t *testing.T, snapshot Snapshot) Snapshot {
				t.Helper()
				snapshot.Files[0].Status = "A"
				return rehash164TamperedSnapshot(t, snapshot)
			},
		},
		{
			name:  "sources",
			setup: func(t *testing.T) (string, Snapshot) { return new164FreshnessSnapshot(t, true) },
			tamper: func(t *testing.T, snapshot Snapshot) Snapshot {
				t.Helper()
				snapshot.Files[0].Sources = []Source{SourceStaged}
				return rehash164TamperedSnapshot(t, snapshot)
			},
		},
		{
			name:  "hunks",
			setup: func(t *testing.T) (string, Snapshot) { return new164FreshnessSnapshot(t, true) },
			tamper: func(t *testing.T, snapshot Snapshot) Snapshot {
				t.Helper()
				if len(snapshot.Files[0].Hunks) == 0 {
					t.Fatal("fixture must contain a committed hunk")
				}
				snapshot.Files[0].Hunks[0].NewStart++
				return rehash164TamperedSnapshot(t, snapshot)
			},
		},
		{
			name: "untracked_file_removed",
			setup: func(t *testing.T) (string, Snapshot) {
				repo, _ := new164FreshnessSnapshot(t, true)
				write153(t, repo, "src/main/java/acme/UntrackedProjection.java", "class UntrackedProjection {}\n")
				return compute164Snapshot(t, repo, true)
			},
			tamper: func(t *testing.T, snapshot Snapshot) Snapshot {
				t.Helper()
				out := make([]File, 0, len(snapshot.Files))
				for _, file := range snapshot.Files {
					if file.Path != "src/main/java/acme/UntrackedProjection.java" {
						out = append(out, file)
					}
				}
				snapshot.Files = out
				return rehash164TamperedSnapshot(t, snapshot)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo, snapshot := tc.setup(t)
			if err := VerifyFreshness(repo, snapshot); err != nil {
				t.Fatalf("fixture must start fresh: %v", err)
			}
			tampered := tc.tamper(t, snapshot)
			if tampered.GitStateSHA256 != snapshot.GitStateSHA256 {
				t.Fatalf("tamper regression must preserve GitStateSHA256: got=%s want=%s", tampered.GitStateSHA256, snapshot.GitStateSHA256)
			}
			if tampered.SnapshotSHA256 == snapshot.SnapshotSHA256 {
				t.Fatal("tamper regression must be self-consistently rehashed to a different snapshot identity")
			}
			err := VerifyFreshness(repo, tampered)
			if err == nil {
				t.Fatal("self-consistent projection tamper must be rejected")
			}
			if !strings.Contains(err.Error(), "CHANGE_SET_SNAPSHOT_STALE") {
				t.Fatalf("projection tamper must fail with stale authority, got: %v", err)
			}
		})
	}
}

func rehash164TamperedSnapshot(t *testing.T, snapshot Snapshot) Snapshot {
	t.Helper()
	snapshot.SnapshotSHA256 = ""
	snapshot.SHA256 = ""
	rehashed, err := finalizeSnapshot162(snapshot)
	if err != nil {
		t.Fatalf("rehash self-consistent tampered snapshot: %v", err)
	}
	return rehashed
}
