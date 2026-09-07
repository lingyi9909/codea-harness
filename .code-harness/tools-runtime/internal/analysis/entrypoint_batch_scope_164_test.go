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
	if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, filepath.FromSlash(name)), link); err != nil {
		t.Skipf("symlink not available: %v", err)
	}
	runner := &exactBatchEntrypointRunner164{dir: root, allowed: pathSet164([]string{name})}
	_, err := runner.Run(context.Background(), "must-not-execute", "scan", "--inline-rules", "rule: {}", "--json=stream", "--color", "never", "--", name)
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
	if err == nil {
		t.Fatal("invalid scope must be rejected before launching the executable")
	}
	if !errors.Is(err, os.ErrNotExist) && !strings.Contains(err.Error(), "ENTRYPOINT_SCAN_SCOPE_WIDENED") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func Test164Task1BRunnerExactCommandContract(t *testing.T) {
	root := t.TempDir()
	a := "src/main/java/demo/A.java"
	b := "src/main/java/demo/B.java"
	extra := "src/main/java/demo/Extra.java"
	for _, p := range []string{a, b, extra} {
		writeJava164(t, root, p, "class A {}")
	}
	r := exactBatchEntrypointRunner164{dir: root, allowed: pathSet164([]string{a, b})}
	prefix := []string{"scan", "--inline-rules", "rule: {}", "--json=stream", "--color", "never", "--"}
	valid := append(append([]string{}, prefix...), a, b)
	if err := validateEntrypointBatchInvocation164(root, r.allowed, valid); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"extra-before-targets", append(append([]string{}, prefix...), extra, a, b)},
		{"extra-after-targets", append(append([]string{}, prefix...), a, b, extra)},
		{"duplicate", append(append([]string{}, prefix...), a, a)},
		{"directory", append(append([]string{}, prefix...), "src/main/java", b)},
		{"glob", append(append([]string{}, prefix...), "**/*.java", b)},
		{"missing", append(append([]string{}, prefix...), a, "missing.java")},
		{"extra-option", append([]string{"scan", "--inline-rules", "rule: {}", "--json=stream", "--color", "never", "--no-ignore", "--"}, a, b)},
		{"missing-delimiter", append([]string{"scan", "--inline-rules", "rule: {}", "--json=stream", "--color", "never"}, a, b)},
		{"absolute", append(append([]string{}, prefix...), filepath.Join(root, filepath.FromSlash(a)), b)},
		{"traversal", append(append([]string{}, prefix...), "../A.java", b)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := r.Run(context.Background(), "must-not-execute", tc.args...)
			if err == nil || !strings.Contains(err.Error(), "ENTRYPOINT_SCAN_SCOPE_WIDENED") {
				t.Fatalf("scope escape: %v", err)
			}
		})
	}
}

func Test164Task1BRunnerRejectsParentSymlink(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	name := "src/main/java/demo/A.java"
	writeJava164(t, outside, "demo/A.java", "class A {}")
	parent := filepath.Join(root, "src/main/java")
	if err := os.MkdirAll(filepath.Dir(parent), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, parent); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	r := exactBatchEntrypointRunner164{dir: root, allowed: pathSet164([]string{name})}
	args := []string{"scan", "--inline-rules", "rule: {}", "--json=stream", "--color", "never", "--", name}
	_, err := r.Run(context.Background(), "must-not-execute", args...)
	if err == nil || !strings.Contains(err.Error(), "ENTRYPOINT_SCAN_SCOPE_WIDENED") {
		t.Fatalf("parent symlink escaped: %v", err)
	}
}
