package analysis

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test164Task1BRunnerRejectsWidenedTargets(t *testing.T) {
	root := t.TempDir()
	allowed := "src/main/java/demo/Controller.java"
	outside := "src/main/java/demo/Unrelated.java"
	writeJava164(t, root, allowed, "class Controller {}")
	writeJava164(t, root, outside, "class Unrelated {}")
	runner := &exactBatchEntrypointRunner164{dir: root, allowed: pathSet164([]string{allowed})}
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"directory", []string{"scan", "--json=stream", "src/main/java"}},
		{"wildcard", []string{"scan", "--json=stream", "src/main/java/**/*.java"}},
		{"extra-file", []string{"scan", "--json=stream", outside, allowed}},
		{"missing-file", []string{"scan", "--json=stream", allowed, "missing.java"}},
		{"missing-targets", []string{"scan", "--json=stream"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runner.Run(context.Background(), "must-not-execute", tc.args...)
			if err == nil || !strings.Contains(err.Error(), "ENTRYPOINT_SCAN_SCOPE_WIDENED") {
				t.Fatalf("expected scope-widened rejection, got %v", err)
			}
		})
	}
}

func Test164Task1BRunnerRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	name := "src/main/java/demo/Controller.java"
	writeJava164(t, outside, name, "class Controller {}")
	link := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil { t.Fatal(err) }
	if err := os.Symlink(filepath.Join(outside, filepath.FromSlash(name)), link); err != nil {
		t.Skipf("symlink not available: %v", err)
	}
	runner := &exactBatchEntrypointRunner164{dir: root, allowed: pathSet164([]string{name})}
	_, err := runner.Run(context.Background(), "must-not-execute", "scan", "--json=stream", name)
	if err == nil || !strings.Contains(err.Error(), "ENTRYPOINT_SCAN_SCOPE_WIDENED") {
		t.Fatalf("expected symlink escape rejection, got %v", err)
	}
}

func Test164Task1BRunnerDoesNotLaunchOnInvalidScope(t *testing.T) {
	root := t.TempDir()
	name := "src/main/java/demo/Controller.java"
	writeJava164(t, root, name, "class Controller {}")
	runner := &exactBatchEntrypointRunner164{dir: root, allowed: pathSet164([]string{name})}
	_, err := runner.Run(context.Background(), filepath.Join(root, "nonexistent-ast-grep"), "scan", "--json=stream", "src/main/java")
	if err == nil { t.Fatal("invalid scope must be rejected before launching the executable") }
	if !errors.Is(err, os.ErrNotExist) && !strings.Contains(err.Error(), "ENTRYPOINT_SCAN_SCOPE_WIDENED") {
		t.Fatalf("unexpected error: %v", err)
	}
}
