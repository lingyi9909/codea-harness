package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func Test162HotfixAnalysisSnapshotPublishesRuntimeOwnedCanonicalArtifact(t *testing.T) {
	root := t.TempDir()
	git153Cmd(t, root, "init")
	git153Cmd(t, root, "config", "user.email", "task162@example.test")
	git153Cmd(t, root, "config", "user.name", "Task 162 Hotfix")
	mustWrite153Cmd(t, filepath.Join(root, "src", "main", "java", "acme", "AService.java"), "class AService {}\n")
	git153Cmd(t, root, "add", ".")
	git153Cmd(t, root, "commit", "-m", "base")

	req := map[string]any{"runId": "r162hotfix", "baseRef": "HEAD", "includeWorkingTree": true}
	b, err := json.Marshal(req)
	if err != nil { t.Fatal(err) }
	requestPath := filepath.Join(root, ".code-harness", "runs", "r162hotfix", "requests", "change-set.json")
	mustWrite153Cmd(t, requestPath, string(b))

	withChdir153Cmd(t, root, func() {
		if err := run([]string{"analysis", "snapshot", "--input", ".code-harness/runs/r162hotfix/requests/change-set.json"}); err != nil {
			t.Fatalf("analysis snapshot failed: %v", err)
		}
		artifact := filepath.Join(".code-harness", "runs", "r162hotfix", "analysis", "change-set.json")
		data, err := os.ReadFile(artifact)
		if err != nil { t.Fatalf("read canonical snapshot: %v", err) }
		var doc map[string]any
		if err := json.Unmarshal(data, &doc); err != nil { t.Fatal(err) }
		if doc["requestedBaseRef"] != "HEAD" { t.Fatalf("unexpected snapshot: %s", data) }
		if got, _ := doc["snapshotSha256"].(string); len(got) != 64 { t.Fatalf("snapshotSha256=%q", got) }
	})
}
