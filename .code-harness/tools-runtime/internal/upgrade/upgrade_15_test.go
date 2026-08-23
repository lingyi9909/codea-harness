package upgrade

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChainsAreNeverFrameworkManaged(t *testing.T) {
	root := t.TempDir()
	write(t, root, "chains/order-approve.yaml", "user-chain\n")
	write(t, root, "skills/x/SKILL.md", "framework\n")

	managed, err := listManagedFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if contains(managed, "chains/order-approve.yaml") {
		t.Fatalf("chains/** must not be Framework Managed: %v", managed)
	}
	if !contains(managed, "skills/x/SKILL.md") {
		t.Fatalf("control framework path missing from managed list: %v", managed)
	}
	if isManaged("chains/order-approve.yaml") {
		t.Fatal("chains/** must be Project State")
	}
}

func TestUpgradePreservesChainsByteForByteAndReportsProjectState(t *testing.T) {
	source, target := makePair(t, validConfig("review:\n  baseRef: origin/develop\n  includeWorkingTree: true\n"))
	original := []byte("# user-owned business knowledge\r\nversion: 1\r\nid: order-approve\r\n")
	chainPath := filepath.Join(target, "chains", "order-approve.yaml")
	if err := os.MkdirAll(filepath.Dir(chainPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(chainPath, original, 0o644); err != nil {
		t.Fatal(err)
	}

	// Even a malformed package that contains a business Chain instance must not
	// replace user-owned Project State during the managed-tree transaction.
	write(t, source, "chains/order-approve.yaml", "package-must-not-overwrite\n")

	result := Run(Options{SourceDir: source, TargetDir: target, Refs: StaticRefs{RemoteBranches: []string{"origin/develop"}}})
	if result.Status != StatusUpgraded {
		t.Fatalf("result=%+v", result)
	}
	got, err := os.ReadFile(chainPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("chains/** changed during upgrade: got %q want %q", got, original)
	}
	if !contains(result.PreservedFiles, "chains/**") {
		t.Fatalf("chains/** missing from preservedFiles: %v", result.PreservedFiles)
	}
	if contains(result.UpdatedFiles, "chains/order-approve.yaml") || contains(result.RemovedFiles, "chains/order-approve.yaml") {
		t.Fatalf("chains/** must never appear in managed update/remove evidence: updated=%v removed=%v", result.UpdatedFiles, result.RemovedFiles)
	}
}
