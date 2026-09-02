package changeset

import (
	"encoding/json"
	"reflect"
	"testing"
)

func Test162HotfixCanonicalSnapshotPublishesResolvedGitIdentity(t *testing.T) {
	repo := new153GitRepo(t)
	write153(t, repo, "src/main/java/acme/AService.java", "class AService {}\n")
	git153(t, repo, "add", ".")
	git153(t, repo, "commit", "-m", "base")
	head := git153(t, repo, "rev-parse", "HEAD")
	git153(t, repo, "branch", "main")

	snap, err := Compute(repo, "main", true)
	if err != nil { t.Fatal(err) }
	data, err := json.Marshal(snap)
	if err != nil { t.Fatal(err) }
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil { t.Fatal(err) }

	for _, key := range []string{"requestedBaseRef", "resolvedBaseCommit", "mergeBase", "headCommit", "currentBranch", "includeWorkingTree", "files", "gitStateSha256", "snapshotSha256"} {
		if _, ok := doc[key]; !ok { t.Fatalf("canonical snapshot missing %s: %s", key, data) }
	}
	if doc["requestedBaseRef"] != "main" { t.Fatalf("requestedBaseRef=%v", doc["requestedBaseRef"]) }
	if doc["resolvedBaseCommit"] != head || doc["mergeBase"] != head || doc["headCommit"] != head {
		t.Fatalf("resolved Git identity mismatch: %s", data)
	}
	if doc["currentBranch"] == "" { t.Fatalf("currentBranch missing: %s", data) }
	if got, _ := doc["gitStateSha256"].(string); len(got) != 64 { t.Fatalf("gitStateSha256=%q", got) }
	if got, _ := doc["snapshotSha256"].(string); len(got) != 64 { t.Fatalf("snapshotSha256=%q", got) }
	if len(snap.Files) != 0 { t.Fatalf("same HEAD must produce zero committed files: %+v", snap.Files) }
}

func Test162HotfixEquivalentBaseRefsShareCanonicalIdentity(t *testing.T) {
	repo := new153GitRepo(t)
	write153(t, repo, "src/main/java/acme/AService.java", "class AService {}\n")
	git153(t, repo, "add", ".")
	git153(t, repo, "commit", "-m", "base")
	head := git153(t, repo, "rev-parse", "HEAD")
	git153(t, repo, "update-ref", "refs/heads/main", head)
	git153(t, repo, "update-ref", "refs/remotes/origin/main", head)

	refs := []string{"main", "origin/main", "refs/heads/main"}
	var first Snapshot
	for i, ref := range refs {
		snap, err := Compute(repo, ref, false)
		if err != nil { t.Fatalf("Compute(%s): %v", ref, err) }
		if i == 0 { first = snap; continue }
		if snap.SHA256 != first.SHA256 {
			t.Fatalf("equivalent refs must share canonical identity: %s=%s %s=%s", refs[0], first.SHA256, ref, snap.SHA256)
		}
	}
}

func Test162HotfixCanonicalSnapshotFiltersNonReviewFilesWithoutAgentReconciliation(t *testing.T) {
	repo := new153GitRepo(t)
	write153(t, repo, "seed.txt", "seed\n")
	git153(t, repo, "add", ".")
	git153(t, repo, "commit", "-m", "base")

	write153(t, repo, "src/main/java/acme/AService.java", "class AService {}\n")
	write153(t, repo, "pom.xml", "<project/>\n")
	write153(t, repo, "README.md", "changed\n")
	write153(t, repo, "src/main/resources/application.properties", "x=1\n")

	snap, err := Compute(repo, "HEAD", true)
	if err != nil { t.Fatal(err) }
	if got := []string{snap.Files[0].Path}; len(snap.Files) != 1 || !reflect.DeepEqual(got, []string{"src/main/java/acme/AService.java"}) {
		t.Fatalf("canonical Review ChangeSet must contain only Runtime Review scope: %+v", snap.Files)
	}
	if !reflect.DeepEqual(snap.Files[0].Sources, []Source{SourceUntracked}) {
		t.Fatalf("sources=%v", snap.Files[0].Sources)
	}
}

func Test162HotfixSnapshotIdentityChangesWhenReviewBytesChangeAtSameHunk(t *testing.T) {
	repo := new153GitRepo(t)
	write153(t, repo, "src/main/resources/application.yml", "feature: false\n")
	git153(t, repo, "add", ".")
	git153(t, repo, "commit", "-m", "base")

	write153(t, repo, "src/main/resources/application.yml", "feature: true\n")
	first, err := Compute(repo, "HEAD", true)
	if err != nil { t.Fatal(err) }
	write153(t, repo, "src/main/resources/application.yml", "feature: maybe\n")
	second, err := Compute(repo, "HEAD", true)
	if err != nil { t.Fatal(err) }
	if reflect.DeepEqual(first.Files, second.Files) && first.SnapshotSHA256 == second.SnapshotSHA256 {
		t.Fatalf("snapshot identity ignored changed bytes with identical hunk coordinates: %s", first.SnapshotSHA256)
	}
}

func Test162HotfixDistinctResolvedBasesHaveDifferentIdentity(t *testing.T) {
	repo := new153GitRepo(t)
	write153(t, repo, "src/main/java/acme/AService.java", "class AService {}\n")
	git153(t, repo, "add", ".")
	git153(t, repo, "commit", "-m", "base-one")
	firstCommit := git153(t, repo, "rev-parse", "HEAD")
	write153(t, repo, "src/main/java/acme/BService.java", "class BService {}\n")
	git153(t, repo, "add", ".")
	git153(t, repo, "commit", "-m", "base-two")
	secondCommit := git153(t, repo, "rev-parse", "HEAD")

	first, err := Compute(repo, firstCommit, false)
	if err != nil { t.Fatal(err) }
	second, err := Compute(repo, secondCommit, false)
	if err != nil { t.Fatal(err) }
	if first.ResolvedBaseCommit == second.ResolvedBaseCommit || first.SnapshotSHA256 == second.SnapshotSHA256 {
		t.Fatalf("different resolved base commits must remain distinct: first=%+v second=%+v", first, second)
	}
}
