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

func writeAcceptanceInventory(t *testing.T, root string) {
	t.Helper()
	files, err := listManagedFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	inventory := map[string]string{}
	for _, rel := range files {
		if rel == "RELEASE-MANIFEST.json" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		inventory[rel] = fmt.Sprintf("%x", sha256.Sum256(b))
	}
	version, _ := os.ReadFile(filepath.Join(root, "VERSION"))
	manifest, _ := json.Marshal(map[string]any{
		"version":       strings.TrimSpace(string(version)),
		"runtimeSha256": inventory["bin/codea-dcep-tools.exe"],
		"buildCommit":   "0123456789012345678901234567890123456789",
		"managedFiles":  inventory,
	})
	write(t, root, "RELEASE-MANIFEST.json", string(manifest))
}

func Test163AcceptanceREDValidLongManagedFilenameUpgrades(t *testing.T) {
	source, target := makeDeltaPair(t)
	name := strings.Repeat("x", 250)
	write(t, source, "agents/"+name, "new\n")
	result := Run(Options{SourceDir: source, TargetDir: target})
	if result.Status != StatusUpgraded {
		t.Fatalf("long filename: %+v", result)
	}
}

func Test163AcceptanceREDInventoryOwnership(t *testing.T) {
	source, target := makeDeltaPair(t)
	write(t, target, "tools/obsolete.txt", "owned")
	writeAcceptanceInventory(t, target)
	write(t, target, "tools/company-local-notes.txt", "keep")
	writeAcceptanceInventory(t, source)
	result := Run(Options{SourceDir: source, TargetDir: target})
	if result.Status != StatusUpgraded {
		t.Fatalf("result=%+v", result)
	}
	if _, err := os.Stat(filepath.Join(target, "tools/obsolete.txt")); !os.IsNotExist(err) {
		t.Fatal("owned stale file remains")
	}
	if b, err := os.ReadFile(filepath.Join(target, "tools/company-local-notes.txt")); err != nil || string(b) != "keep" {
		t.Fatalf("unknown user file lost: %q %v", b, err)
	}
}

func Test163AcceptanceREDInstalledManifest(t *testing.T) {
	source, target := makeDeltaPair(t)
	write(t, target, "bin/codea-dcep-tools.exe", "old runtime")
	writeAcceptanceInventory(t, target)
	writeAcceptanceInventory(t, source)
	want, _ := os.ReadFile(filepath.Join(source, "RELEASE-MANIFEST.json"))
	result := Run(Options{SourceDir: source, TargetDir: target})
	if result.Status != StatusUpgraded {
		t.Fatalf("result=%+v", result)
	}
	got, err := os.ReadFile(filepath.Join(target, "RELEASE-MANIFEST.json"))
	if err != nil || string(got) != string(want) {
		t.Fatalf("installed manifest not updated: %s %v", got, err)
	}
}

func Test163AcceptanceREDInvalidSourceInventoryFailsClosed(t *testing.T) {
	source, target := makeDeltaPair(t)
	writeAcceptanceInventory(t, source)
	manifestPath := filepath.Join(source, "RELEASE-MANIFEST.json")
	b, _ := os.ReadFile(manifestPath)
	var manifest map[string]any
	_ = json.Unmarshal(b, &manifest)
	manifest["runtimeSha256"] = strings.Repeat("0", 64)
	b, _ = json.Marshal(manifest)
	_ = os.WriteFile(manifestPath, b, 0644)
	result := Run(Options{SourceDir: source, TargetDir: target})
	if result.Status != StatusManualActionRequired {
		t.Fatalf("invalid inventory accepted: %+v", result)
	}
}
