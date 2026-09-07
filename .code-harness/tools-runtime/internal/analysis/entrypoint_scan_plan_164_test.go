package analysis

import (
	"context"
	"errors"
	"os"
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
	for _, key := range []string{"a", "b", "d"} {
		full := filepath.Join(root, filepath.FromSlash(paths[key]))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil { t.Fatal(err) }
		if err := os.WriteFile(full, []byte("class X {}\n"), 0o600); err != nil { t.Fatal(err) }
	}
	snapshot := changeset.Snapshot{
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
