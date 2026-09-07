package analysis

import (
	"context"
	"strings"
	"testing"

	"codea-harness-tools/internal/changeset"
)

func Test164Task1BMissingVerifiedMergeBaseFailsClosed(t *testing.T) {
	root := t.TempDir()
	git164(t, root, "init")
	git164(t, root, "config", "user.email", "task164@example.invalid")
	git164(t, root, "config", "user.name", "Task 164")
	p := "src/main/java/demo/A.java"
	writeJava164(t, root, p, "class A {}\n")
	git164(t, root, "add", ".")
	git164(t, root, "commit", "-m", "base")
	snapshot := changeset.Snapshot{
		BaseRef: "HEAD",
		Files: []changeset.File{{Path: p, Status: "M"}},
	}
	stats := entrypointBatchStats164{}
	_, _, err := loadEntrypointBaseSources164(context.Background(), root, snapshot, []string{p}, &stats)
	if err == nil || !strings.Contains(err.Error(), "ENTRYPOINT_BASE_SOURCE_OUT_OF_SCOPE") {
		t.Fatalf("missing verified MergeBase must fail closed, got %v", err)
	}
	if stats.MergeBaseProcesses != 0 || stats.BaseGitBatchProcesses != 0 {
		t.Fatalf("unverified snapshot launched Git processes: %+v", stats)
	}
}
