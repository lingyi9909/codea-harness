package nav

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type workspaceScopeRunner163 struct {
	file  string
	calls [][]string
}

func (r *workspaceScopeRunner163) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	copied := append([]string(nil), args...)
	r.calls = append(r.calls, copied)
	pattern := ""
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--pattern" {
			pattern = args[i+1]
			break
		}
	}

	line := workspaceSGLine{File: r.file, Text: "match"}
	line.Range.Start.Line = 0
	line.Range.Start.Column = 0
	line.Range.End.Line = 2100
	line.Range.End.Column = 1
	line.MetaVariables.Single = map[string]workspaceMetaValue{}

	switch {
	case strings.Contains(pattern, "class BigService"):
		line.MetaVariables.Single["SUPER"] = workspaceMetaValue{Text: "BaseService"}
	case strings.Contains(pattern, "class $C"):
		line.MetaVariables.Single["C"] = workspaceMetaValue{Text: "BigService"}
		line.MetaVariables.Single["SUPER"] = workspaceMetaValue{Text: "BaseService"}
	case strings.Contains(pattern, " run(") || strings.HasPrefix(pattern, "run("):
		line.Range.Start.Line = 2050
		line.Range.End.Line = 2052
	default:
		return nil, nil
	}
	b, err := json.Marshal(line)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func Test163WorkspaceMethodNarrowsAstGrepToCandidateJavaFile(t *testing.T) {
	root := t.TempDir()
	rel := filepath.FromSlash("src/main/java/com/example/BigService.java")
	file := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	var source strings.Builder
	source.WriteString("package com.example;\npublic class BigService extends BaseService {\n")
	for i := 0; i < 2040; i++ {
		source.WriteString("  // filler line\n")
	}
	source.WriteString("  public void run() {}\n}\n")
	if err := os.WriteFile(file, []byte(source.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 25; i++ {
		decoy := filepath.Join(root, "src", "main", "java", "com", "example", "decoy", "Decoy"+string(rune('A'+i))+".java")
		if err := os.MkdirAll(filepath.Dir(decoy), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(decoy, []byte("package com.example.decoy; class Decoy {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	runner := &workspaceScopeRunner163{file: file}
	n := Navigator{RepoRoot: root, AstGrepPath: "ast-grep", Runner: runner}
	if _, err := n.WorkspaceMethod(context.Background(), "BigService", "run"); err != nil {
		t.Fatalf("WorkspaceMethod failed: %v", err)
	}
	if len(runner.calls) == 0 {
		t.Fatal("expected ast-grep calls")
	}
	sourceRoot := filepath.Clean(filepath.Join(root, "src", "main", "java"))
	for _, call := range runner.calls {
		if len(call) == 0 {
			continue
		}
		target := filepath.Clean(call[len(call)-1])
		if target == sourceRoot {
			t.Fatalf("large-class navigation must not rescan full src/main/java per pattern; call=%v", call)
		}
		if target != filepath.Clean(file) {
			t.Fatalf("expected candidate-file AST target %s, got %s; call=%v", file, target, call)
		}
	}
}
