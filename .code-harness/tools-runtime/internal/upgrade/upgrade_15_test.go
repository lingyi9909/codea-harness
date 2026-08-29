package upgrade

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
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

func TestChainTemplateIsFrameworkManaged(t *testing.T) {
	if !isManaged("templates/chain.template.yaml") {
		t.Fatal("templates/chain.template.yaml must be Framework Managed")
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

func TestUpgrade140ToCurrent15xInstallsChainFrameworkAndPreservesAllProjectStateBytes(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, ".code-harness")
	source := filepath.Join(root, ".code-harness-upgrade")
	harnessRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	if err := copyTree(harnessRoot, source, nil); err != nil {
		t.Fatalf("copy release source: %v", err)
	}
	versionBytes, err := os.ReadFile(filepath.Join(source, "VERSION"))
	if err != nil {
		t.Fatalf("read current release VERSION: %v", err)
	}
	currentVersion := strings.TrimSpace(string(versionBytes))
	if currentVersion == "" {
		t.Fatal("current release VERSION must not be empty")
	}

	write(t, source, "bin/codea-harness-tools.exe", "release-runtime-current-1.5.x")
	write(t, source, "bin/ast-grep.exe", "release-ast-grep")
	write(t, source, "chains/package-business.yaml", "must-never-install\n")

	accepted140Config, changed, err := migrateConfigV1ToV2ResourceScopes([]byte(validConfig("review:\n  baseRef: origin/develop\n  includeWorkingTree: true\n")))
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("accepted 1.4 fixture must be migrated to harness config version 2")
	}
	write(t, target, "VERSION", "1.4.0\n")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "harness.yaml"), accepted140Config, 0o644); err != nil {
		t.Fatal(err)
	}
	write(t, target, "AGENTS.md", "accepted-1.4-framework\n")
	write(t, target, "skills/stale-140/SKILL.md", "remove-me\n")
	write(t, target, "bin/codea-harness-tools.exe", "accepted-1.4-runtime")

	state := map[string][]byte{
		"harness.yaml":                    accepted140Config,
		"project.md":                      []byte("# user project\r\nkeep exactly\r\n"),
		"database.yaml":                   []byte("version: 1\r\npassword: user-secret\r\n"),
		"runs/run-140/evidence/result.txt": []byte("accepted-run-evidence\r\n"),
		"chains/order-approve.yaml":        []byte("# user chain\r\nversion: 1\r\nid: order-approve\r\nnotes: keep bytes\r\n"),
	}
	before := make(map[string][32]byte, len(state))
	for rel, data := range state {
		path := filepath.Join(target, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		before[rel] = sha256.Sum256(data)
	}

	result := Run(Options{SourceDir: source, TargetDir: target, Refs: StaticRefs{RemoteBranches: []string{"origin/develop"}}})
	if result.Status != StatusUpgraded || result.FromVersion != "1.4.0" || result.ToVersion != currentVersion {
		t.Fatalf("expected accepted 1.4 -> current %s upgrade, result=%+v", currentVersion, result)
	}

	for rel, wantHash := range before {
		got, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read preserved %s: %v", rel, err)
		}
		if gotHash := sha256.Sum256(got); gotHash != wantHash {
			t.Fatalf("Project State changed for %s: got=%x want=%x", rel, gotHash, wantHash)
		}
	}

	for _, rel := range []string{
		"contracts/chain.schema.json",
		"contracts/chain-validation-result.schema.json",
		"templates/chain.template.yaml",
		"skills/discover-chain/SKILL.md",
		"skills/validate-chain/SKILL.md",
		"tools-runtime/internal/chain/model.go",
		"tools-runtime/internal/reviewscope/chain_context.go",
	} {
		if _, err := os.Stat(filepath.Join(target, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("1.5 Chain Framework missing %s: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(target, "chains", "package-business.yaml")); !os.IsNotExist(err) {
		t.Fatalf("release package business Chain leaked into Project State: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "skills", "stale-140", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("stale 1.4 Framework file survived managed replace")
	}
	if !contains(result.RemovedFiles, "skills/stale-140/SKILL.md") {
		t.Fatalf("stale Framework missing from removedFiles: %v", result.RemovedFiles)
	}
	for _, removed := range result.RemovedFiles {
		if len(removed) >= len("chains/") && removed[:len("chains/")] == "chains/" {
			t.Fatalf("Project State chain appeared in removedFiles: %s", removed)
		}
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("upgrade source must be consumed after success: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(root, ".code-harness-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("stage/backup leaked after current 1.5.x upgrade: %v", matches)
	}
}

func TestCurrentUpgradeFrom152PreservesAllProjectStateBytes(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, ".code-harness")
	source := filepath.Join(root, ".code-harness-upgrade")
	harnessRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	if err := copyTree(harnessRoot, source, nil); err != nil {
		t.Fatalf("copy current release source: %v", err)
	}
	currentVersion := strings.TrimSpace(string(mustRead153Upgrade(t, filepath.Join(source, "VERSION"))))
	if currentVersion != "1.6.0" {
		t.Fatalf("current release source VERSION=%q want 1.6.0", currentVersion)
	}
	write(t, source, "bin/codea-harness-tools.exe", "release-runtime-1.6.0")
	write(t, source, "bin/ast-grep.exe", "release-ast-grep-0.42.1")
	write(t, source, "chains/package-business.yaml", "must-never-install\n")

	accepted152Config, changed, err := migrateConfigV1ToV2ResourceScopes([]byte(validConfig("review:\n  baseRef: origin/custom-release\n  includeWorkingTree: false\n")))
	if err != nil {
		t.Fatalf("build accepted 1.5.2 config fixture: %v", err)
	}
	if !changed {
		t.Fatal("accepted 1.5.2 fixture must use harness config version 2")
	}

	write(t, target, "VERSION", "1.5.2\n")
	write(t, target, "AGENTS.md", "accepted-1.5.2-framework\n")
	write(t, target, "skills/stale-152/SKILL.md", "remove-me\n")
	write(t, target, "bin/codea-harness-tools.exe", "accepted-1.5.2-runtime")
	state := map[string][]byte{
		"harness.yaml":                              accepted152Config,
		"project.md":                                []byte("project-152\r\n"),
		"database.yaml":                             []byte("version: 1\r\npassword: keep-secret\r\n"),
		"runs/run-152/requests/agent-proposal.json": []byte("{\"proposal\":\"must-remain-proposal\"}\r\n"),
		"runs/run-152/evidence/result.bin":          []byte{1, 2, 3, 4, 5},
		"chains/order-approve.yaml":                 []byte("# accepted user chain\r\nversion: 1\r\nid: order-approve\r\nstatus: ACCEPTED\r\n"),
	}
	before := make(map[string][32]byte, len(state))
	for rel, data := range state {
		p := filepath.Join(target, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil { t.Fatal(err) }
		if err := os.WriteFile(p, data, 0o644); err != nil { t.Fatal(err) }
		before[rel] = sha256.Sum256(data)
	}

	result := Run(Options{SourceDir: source, TargetDir: target, Refs: StaticRefs{RemoteBranches: []string{"origin/custom-release"}}})
	if result.Status != StatusUpgraded || result.FromVersion != "1.5.2" || result.ToVersion != currentVersion {
		t.Fatalf("expected exact 1.5.2 -> %s upgrade, result=%+v", currentVersion, result)
	}
	for rel, want := range before {
		got := mustRead153Upgrade(t, filepath.Join(target, filepath.FromSlash(rel)))
		if sum := sha256.Sum256(got); sum != want {
			t.Fatalf("1.5.2 -> %s changed Project State %s: got=%x want=%x", currentVersion, rel, sum, want)
		}
	}
	if got := strings.TrimSpace(string(mustRead153Upgrade(t, filepath.Join(target, "VERSION")))); got != currentVersion {
		t.Fatalf("upgraded VERSION=%q want %s", got, currentVersion)
	}
	if strings.Contains(string(mustRead153Upgrade(t, filepath.Join(target, "harness.yaml"))), "workspaceDependencies:") {
		t.Fatalf("1.5.2 -> %s must not auto-inject workspaceDependencies", currentVersion)
	}
	for _, rel := range []string{
		"contracts/entrypoint-inventory.schema.json",
		"contracts/change-analysis-cert.schema.json",
		"contracts/review-options.schema.json",
		"contracts/chain-edit-request.schema.json",
		"skills/edit-chain/SKILL.md",
	} {
		if _, err := os.Stat(filepath.Join(target, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("current Framework missing preserved 1.5.3 capability %s: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(target, "chains", "package-business.yaml")); !os.IsNotExist(err) {
		t.Fatalf("release package Chain leaked into Project State: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "skills", "stale-152", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("stale 1.5.2 Framework survived managed replace")
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("upgrade source must be consumed after success: %v", err)
	}
}

func mustRead153Upgrade(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}
