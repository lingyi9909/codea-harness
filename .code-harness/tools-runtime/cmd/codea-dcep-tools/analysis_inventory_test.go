package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func Test153AnalysisInventoryWritesRuntimeOwnedArtifact(t *testing.T) {
	root := t.TempDir()
	git153Cmd(t, root, "init")
	git153Cmd(t, root, "config", "user.email", "task153@example.test")
	git153Cmd(t, root, "config", "user.name", "Task 153")
	mustWrite153Cmd(t, filepath.Join(root, "seed.txt"), "seed\n")
	git153Cmd(t, root, "add", ".")
	git153Cmd(t, root, "commit", "-m", "base")

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok { t.Fatal("runtime.Caller failed") }
	schemaSource := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "contracts", "entrypoint-inventory.schema.json"))
	schemaBytes, err := os.ReadFile(schemaSource)
	if err != nil { t.Fatalf("read Task 1 schema: %v", err) }
	mustWrite153Cmd(t, filepath.Join(root, ".code-harness", "contracts", "entrypoint-inventory.schema.json"), string(schemaBytes))

	requestPath := filepath.Join(root, ".code-harness", "runs", "r153", "requests", "entrypoint-inventory.json")
	mustWrite153Cmd(t, requestPath, `{"runId":"r153","baseRef":"HEAD","includeWorkingTree":true,"intent":{"mode":"FULL"}}`)

	withChdir153Cmd(t, root, func() {
		if err := run([]string{"analysis", "inventory", "--input", ".code-harness/runs/r153/requests/entrypoint-inventory.json"}); err != nil {
			t.Fatalf("analysis inventory failed: %v", err)
		}
	})

	artifact := filepath.Join(root, ".code-harness", "runs", "r153", "analysis", "entrypoint-inventory.json")
	b, err := os.ReadFile(artifact)
	if err != nil { t.Fatalf("read inventory artifact: %v", err) }
	var got struct {
		RunID string `json:"runId"`
		Status string `json:"status"`
		Expected []json.RawMessage `json:"expectedEntryPoints"`
		ChangeSetSHA256 string `json:"changeSetSha256"`
	}
	if err := json.Unmarshal(b, &got); err != nil { t.Fatal(err) }
	if got.RunID != "r153" || got.Status != "COMPLETE" || len(got.Expected) != 0 || len(got.ChangeSetSHA256) != 64 {
		t.Fatalf("unexpected inventory artifact: %s", b)
	}
}

func Test153AnalysisInventoryRejectsRunIDPathMismatch(t *testing.T) {
	root := t.TempDir()
	requestPath := filepath.Join(root, ".code-harness", "runs", "r153", "requests", "entrypoint-inventory.json")
	mustWrite153Cmd(t, requestPath, `{"runId":"other","baseRef":"HEAD","includeWorkingTree":true,"intent":{"mode":"FULL"}}`)
	withChdir153Cmd(t, root, func() {
		err := run([]string{"analysis", "inventory", "--input", ".code-harness/runs/r153/requests/entrypoint-inventory.json"})
		if err == nil || !strings.Contains(err.Error(), "RUN_ID_PATH_MISMATCH") {
			t.Fatalf("expected same-run request rejection, got %v", err)
		}
	})
}

func git153Cmd(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil { t.Fatalf("git %v: %v\n%s", args, err, out) }
	return strings.TrimSpace(string(out))
}

func mustWrite153Cmd(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { t.Fatal(err) }
}

func withChdir153Cmd(t *testing.T, dir string, fn func()) {
	t.Helper()
	old, err := os.Getwd(); if err != nil { t.Fatal(err) }
	if err := os.Chdir(dir); err != nil { t.Fatal(err) }
	defer func() { _ = os.Chdir(old) }()
	fn()
}
