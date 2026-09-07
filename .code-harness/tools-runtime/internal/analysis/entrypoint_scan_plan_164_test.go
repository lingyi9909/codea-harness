package analysis

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"codea-harness-tools/internal/changeset"
)

func Test164Task1AScanPlanIsSnapshotBounded(t *testing.T) {
	root := t.TempDir()
	paths := map[string]string{
		"a": "src/main/java/acme/A.java",
		"b": "src/main/java/acme/B.java",
		"c": "src/main/java/acme/C.java",
		"d": "src/main/java/acme/D.java",
	}
	git164(t, root, "init")
	git164(t, root, "config", "user.email", "task164@example.invalid")
	git164(t, root, "config", "user.name", "Task 164")
	for _, key := range []string{"a", "b", "c"} {
		writeJava164(t, root, paths[key], "class "+strings.ToUpper(key)+" {}\n")
	}
	git164(t, root, "add", ".")
	git164(t, root, "commit", "-m", "base")
	mergeBase := strings.TrimSpace(git164(t, root, "rev-parse", "HEAD"))

	writeJava164(t, root, paths["a"], "class A { int changed; }\n")
	writeJava164(t, root, paths["b"], "class B { int changed; }\n")
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(paths["c"]))); err != nil { t.Fatal(err) }
	writeJava164(t, root, paths["d"], "class D {}\n")

	snapshot := changeset.Snapshot{
		MergeBase: mergeBase,
		SHA256: strings.Repeat("1", 64),
		Files: []changeset.File{
			{Path: paths["a"], Status: "M"},
			{Path: paths["b"], Status: "M"},
			{Path: paths["c"], Status: "D"},
			{Path: paths["d"], Status: "A"},
			{Path: "README.md", Status: "M"},
		},
	}
	plan, err := buildEntrypointScanPlan164(context.Background(), root, "r164-plan", snapshot)
	if err != nil { t.Fatal(err) }
	if want := []string{paths["a"], paths["b"], paths["d"]}; !reflect.DeepEqual(plan.CurrentPaths, want) {
		t.Fatalf("CurrentPaths=%v want=%v", plan.CurrentPaths, want)
	}
	if want := []string{paths["a"], paths["b"], paths["c"]}; !reflect.DeepEqual(plan.BasePaths, want) {
		t.Fatalf("BasePaths=%v want=%v", plan.BasePaths, want)
	}
	if plan.CurrentScopeSHA256 == "" || plan.BaseScopeSHA256 == "" || plan.CurrentScopeSHA256 == plan.BaseScopeSHA256 {
		t.Fatalf("scope hashes not bound correctly: %+v", plan)
	}
	if plan.baseGitBatchProcesses != 1 {
		t.Fatalf("Base plan must use one git cat-file --batch process, got %d", plan.baseGitBatchProcesses)
	}
}

func Test164Task1ARunnerRejectsDirectoryScope(t *testing.T) {
	r := exactEntrypointRunner164{dir: t.TempDir(), allowed: map[string]bool{"src/main/java/acme/A.java": true}}
	_, err := r.Run(context.Background(), "ast-grep.exe", "--lang", "java", "--pattern", "class $C { $$$BODY }", "src/main/java")
	if err == nil || !strings.Contains(err.Error(), "ENTRYPOINT_SCAN_SCOPE_WIDENED") {
		t.Fatalf("directory scope must fail closed, got %v", err)
	}
	if errors.Is(err, os.ErrNotExist) {
		t.Fatalf("runner reached process execution before rejecting widened scope: %v", err)
	}
}

func writeJava164(t *testing.T, root, p, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(p))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil { t.Fatal(err) }
}

func git164(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil { t.Fatalf("git %v: %v: %s", args, err, out) }
	return string(out)
}
