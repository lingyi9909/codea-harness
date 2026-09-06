package nav

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type task163RecordingRunner struct {
	targetFile string
	calls      [][]string
}

func (r *task163RecordingRunner) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	copyArgs := append([]string(nil), args...)
	r.calls = append(r.calls, copyArgs)
	pattern := task163Pattern(args)
	switch {
	case strings.Contains(pattern, "class OrderService"):
		return task163WorkspaceRecord(r.targetFile, "public class OrderService { public void createOrder() {} }", 0, 10, nil), nil
	case strings.Contains(pattern, " createOrder("):
		return task163WorkspaceRecord(r.targetFile, "public void createOrder() {}", 2, 4, nil), nil
	case strings.Contains(pattern, "class $C"):
		return task163WorkspaceRecord(r.targetFile, "public class OrderService { public void createOrder() {} }", 0, 10, map[string]string{"C": "OrderService"}), nil
	default:
		return nil, nil
	}
}

type task163FixedRunner struct {
	output []byte
}

func (r task163FixedRunner) Run(context.Context, string, ...string) ([]byte, error) {
	return r.output, nil
}

func Test163Task1LargeJSONRecordExceedsScannerDefault(t *testing.T) {
	root := t.TempDir()
	javaFile := filepath.Join(root, "src", "main", "java", "com", "acme", "BigService.java")
	if err := os.MkdirAll(filepath.Dir(javaFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(javaFile, []byte("class BigService {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	largeText := strings.Repeat("x", 96*1024)
	output := task163WorkspaceRecord(javaFile, largeText, 0, 1, nil)
	n := Navigator{RepoRoot: root, AstGrepPath: "ast-grep", Runner: task163FixedRunner{output: output}}
	matches, err := n.runWorkspaceRaw(context.Background(), "class BigService { $$$BODY }")
	if err != nil {
		t.Fatalf("workspace JSON stream record >64KB must parse without Scanner token failure: %v", err)
	}
	if len(matches) != 1 || len(matches[0].Text) != len(largeText) {
		t.Fatalf("large workspace match was not preserved: count=%d textLen=%d", len(matches), func() int {
			if len(matches) == 0 {
				return 0
			}
			return len(matches[0].Text)
		}())
	}
}

func Test163Task1WorkspaceMethodNarrowsASTToCandidateFiles(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "src", "main", "java", "com", "acme", "OrderService.java")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("package com.acme; public class OrderService { public void createOrder() {} }"), 0o644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 300; i++ {
		path := filepath.Join(root, "src", "main", "java", "com", "acme", "noise", fmt.Sprintf("Noise%03d.java", i))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(fmt.Sprintf("package com.acme.noise; class Noise%03d { void unrelated() {} }", i)), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	runner := &task163RecordingRunner{targetFile: target}
	n := Navigator{RepoRoot: root, AstGrepPath: "ast-grep", Runner: runner}
	method, err := n.WorkspaceMethod(context.Background(), "OrderService", "createOrder")
	if err != nil {
		t.Fatalf("WorkspaceMethod failed: %v", err)
	}
	if method.Symbol != "OrderService.createOrder" {
		t.Fatalf("unexpected method: %#v", method)
	}
	if len(runner.calls) == 0 {
		t.Fatal("expected AST verification calls")
	}

	sourceRoot := filepath.Clean(filepath.Join(root, "src", "main", "java"))
	for _, call := range runner.calls {
		targets := task163Targets(call)
		if len(targets) == 0 {
			t.Fatalf("AST call has no candidate target: %#v", call)
		}
		for _, got := range targets {
			if filepath.Clean(got) == sourceRoot {
				t.Fatalf("pattern still rescans full src/main/java instead of candidate files: %#v", call)
			}
		}
	}
}

func Test163Task1CandidateTextDoesNotBecomeSemanticAuthority(t *testing.T) {
	root := t.TempDir()
	commentOnly := filepath.Join(root, "src", "main", "java", "com", "acme", "CommentOnly.java")
	if err := os.MkdirAll(filepath.Dir(commentOnly), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(commentOnly, []byte("package com.acme; // class GhostService { void run() {} }\nclass CommentOnly {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	n := Navigator{RepoRoot: root, AstGrepPath: "ast-grep", Runner: task163FixedRunner{}}
	_, err := n.WorkspaceMethod(context.Background(), "GhostService", "run")
	if !errors.Is(err, ErrSymbolNotFound) {
		t.Fatalf("text candidate must not create a semantic fact without AST confirmation: %v", err)
	}
}

func Test163Task1RealLargeClassAndMethodLookup(t *testing.T) {
	astPath := os.Getenv("CODEA_AST_GREP_TEST_PATH")
	if astPath == "" {
		t.Skip("real ast-grep path not configured")
	}
	root := t.TempDir()
	var source strings.Builder
	source.WriteString("package com.acme;\npublic class BigService extends BigBase {\n")
	for i := 0; i < 2200; i++ {
		fmt.Fprintf(&source, "    private String field%04d = \"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\";\n", i)
	}
	source.WriteString("    public void createOrder() { field0001.length(); }\n}\n")
	writeJava(t, root, "src/main/java/com/acme/BigService.java", source.String())
	for i := 0; i < 200; i++ {
		writeJava(t, root, fmt.Sprintf("src/main/java/com/acme/noise/Noise%03d.java", i), fmt.Sprintf("package com.acme.noise; class Noise%03d { void unrelated() {} }", i))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	n := Navigator{RepoRoot: root, AstGrepPath: astPath}
	super, err := n.WorkspaceSuperclass(ctx, "BigService")
	if err != nil {
		t.Fatalf("large class lookup failed: %v", err)
	}
	if super.Name != "BigService" || super.Super != "BigBase" {
		t.Fatalf("unexpected large class match: %#v", super)
	}
	method, err := n.WorkspaceMethod(ctx, "BigService", "createOrder")
	if err != nil {
		t.Fatalf("large method lookup failed: %v", err)
	}
	if method.Symbol != "BigService.createOrder" {
		t.Fatalf("unexpected large method match: %#v", method)
	}
}

func task163WorkspaceRecord(file, text string, startLine, endLine int, meta map[string]string) []byte {
	line := workspaceSGLine{File: file, Text: text}
	line.Range.Start.Line = startLine
	line.Range.Start.Column = 0
	line.Range.End.Line = endLine
	line.Range.End.Column = 1
	line.MetaVariables.Single = map[string]workspaceMetaValue{}
	for key, value := range meta {
		line.MetaVariables.Single[key] = workspaceMetaValue{Text: value}
	}
	data, err := json.Marshal(line)
	if err != nil {
		panic(err)
	}
	return append(data, '\n')
}

func task163Pattern(args []string) string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--pattern" {
			return args[i+1]
		}
	}
	return ""
}

func task163Targets(args []string) []string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--pattern" {
			return append([]string(nil), args[i+2:]...)
		}
	}
	return nil
}
