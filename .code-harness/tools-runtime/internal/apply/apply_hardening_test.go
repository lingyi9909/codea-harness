package apply

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyPreservesExistingCRLFLineEndings(t *testing.T) {
	root := t.TempDir()
	writePolicy(t, root)
	path := "src/main/java/A.java"
	before := "line1\r\nold\r\n"
	writeRepoFile(t, root, path, before)
	diff := "--- a/" + path + "\n+++ b/" + path + "\n@@ -1,2 +1,2 @@\n line1\n-old\n+new\n"
	req := Request{
		RunID: "run-crlf", PlanType: "FIX", PlanID: "fix-crlf",
		DiffSha256: hashText(diff), UnifiedDiff: diff,
		Files: []FileRequest{{Path: path, BaseSha256: hashText(before)}},
	}
	if _, err := Apply(root, req); err != nil {
		t.Fatalf("CRLF patch rejected: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "line1\r\nnew\r\n" {
		t.Fatalf("line endings changed: %q", got)
	}
}

func TestApplyRequestSchemaRejectsWindowsCaseFoldDuplicatePaths(t *testing.T) {
	root := t.TempDir()
	writePolicy(t, root)
	diff := "--- a/src/main/java/A.java\n+++ b/src/main/java/A.java\n@@ -1 +1 @@\n-old\n+new\n"
	data := []byte(`{
      "runId":"run-case","planType":"FIX","planId":"fix-case",
      "diffSha256":"` + hashText(diff) + `",
      "files":[
        {"path":"src/main/java/A.java","baseSha256":"` + strings.Repeat("a", 64) + `"},
        {"path":"SRC/MAIN/JAVA/a.java","baseSha256":"` + strings.Repeat("b", 64) + `"}
      ],
      "unifiedDiff":"--- a/src/main/java/A.java\n+++ b/src/main/java/A.java\n@@ -1 +1 @@\n-old\n+new\n"
    }`)
	if _, err := DecodeRequest(root, data); err == nil {
		t.Fatal("Windows case-fold duplicate paths must be rejected")
	}
}

func TestTextPatchWithNULIsRejectedAsBinary(t *testing.T) {
	root := t.TempDir()
	writePolicy(t, root)
	path := "src/main/java/A.java"
	before := "old\n"
	writeRepoFile(t, root, path, before)
	diff := "--- a/" + path + "\n+++ b/" + path + "\n@@ -1 +1 @@\n-old\n+new\x00data\n"
	req := Request{RunID: "run-nul", PlanType: "FIX", PlanID: "fix-nul", DiffSha256: hashText(diff), UnifiedDiff: diff, Files: []FileRequest{{Path: path, BaseSha256: hashText(before)}}}
	if _, err := Apply(root, req); err == nil || !strings.Contains(err.Error(), "BINARY_PATCH_NOT_SUPPORTED") {
		t.Fatalf("NUL patch must be rejected as binary, err=%v", err)
	}
	got, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if string(got) != before {
		t.Fatalf("file changed: %q", got)
	}
}

func TestAllowedPathCannotEscapeThroughParentSymlink(t *testing.T) {
	root := t.TempDir()
	writePolicy(t, root)
	external := t.TempDir()
	writeRepoFile(t, external, "A.java", "old\n")
	linkParent := filepath.Join(root, "src", "main", "java", "linked")
	if err := os.MkdirAll(filepath.Dir(linkParent), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, linkParent); err != nil {
		t.Skipf("symlink unavailable on runner: %v", err)
	}
	path := "src/main/java/linked/A.java"
	req := singleFileRequest("FIX", "run-link", "fix-link", path, "old\n", "new\n")
	if _, err := Apply(root, req); err == nil || !strings.Contains(err.Error(), "UNSAFE_SYMLINK_PATH") {
		t.Fatalf("symlink escape must be rejected, err=%v", err)
	}
	got, err := os.ReadFile(filepath.Join(external, "A.java"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old\n" {
		t.Fatalf("external file changed: %q", got)
	}
}
