package changeset

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func Test153ComputeChangeSetIncludesCommittedStagedUnstagedUntracked(t *testing.T) {
	repo := new153GitRepo(t)
	write153(t, repo, "src/main/java/acme/CController.java", "class CController {\n  void oldValue() {}\n}\n")
	write153(t, repo, "seed.txt", "seed\n")
	git153(t, repo, "add", ".")
	git153(t, repo, "commit", "-m", "base")

	write153(t, repo, "src/main/java/acme/AController.java", "class AController {\n  void create() {}\n}\n")
	git153(t, repo, "add", "src/main/java/acme/AController.java")
	git153(t, repo, "commit", "-m", "add A")

	// Same path appears in committed + staged sources and must be merged once.
	write153(t, repo, "src/main/java/acme/AController.java", "class AController {\n  void create() {}\n  void stagedChange() {}\n}\n")
	git153(t, repo, "add", "src/main/java/acme/AController.java")
	write153(t, repo, "src/main/java/acme/BController.java", "class BController {\n  void submit() {}\n}\n")
	git153(t, repo, "add", "src/main/java/acme/BController.java")
	write153(t, repo, "src/main/java/acme/CController.java", "class CController {\n  void changedValue() {}\n}\n")
	write153(t, repo, "src/main/java/acme/DController.java", "class DController {\n  void untracked() {}\n}\n")

	snap, err := Compute(repo, "HEAD~1", true)
	if err != nil {
		t.Fatal(err)
	}
	assert153Source(t, snap, "src/main/java/acme/AController.java", SourceCommitted)
	assert153Source(t, snap, "src/main/java/acme/AController.java", SourceStaged)
	assert153Source(t, snap, "src/main/java/acme/BController.java", SourceStaged)
	assert153Source(t, snap, "src/main/java/acme/CController.java", SourceUnstaged)
	assert153Source(t, snap, "src/main/java/acme/DController.java", SourceUntracked)

	if got := len(snap.Files); got != 4 {
		t.Fatalf("files=%d want 4: %+v", got, snap.Files)
	}
	paths := make([]string, 0, len(snap.Files))
	for _, f := range snap.Files {
		paths = append(paths, f.Path)
	}
	if !sort.StringsAreSorted(paths) {
		t.Fatalf("files are not deterministically sorted: %v", paths)
	}
	a := file153(t, snap, "src/main/java/acme/AController.java")
	wantSources := []Source{SourceCommitted, SourceStaged}
	if !reflect.DeepEqual(a.Sources, wantSources) {
		t.Fatalf("A sources=%v want %v", a.Sources, wantSources)
	}
	if len(a.Hunks) == 0 || len(file153(t, snap, "src/main/java/acme/CController.java").Hunks) == 0 {
		t.Fatal("tracked changed files must retain unified hunk ranges")
	}
	if snap.BaseRef != "HEAD~1" || strings.TrimSpace(snap.Head) == "" {
		t.Fatalf("snapshot identity missing: %+v", snap)
	}
	if len(snap.SHA256) != 64 {
		t.Fatalf("sha256=%q", snap.SHA256)
	}

	again, err := Compute(repo, "HEAD~1", true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(snap, again) {
		t.Fatalf("Compute is not deterministic:\nfirst=%+v\nagain=%+v", snap, again)
	}
}

func Test153ComputeChangeSetExcludesWorkingTreeWhenDisabled(t *testing.T) {
	repo := new153GitRepo(t)
	write153(t, repo, "seed.txt", "seed\n")
	git153(t, repo, "add", ".")
	git153(t, repo, "commit", "-m", "base")
	write153(t, repo, "src/main/java/acme/AController.java", "class AController {}\n")
	git153(t, repo, "add", "src/main/java/acme/AController.java")
	git153(t, repo, "commit", "-m", "committed")
	write153(t, repo, "src/main/java/acme/BController.java", "class BController {}\n")
	git153(t, repo, "add", "src/main/java/acme/BController.java")
	write153(t, repo, "src/main/java/acme/CController.java", "class CController {}\n")

	snap, err := Compute(repo, "HEAD~1", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Files) != 1 || snap.Files[0].Path != "src/main/java/acme/AController.java" {
		t.Fatalf("working tree leaked into disabled snapshot: %+v", snap.Files)
	}
	if !reflect.DeepEqual(snap.Files[0].Sources, []Source{SourceCommitted}) {
		t.Fatalf("sources=%v", snap.Files[0].Sources)
	}
}

func Test153ComputeRejectsMissingLocalBaseRef(t *testing.T) {
	repo := new153GitRepo(t)
	write153(t, repo, "seed.txt", "seed\n")
	git153(t, repo, "add", ".")
	git153(t, repo, "commit", "-m", "base")
	if _, err := Compute(repo, "origin/does-not-exist", true); err == nil {
		t.Fatal("missing local baseRef must fail; Compute must never fetch or substitute")
	}
}

func new153GitRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	git153(t, repo, "init")
	git153(t, repo, "config", "user.email", "task153@example.test")
	git153(t, repo, "config", "user.name", "Task 153")
	git153(t, repo, "config", "core.autocrlf", "false")
	return repo
}

func git153(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write153(t *testing.T, repo, rel, content string) {
	t.Helper()
	p := filepath.Join(repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func file153(t *testing.T, snap Snapshot, path string) File {
	t.Helper()
	for _, f := range snap.Files {
		if f.Path == path {
			return f
		}
	}
	t.Fatalf("missing %s in %+v", path, snap.Files)
	return File{}
}

func assert153Source(t *testing.T, snap Snapshot, path string, source Source) {
	t.Helper()
	f := file153(t, snap, path)
	for _, got := range f.Sources {
		if got == source {
			return
		}
	}
	t.Fatalf("%s sources=%v missing %s", path, f.Sources, source)
}
