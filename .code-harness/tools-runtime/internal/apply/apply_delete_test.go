package apply

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFixPlanCanDeleteAllowedProductionFile(t *testing.T) {
	root := t.TempDir()
	writePolicy(t, root)
	path := "src/main/java/DeleteMe.java"
	before := "class DeleteMe {}\n"
	writeRepoFile(t, root, path, before)
	diff := "--- a/" + path + "\n+++ /dev/null\n@@ -1 +0,0 @@\n-class DeleteMe {}\n"
	req := Request{
		RunID: "run-delete", PlanType: "FIX", PlanID: "fix-delete",
		DiffSha256: hashText(diff), UnifiedDiff: diff,
		Files: []FileRequest{{Path: path, BaseSha256: hashText(before)}},
	}
	res, err := Apply(root, req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusApplied || len(res.Files) != 1 || res.Files[0].AfterSha256 != hashText("") {
		t.Fatalf("result=%+v", res)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("deleted file still exists: %v", err)
	}
}
