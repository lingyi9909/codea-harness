package upgrade

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func makeHostUpgrade166Pair(t *testing.T, oldVersion string) (string, string, []byte) {
	t.Helper()
	s, d := make164To165Pair(t)
	write(t, d, "VERSION", oldVersion+"\n")
	write(t, s, "VERSION", "1.6.6\n")
	// Use the real release submission tool, so the test checks replacement of
	// the executable Host resource that changes in this patch.
	tool, err := os.ReadFile(filepath.Join("..", "..", "..", "tools", "codea-reviewer-submit.ts"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, s, "host/"+reviewerHostFiles164[2], string(tool))
	// An unregistered user file must stay outside the installed ownership map.
	if err := os.Remove(filepath.Join(d, "tools/user.txt")); err != nil {
		t.Fatal(err)
	}
	write165HostInventory(t, d, filepath.Dir(d))
	write165HostInventory(t, s, filepath.Join(s, "host"))
	write(t, d, "tools/user.txt", "unknown sentinel")
	return s, d, tool
}

func Test166UpgradeReplacesInstalledSubmissionToolAndPreservesState(t *testing.T) {
	for _, version := range []string{"1.6.4", "1.6.5"} {
		t.Run(version, func(t *testing.T) {
			s, d, tool := makeHostUpgrade166Pair(t, version)
			preserved := map[string][]byte{}
			for _, p := range []string{"harness.yaml", "project.md", "database.yaml", "chains/payments.yaml", "runs/keep.txt", "tools/user.txt"} {
				b, err := os.ReadFile(filepath.Join(d, p))
				if err != nil {
					t.Fatal(err)
				}
				preserved[p] = b
			}
			r := Run(Options{SourceDir: s, TargetDir: d})
			if r.Status != StatusUpgraded || len(r.Migrations) != 0 {
				t.Fatalf("upgrade: %+v", r)
			}
			got, err := os.ReadFile(filepath.Join(filepath.Dir(d), reviewerHostFiles164[2]))
			if err != nil || !bytes.Equal(got, tool) {
				t.Fatalf("submission tool was not replaced: %v", err)
			}
			for _, rel := range reviewerHostFiles164 {
				if !contains(r.UpdatedFiles, rel) {
					t.Fatalf("Host update not recorded: %s: %+v", rel, r)
				}
			}
			for p, want := range preserved {
				got, err := os.ReadFile(filepath.Join(d, p))
				if err != nil || !bytes.Equal(got, want) {
					t.Fatalf("project state changed: %s: %v", p, err)
				}
			}
			got, err = os.ReadFile(filepath.Join(filepath.Dir(d), ".opencode/user.json"))
			if err != nil || string(got) != "user sentinel" {
				t.Fatalf("user Host config changed: %v", err)
			}
			if _, err := os.Stat(s); !os.IsNotExist(err) {
				t.Fatalf("source not consumed: %v", err)
			}
		})
	}
}

func Test166UpgradeRejectsHostConflictsBeforeWrites(t *testing.T) {
	for _, version := range []string{"1.6.4", "1.6.5"} {
		for _, kind := range []string{"user-tool-edit", "candidate-tool-tamper", "missing-candidate-tool", "missing-installed-ownership"} {
			t.Run(version+"/"+kind, func(t *testing.T) {
				s, d, _ := makeHostUpgrade166Pair(t, version)
				rel := reviewerHostFiles164[2]
				switch kind {
				case "user-tool-edit":
					write(t, filepath.Dir(d), rel, "user-modified tool")
				case "candidate-tool-tamper":
					write(t, s, "host/"+rel, "tampered candidate tool")
				case "missing-candidate-tool":
					if err := os.Remove(filepath.Join(s, "host", rel)); err != nil {
						t.Fatal(err)
					}
				case "missing-installed-ownership":
					writeInventory(t, d)
				}
				before := snapshotRollbackTree(t, d)
				hostRoot := filepath.Join(filepath.Dir(d), ".opencode")
				hostBefore := snapshotRollbackTree(t, hostRoot)
				r := Run(Options{SourceDir: s, TargetDir: d})
				if r.Status != StatusManualActionRequired {
					t.Fatalf("accepted %s: %+v", kind, r)
				}
				if !reflect.DeepEqual(before, snapshotRollbackTree(t, d)) || !reflect.DeepEqual(hostBefore, snapshotRollbackTree(t, hostRoot)) {
					t.Fatal("conflict changed installation")
				}
				if _, err := os.Stat(s); err != nil {
					t.Fatalf("rejected upgrade consumed source: %v", err)
				}
			})
		}
	}
}

func Test166UpgradeRollsBackFrameworkAndPartialHostInstall(t *testing.T) {
	s, d, _ := makeHostUpgrade166Pair(t, "1.6.5")
	before := snapshotRollbackTree(t, d)
	hostRoot := filepath.Join(filepath.Dir(d), ".opencode")
	hostBefore := snapshotRollbackTree(t, hostRoot)
	previous := reviewerHostInstallHook
	reviewerHostInstallHook = func(i int, rel string) error {
		if i == 2 {
			return errors.New("injected submission tool install failure")
		}
		return nil
	}
	t.Cleanup(func() { reviewerHostInstallHook = previous })
	r := Run(Options{SourceDir: s, TargetDir: d})
	if r.Status != StatusUpgradeFailed || !r.RollbackPerformed {
		t.Fatalf("rollback: %+v", r)
	}
	if !reflect.DeepEqual(before, snapshotRollbackTree(t, d)) || !reflect.DeepEqual(hostBefore, snapshotRollbackTree(t, hostRoot)) {
		t.Fatal("rollback changed installation bytes or modes")
	}
}
