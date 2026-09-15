package upgrade

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const primaryCommand167 = ".opencode/commands/harness-review.md"

func makeHostUpgrade167Pair(t *testing.T) (string, string) {
	t.Helper()
	s, d, _ := makeHostUpgrade166Pair(t, "1.6.6")
	write(t, s, "VERSION", "1.6.7\n")
	write(t, s, "host/"+primaryCommand167, "primary review command")
	write(t, s, "commands/harness-review.md", "primary review command")
	write165HostInventory(t, s, filepath.Join(s, "host"))
	b, err := os.ReadFile(filepath.Join(s, manifestPath))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	h := m["hostAgents"].(map[string]any)["reviewer"].(map[string]any)
	hash, err := hashFile(filepath.Join(s, "host", primaryCommand167))
	if err != nil {
		t.Fatal(err)
	}
	h["primaryCommand"], h["primaryCommandUpgradeSource"], h["primaryCommandSha256"] = primaryCommand167, "host/"+primaryCommand167, hash
	b, err = json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	write(t, s, manifestPath, string(b))
	return s, d
}

func Test167UpgradeInstallsPrimaryCommand(t *testing.T) {
	s, d := makeHostUpgrade167Pair(t)
	r := Run(Options{SourceDir: s, TargetDir: d})
	if r.Status != StatusUpgraded {
		t.Fatalf("upgrade: %+v", r)
	}
	if _, err := os.Stat(filepath.Join(d, "commands/harness-review.md")); err != nil {
		t.Fatalf("framework command not installed: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(filepath.Dir(d), primaryCommand167))
	if err != nil || string(b) != "primary review command" || !contains(r.UpdatedFiles, primaryCommand167) {
		t.Fatalf("command missing: %q %v %+v", b, err, r)
	}
}

func Test167PrimaryCommandConflictAndRollback(t *testing.T) {
	for _, kind := range []string{"conflict", "tamper", "missing", "rollback"} {
		t.Run(kind, func(t *testing.T) {
			s, d := makeHostUpgrade167Pair(t)
			switch kind {
			case "conflict":
				write(t, filepath.Dir(d), primaryCommand167, "user command")
			case "tamper":
				write(t, s, "host/"+primaryCommand167, "tampered")
			case "missing":
				if err := os.Remove(filepath.Join(s, "host", primaryCommand167)); err != nil {
					t.Fatal(err)
				}
			case "rollback":
				old := reviewerHostInstallHook
				reviewerHostInstallHook = func(i int, rel string) error {
					if rel == primaryCommand167 {
						return errors.New("injected primary command failure")
					}
					return nil
				}
				t.Cleanup(func() { reviewerHostInstallHook = old })
			}
			before := snapshotRollbackTree(t, d)
			host := filepath.Join(filepath.Dir(d), ".opencode")
			hostBefore := snapshotRollbackTree(t, host)
			r := Run(Options{SourceDir: s, TargetDir: d})
			if kind == "rollback" {
				if r.Status != StatusUpgradeFailed || !r.RollbackPerformed {
					t.Fatalf("rollback: %+v", r)
				}
			} else if r.Status != StatusManualActionRequired {
				t.Fatalf("accepted %s: %+v", kind, r)
			}
			if !reflect.DeepEqual(before, snapshotRollbackTree(t, d)) || !reflect.DeepEqual(hostBefore, snapshotRollbackTree(t, host)) {
				t.Fatal("installation changed")
			}
		})
	}
}
