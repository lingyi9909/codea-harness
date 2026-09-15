package upgrade

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func write165HostInventory(t *testing.T, root, hostRoot string) {
	t.Helper()
	writeInventory(t, root)
	b, err := os.ReadFile(filepath.Join(root, manifestPath))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	h := map[string]string{"host": "opencode", "mode": "subagent"}
	for i, rel := range reviewerHostFiles164 {
		fields := [][3]string{{"path", "upgradeSource", "sha256"}, {"command", "commandUpgradeSource", "commandSha256"}, {"submissionTool", "submissionToolUpgradeSource", "submissionToolSha256"}}[i]
		hash, err := hashFile(filepath.Join(hostRoot, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		h[fields[0]], h[fields[1]], h[fields[2]] = rel, "host/"+rel, hash
	}
	m["hostAgents"] = map[string]any{"reviewer": h}
	b, err = json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, manifestPath, string(b))
}

func make164To165Pair(t *testing.T) (string, string) {
	t.Helper()
	s, d := makePair(t, task164Real163Config)
	write(t, d, "VERSION", "1.6.4\n")
	write(t, s, "VERSION", "1.6.5\n")
	for _, rel := range reviewerHostFiles164 {
		write(t, filepath.Dir(d), rel, "old host "+rel)
		write(t, s, "host/"+rel, "new host "+rel)
	}
	write165HostInventory(t, d, filepath.Dir(d))
	write165HostInventory(t, s, filepath.Join(s, "host"))
	write(t, d, "database.yaml", "database sentinel")
	write(t, d, "chains/payments.yaml", "chain sentinel")
	write(t, d, "tools/user.txt", "unknown sentinel")
	write(t, filepath.Dir(d), ".opencode/user.json", "user sentinel")
	return s, d
}

func Test165UpgradeUpdatesOwnedHostAndPreservesState(t *testing.T) {
	s, d := make164To165Pair(t)
	preserve := []string{"harness.yaml", "project.md", "database.yaml", "chains/payments.yaml", "runs/keep.txt", "tools/user.txt"}
	before := map[string]string{}
	for _, p := range preserve {
		b, e := os.ReadFile(filepath.Join(d, p))
		if e != nil {
			t.Fatal(e)
		}
		before[p] = string(b)
	}
	r := Run(Options{SourceDir: s, TargetDir: d})
	if r.Status != StatusUpgraded {
		t.Fatalf("%+v", r)
	}
	for _, rel := range reviewerHostFiles164 {
		b, e := os.ReadFile(filepath.Join(filepath.Dir(d), rel))
		if e != nil || string(b) != "new host "+rel {
			t.Fatalf("Host was not upgraded: %s = %q (%v)", rel, b, e)
		}
		if !contains(r.UpdatedFiles, rel) {
			t.Fatalf("missing Host update evidence: %+v", r)
		}
	}
	for p, want := range before {
		b, e := os.ReadFile(filepath.Join(d, p))
		if e != nil || string(b) != want {
			t.Fatalf("state changed: %s", p)
		}
	}
	if len(r.Migrations) != 0 {
		t.Fatalf("unexpected config migration: %+v", r)
	}
	if _, e := os.Stat(s); !os.IsNotExist(e) {
		t.Fatalf("source not consumed: %v", e)
	}
}

func Test165UpgradeRejectsHostProblemsBeforeMutation(t *testing.T) {
	for _, kind := range []string{"user-conflict", "missing-source", "tampered-source", "missing-source-manifest", "missing-target-ownership", "wrong-source-path", "symlink-source"} {
		t.Run(kind, func(t *testing.T) {
			s, d := make164To165Pair(t)
			rel := reviewerHostFiles164[0]
			switch kind {
			case "user-conflict":
				write(t, filepath.Dir(d), rel, "user modified")
			case "missing-source":
				if e := os.RemoveAll(filepath.Join(s, "host")); e != nil {
					t.Fatal(e)
				}
			case "tampered-source":
				write(t, s, "host/"+rel, "tampered")
			case "missing-source-manifest":
				writeInventory(t, s)
			case "missing-target-ownership":
				writeInventory(t, d)
			case "wrong-source-path":
				p := filepath.Join(s, manifestPath)
				b, e := os.ReadFile(p)
				if e != nil {
					t.Fatal(e)
				}
				var m map[string]any
				if e = json.Unmarshal(b, &m); e != nil {
					t.Fatal(e)
				}
				m["hostAgents"].(map[string]any)["reviewer"].(map[string]any)["path"] = ".opencode/agents/other.md"
				b, e = json.Marshal(m)
				if e != nil {
					t.Fatal(e)
				}
				write(t, s, manifestPath, string(b))
			case "symlink-source":
				p := filepath.Join(s, "host", rel)
				b, e := os.ReadFile(p)
				if e != nil {
					t.Fatal(e)
				}
				outside := filepath.Join(t.TempDir(), "outside")
				if e = os.WriteFile(outside, b, 0600); e != nil {
					t.Fatal(e)
				}
				if e = os.Remove(p); e != nil {
					t.Fatal(e)
				}
				if e = os.Symlink(outside, p); e != nil {
					t.Skipf("symlink unavailable: %v", e)
				}
			}
			before := snapshotRollbackTree(t, d)
			hostBefore := snapshotRollbackTree(t, filepath.Join(filepath.Dir(d), ".opencode"))
			r := Run(Options{SourceDir: s, TargetDir: d})
			if r.Status != StatusManualActionRequired {
				t.Fatalf("accepted %s: %+v", kind, r)
			}
			if !reflect.DeepEqual(before, snapshotRollbackTree(t, d)) || !reflect.DeepEqual(hostBefore, snapshotRollbackTree(t, filepath.Join(filepath.Dir(d), ".opencode"))) {
				t.Fatal("preflight mutated installation")
			}
		})
	}
}

func Test165UpgradeRollsBackChangedHostAndFramework(t *testing.T) {
	s, d := make164To165Pair(t)
	before := snapshotRollbackTree(t, d)
	hostRoot := filepath.Join(filepath.Dir(d), ".opencode")
	hostBefore := snapshotRollbackTree(t, hostRoot)
	previous := reviewerHostInstallHook
	reviewerHostInstallHook = func(i int, rel string) error {
		if i == 1 {
			return errors.New("injected partial Host apply failure")
		}
		return nil
	}
	t.Cleanup(func() { reviewerHostInstallHook = previous })
	r := Run(Options{SourceDir: s, TargetDir: d})
	if r.Status != StatusUpgradeFailed || !r.RollbackPerformed {
		t.Fatalf("%+v", r)
	}
	if !reflect.DeepEqual(before, snapshotRollbackTree(t, d)) || !reflect.DeepEqual(hostBefore, snapshotRollbackTree(t, hostRoot)) {
		t.Fatal("rollback did not restore all bytes/modes")
	}
	if _, e := os.Stat(s); e != nil {
		t.Fatalf("failed transaction consumed source: %v", e)
	}
}
