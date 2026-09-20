package upgrade

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeInventory(t *testing.T, root string) {
	t.Helper()
	files, err := listManagedFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	inv := map[string]string{}
	for _, rel := range files {
		if rel == "RELEASE-MANIFEST.json" {
			continue
		}
		b, e := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if e != nil {
			t.Fatal(e)
		}
		inv[rel] = fmt.Sprintf("%x", sha256.Sum256(b))
	}
	v, _ := os.ReadFile(filepath.Join(root, "VERSION"))
	b, _ := json.Marshal(map[string]any{"version": strings.TrimSpace(string(v)), "runtimeSha256": inv["bin/codea-dcep-tools.exe"], "buildCommit": "0123456789012345678901234567890123456789", "managedFiles": inv})
	write(t, root, "RELEASE-MANIFEST.json", string(b))
}
func Test163Task4InventoryPreservesUnknownAndRemovesOwned(t *testing.T) {
	s, d := makeDeltaPair(t)
	write(t, d, "tools/obsolete.txt", "owned")
	writeInventory(t, d)
	write(t, d, "tools/user.txt", "keep")
	writeInventory(t, s)
	want, _ := os.ReadFile(filepath.Join(s, "RELEASE-MANIFEST.json"))
	r := Run(Options{SourceDir: s, TargetDir: d})
	if r.Status != StatusUpgraded {
		t.Fatalf("%+v", r)
	}
	if b, e := os.ReadFile(filepath.Join(d, "tools/user.txt")); e != nil || string(b) != "keep" {
		t.Fatalf("unknown lost %q %v", b, e)
	}
	if _, e := os.Stat(filepath.Join(d, "tools/obsolete.txt")); !os.IsNotExist(e) {
		t.Fatal("owned remains")
	}
	if b, e := os.ReadFile(filepath.Join(d, "RELEASE-MANIFEST.json")); e != nil || string(b) != string(want) {
		t.Fatalf("manifest stale %s %v", b, e)
	}
}
func Test163Task4LegacyUnknownFilesPreserved(t *testing.T) {
	s, d := makeDeltaPair(t)
	write(t, d, "tools/unknown.txt", "keep")
	r := Run(Options{SourceDir: s, TargetDir: d})
	if r.Status != StatusUpgraded {
		t.Fatalf("%+v", r)
	}
	if b, e := os.ReadFile(filepath.Join(d, "tools/unknown.txt")); e != nil || string(b) != "keep" {
		t.Fatalf("legacy unknown lost %q %v", b, e)
	}
}
func Test163Task4MalformedInventoryFailsClosed(t *testing.T) {
	for _, kind := range []string{"traversal", "hash", "version", "runtime", "missing"} {
		t.Run(kind, func(t *testing.T) {
			s, d := makeDeltaPair(t)
			writeInventory(t, s)
			p := filepath.Join(s, "RELEASE-MANIFEST.json")
			b, _ := os.ReadFile(p)
			var m map[string]any
			json.Unmarshal(b, &m)
			inv := m["managedFiles"].(map[string]any)
			switch kind {
			case "traversal":
				inv["../outside"] = fmt.Sprintf("%064d", 0)
			case "hash":
				inv["AGENTS.md"] = "bad"
			case "version":
				m["version"] = "9.9.9"
			case "runtime":
				m["runtimeSha256"] = fmt.Sprintf("%064d", 0)
			case "missing":
				delete(inv, "AGENTS.md")
			}
			b, _ = json.Marshal(m)
			os.WriteFile(p, b, 0644)
			before, _ := os.ReadFile(filepath.Join(d, "VERSION"))
			r := Run(Options{SourceDir: s, TargetDir: d})
			if r.Status != StatusManualActionRequired {
				t.Fatalf("malformed accepted %+v", r)
			}
			after, _ := os.ReadFile(filepath.Join(d, "VERSION"))
			if string(before) != string(after) {
				t.Fatal("target mutated")
			}
		})
	}
}

func Test163Task4InstalledManifestUpdated(t *testing.T) {
	s, d := makeDeltaPair(t)
	write(t, d, "bin/codea-dcep-tools.exe", "previous runtime")
	writeInventory(t, d)
	writeInventory(t, s)
	manifestName := filepath.Join(s, "RELEASE-MANIFEST.json")
	manifestBytes, _ := os.ReadFile(manifestName)
	manifestBytes = []byte(strings.ReplaceAll(string(manifestBytes), "0123456789012345678901234567890123456789", "abcdefabcdefabcdefabcdefabcdefabcdefabcd"))
	if err := os.WriteFile(manifestName, manifestBytes, 0644); err != nil {
		t.Fatal(err)
	}
	want, _ := os.ReadFile(filepath.Join(s, "RELEASE-MANIFEST.json"))
	r := Run(Options{SourceDir: s, TargetDir: d})
	if r.Status != StatusUpgraded {
		t.Fatalf("%+v", r)
	}
	got, e := os.ReadFile(filepath.Join(d, "RELEASE-MANIFEST.json"))
	if e != nil || string(got) != string(want) {
		t.Fatalf("installed manifest not updated: %s %v", got, e)
	}
	var installed releaseInventory
	if err := json.Unmarshal(got, &installed); err != nil {
		t.Fatal(err)
	}
	runtimeHash, err := hashFile(filepath.Join(d, "bin", "codea-dcep-tools.exe"))
	if err != nil {
		t.Fatal(err)
	}
	version, err := os.ReadFile(filepath.Join(d, "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if installed.Version != strings.TrimSpace(string(version)) || installed.RuntimeSHA256 != runtimeHash || installed.BuildCommit != "abcdefabcdefabcdefabcdefabcdefabcdefabcd" {
		t.Fatalf("installed manifest does not describe installed files: %+v runtime=%s", installed, runtimeHash)
	}
}

func Test163Task4InventoryUnknownCollisionFailsClosed(t *testing.T) {
	s, d := makeDeltaPair(t)
	writeInventory(t, d)
	write(t, d, "tools/new-local.txt", "user-owned")
	write(t, s, "tools/new-local.txt", "package-owned")
	writeInventory(t, s)
	r := Run(Options{SourceDir: s, TargetDir: d})
	if r.Status != StatusManualActionRequired {
		t.Fatalf("unknown collision accepted: %+v", r)
	}
	if b, e := os.ReadFile(filepath.Join(d, "tools/new-local.txt")); e != nil || string(b) != "user-owned" {
		t.Fatalf("unknown overwritten: %q %v", b, e)
	}
}

func Test163Task4InstalledInventoryRejectsManifestlessUpgradeSource(t *testing.T) {
	s, d := makeDeltaPair(t)
	writeInventory(t, d)
	r := Run(Options{SourceDir: s, TargetDir: d})
	if r.Status != StatusManualActionRequired || !strings.Contains(strings.Join(r.Errors, "\n"), "missing RELEASE-MANIFEST.json ownership inventory") {
		t.Fatalf("manifestless source accepted over inventoried install: %+v", r)
	}
	if got, err := os.ReadFile(filepath.Join(d, "VERSION")); err != nil || string(got) != "1.1.0\n" {
		t.Fatalf("target changed before fail-closed result: %q %v", got, err)
	}
}
