package analysis

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"codea-harness-tools/internal/changeset"
)

type fakeEntrypointBaseSourceReader164 struct {
	sources map[string][]byte
}

func (f fakeEntrypointBaseSourceReader164) ReadBaseSources(_ context.Context, _ string, _ string, _ []string) (map[string][]byte, error) {
	out := make(map[string][]byte, len(f.sources))
	for path, data := range f.sources {
		out[path] = append([]byte(nil), data...)
	}
	return out, nil
}

func Test164EntrypointScanPlanSnapshotBoundedExactPaths(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{
		"src/main/java/acme/A.java",
		"src/main/java/acme/B.java",
		"src/main/java/acme/D.java",
		"src/main/java/acme/Unrelated.java",
	} {
		writeCurrentJava164(t, root, path)
	}

	snapshot := changeset.Snapshot{
		MergeBase:      strings.Repeat("1", 40),
		SnapshotSHA256: strings.Repeat("a", 64),
		SHA256:         strings.Repeat("a", 64),
		Files: []changeset.File{
			{Path: "src/main/java/acme/A.java", Status: "M"},
			{Path: "src/main/java/acme/B.java", Status: "M"},
			{Path: "src/main/java/acme/C.java", Status: "D"},
			{Path: "src/main/java/acme/D.java", Status: "A"},
			{Path: "src/test/java/acme/TestOnly.java", Status: "M"},
		},
	}
	reader := fakeEntrypointBaseSourceReader164{sources: map[string][]byte{
		"src/main/java/acme/A.java": []byte("class A {}\n"),
		"src/main/java/acme/B.java": []byte("class B {}\n"),
		"src/main/java/acme/C.java": []byte("class C {}\n"),
	}}

	plan, baseSources, err := buildEntrypointScanPlan164(context.Background(), root, "run164", snapshot, reader)
	if err != nil {
		t.Fatal(err)
	}
	wantCurrent := []string{
		"src/main/java/acme/A.java",
		"src/main/java/acme/B.java",
		"src/main/java/acme/D.java",
	}
	wantBase := []string{
		"src/main/java/acme/A.java",
		"src/main/java/acme/B.java",
		"src/main/java/acme/C.java",
	}
	if !reflect.DeepEqual(plan.CurrentPaths, wantCurrent) {
		t.Fatalf("CurrentPaths=%v want=%v", plan.CurrentPaths, wantCurrent)
	}
	if !reflect.DeepEqual(plan.BasePaths, wantBase) {
		t.Fatalf("BasePaths=%v want=%v", plan.BasePaths, wantBase)
	}
	if len(baseSources) != 3 {
		t.Fatalf("baseSources=%v want exact 3 snapshot-derived blobs", keys164(baseSources))
	}
	if plan.RunID != "run164" || plan.SnapshotSHA256 != snapshot.SnapshotSHA256 || plan.MergeBase != snapshot.MergeBase {
		t.Fatalf("plan identity=%+v", plan)
	}
	if len(plan.CurrentScopeSHA256) != 64 || len(plan.BaseScopeSHA256) != 64 || plan.CurrentScopeSHA256 == plan.BaseScopeSHA256 {
		t.Fatalf("scope hashes not bound independently: current=%q base=%q", plan.CurrentScopeSHA256, plan.BaseScopeSHA256)
	}
	for _, got := range append(append([]string(nil), plan.CurrentPaths...), plan.BasePaths...) {
		if got == "src/main/java/acme/Unrelated.java" {
			t.Fatalf("FULL/snapshot plan widened to unrelated repository source: %+v", plan)
		}
	}
}

func Test164EntrypointScopeRejectsWidenedRequest(t *testing.T) {
	plan := EntrypointScanPlan{
		CurrentPaths: []string{"src/main/java/acme/A.java", "src/main/java/acme/B.java"},
		BasePaths:    []string{"src/main/java/acme/A.java"},
	}
	for _, requested := range [][]string{
		{"src/main/java"},
		{"."},
		{"src/main/java/acme/A.java", "src/main/java/acme/B.java", "src/main/java/acme/Unrelated.java"},
	} {
		err := validateEntrypointScanPaths164(plan, entrypointScanSideCurrent164, requested)
		if err == nil || !strings.Contains(err.Error(), "ENTRYPOINT_SCAN_SCOPE_WIDENED") {
			t.Fatalf("requested=%v must fail closed with ENTRYPOINT_SCAN_SCOPE_WIDENED, got %v", requested, err)
		}
	}
}

func Test164EntrypointScopeRejectsBaseSourceOutsideSnapshotPlan(t *testing.T) {
	root := t.TempDir()
	writeCurrentJava164(t, root, "src/main/java/acme/A.java")
	snapshot := changeset.Snapshot{
		MergeBase:      strings.Repeat("1", 40),
		SnapshotSHA256: strings.Repeat("b", 64),
		Files:          []changeset.File{{Path: "src/main/java/acme/A.java", Status: "M"}},
	}
	reader := fakeEntrypointBaseSourceReader164{sources: map[string][]byte{
		"src/main/java/acme/A.java":         []byte("class A {}\n"),
		"src/main/java/acme/Unrelated.java": []byte("class Unrelated {}\n"),
	}}
	_, _, err := buildEntrypointScanPlan164(context.Background(), root, "run164", snapshot, reader)
	if err == nil || !strings.Contains(err.Error(), "ENTRYPOINT_BASE_SOURCE_OUT_OF_SCOPE") {
		t.Fatalf("unexpected base source must fail closed, got %v", err)
	}
}

func Test164EntrypointScopeRejectsScanResultOutsideRequestedFiles(t *testing.T) {
	requested := []string{"src/main/java/acme/A.java"}
	results := []ControllerEndpoint{{
		Controller: "UnrelatedController",
		Symbol:     "UnrelatedController.run",
		Path:       "src/main/java/acme/UnrelatedController.java",
	}}
	if err := validateEntrypointScanResults164(requested, results); err == nil || !strings.Contains(err.Error(), "ENTRYPOINT_SCAN_RESULT_OUT_OF_SCOPE") {
		t.Fatalf("out-of-scope AST result must fail closed, got %v", err)
	}
}

func writeCurrentJava164(t *testing.T, root, path string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("class Fixture {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func keys164(in map[string][]byte) []string {
	out := make([]string, 0, len(in))
	for key := range in {
		out = append(out, key)
	}
	return out
}
