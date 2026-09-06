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
)

type workspaceASTRecordingRunner struct {
	calls [][]string
	out   []byte
}

func (r *workspaceASTRecordingRunner) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string(nil), args...))
	return r.out, nil
}

type workspaceASTFallbackRunner struct {
	sourceRoot string
	out        []byte
	calls      [][]string
}

func (r *workspaceASTFallbackRunner) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string(nil), args...))
	for _, arg := range args[5:] {
		if filepath.Clean(arg) == filepath.Clean(r.sourceRoot) {
			return r.out, nil
		}
	}
	return nil, nil
}

func Test163WorkspaceASTNarrowsConcreteClassCandidates(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "src", "main", "java")
	writeJava(t, root, "src/main/java/com/company/TargetService.java", `package com.company;
public class TargetService extends BaseService {
    public void target() {}
}`)
	for i := 0; i < 120; i++ {
		writeJava(t, root, fmt.Sprintf("src/main/java/com/company/noise/Noise%03d.java", i), fmt.Sprintf(`package com.company.noise;
public class Noise%03d { public void noop() {} }
`, i))
	}

	line := workspaceASTLine(t,
		filepath.Join(sourceRoot, "com", "company", "TargetService.java"),
		"public class TargetService extends BaseService { public void target() {} }",
		map[string]string{"SUPER": "BaseService"},
	)
	runner := &workspaceASTRecordingRunner{out: line}
	n := Navigator{RepoRoot: root, AstGrepPath: "ast-grep", Runner: runner}

	got, err := n.WorkspaceSuperclass(context.Background(), "TargetService")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "TargetService" || got.Super != "BaseService" {
		t.Fatalf("unexpected AST result: %#v", got)
	}
	if len(runner.calls) == 0 {
		t.Fatal("ast-grep was not invoked")
	}

	targetAbs := filepath.Clean(filepath.Join(sourceRoot, "com", "company", "TargetService.java"))
	for _, call := range runner.calls {
		if len(call) < 6 {
			t.Fatalf("unexpected ast-grep args: %v", call)
		}
		for _, arg := range call[5:] {
			if filepath.Clean(arg) == filepath.Clean(sourceRoot) {
				t.Fatalf("concrete lookup rescanned whole source root: %v", call)
			}
			if filepath.Clean(arg) != targetAbs {
				t.Fatalf("unrelated Java file leaked into concrete candidate set: %q in %v", arg, call)
			}
		}
	}
}

func Test163WorkspaceASTLargeJSONRecord(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "src", "main", "java")
	if err := os.MkdirAll(sourceRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	bigText := "public class BigService {" + strings.Repeat(" void filler(){}", 6000) + "}"
	if len(bigText) <= 64*1024 {
		t.Fatalf("fixture must exceed Scanner default token size, got %d bytes", len(bigText))
	}
	line := workspaceASTLine(t, filepath.Join(sourceRoot, "BigService.java"), bigText, nil)
	if len(line) <= 64*1024 {
		t.Fatalf("JSON record must exceed Scanner default token size, got %d bytes", len(line))
	}
	runner := &workspaceASTRecordingRunner{out: line}
	n := Navigator{RepoRoot: root, AstGrepPath: "ast-grep", Runner: runner}

	matches, err := n.runWorkspaceRaw(context.Background(), "class BigService { $$$BODY }")
	if err != nil {
		t.Fatalf("large ast-grep JSON record must parse completely: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one large-record match, got %d", len(matches))
	}
	if matches[0].Text != bigText {
		t.Fatalf("large record text was truncated: got=%d want=%d", len(matches[0].Text), len(bigText))
	}
}

func Test163WorkspaceASTCandidateTextIsNotSemanticAuthority(t *testing.T) {
	root := t.TempDir()
	writeJava(t, root, "src/main/java/com/company/LooksRelevant.java", `package com.company;
// TargetService extends BaseService appears only in a comment.
public class LooksRelevant {}
`)
	runner := &workspaceASTRecordingRunner{}
	n := Navigator{RepoRoot: root, AstGrepPath: "ast-grep", Runner: runner}

	_, err := n.WorkspaceSuperclass(context.Background(), "TargetService")
	if !errors.Is(err, ErrSymbolNotFound) {
		t.Fatalf("candidate text must not become semantic truth without AST confirmation: %v", err)
	}
}

func Test163WorkspaceASTPrefilterMissFallsBackToAST(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "src", "main", "java")
	writeJava(t, root, "src/main/java/com/company/LooksRelevant.java", `package com.company;
// TargetService appears here, but this is not its declaration.
public class LooksRelevant {}
`)
	writeJava(t, root, "src/main/java/com/company/Escaped.java", `package com.company;
public class \u0054argetService extends BaseService {}
`)
	line := workspaceASTLine(t,
		filepath.Join(sourceRoot, "com", "company", "Escaped.java"),
		"public class TargetService extends BaseService {}",
		map[string]string{"SUPER": "BaseService"},
	)
	runner := &workspaceASTFallbackRunner{sourceRoot: sourceRoot, out: line}
	n := Navigator{RepoRoot: root, AstGrepPath: "ast-grep", Runner: runner}

	got, err := n.WorkspaceSuperclass(context.Background(), "TargetService")
	if err != nil {
		t.Fatalf("candidate prefilter miss must fall back to AST authority: %v", err)
	}
	if got.Name != "TargetService" || got.Super != "BaseService" {
		t.Fatalf("unexpected fallback AST fact: %#v", got)
	}
	seenRoot := false
	for _, call := range runner.calls {
		for _, arg := range call[5:] {
			if filepath.Clean(arg) == filepath.Clean(sourceRoot) {
				seenRoot = true
			}
		}
	}
	if !seenRoot {
		t.Fatalf("prefilter miss never reached full AST fallback: %v", runner.calls)
	}
}

func Test163WorkspaceASTEmptySubclassCandidatesDoNotRescanRoot(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "src", "main", "java")
	writeJava(t, root, "src/main/java/com/company/Irrelevant.java", `package com.company;
public class Irrelevant extends BaseService {}
`)
	runner := &workspaceASTRecordingRunner{}
	n := Navigator{RepoRoot: root, AstGrepPath: "ast-grep", Runner: runner}

	got, err := n.WorkspaceDirectSubclassesWithMethod(context.Background(), "BaseService", "target", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no AST-confirmed subclasses, got %#v", got)
	}
	for _, call := range runner.calls {
		if len(call) < 6 || !strings.Contains(call[4], "target") {
			continue
		}
		for _, arg := range call[5:] {
			if filepath.Clean(arg) == filepath.Clean(sourceRoot) {
				t.Fatalf("zero AST-confirmed subclasses triggered a follow-up full-tree method scan: %v", call)
			}
		}
	}
}

func Test163WorkspaceASTLargeClassRealAstGrep(t *testing.T) {
	astPath := os.Getenv("CODEA_AST_GREP_TEST_PATH")
	if astPath == "" {
		t.Skip("real ast-grep path not configured; Task 1 Windows gate supplies it")
	}
	abs, err := filepath.Abs(astPath)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	var java strings.Builder
	java.WriteString("package com.company;\npublic class BigService {\n")
	for i := 0; i < 2200; i++ {
		fmt.Fprintf(&java, "    public void filler%04d() { int value = %d; }\n", i, i)
	}
	java.WriteString("    public void target() { int answer = 42; }\n}\n")
	if java.Len() <= 64*1024 {
		t.Fatalf("large class fixture unexpectedly small: %d bytes", java.Len())
	}
	writeJava(t, root, "src/main/java/com/company/BigService.java", java.String())
	for i := 0; i < 100; i++ {
		writeJava(t, root, fmt.Sprintf("src/main/java/com/company/noise/Noise%03d.java", i), fmt.Sprintf("package com.company.noise; public class Noise%03d {}\n", i))
	}

	n := Navigator{RepoRoot: root, AstGrepPath: abs}
	method, err := n.WorkspaceMethod(context.Background(), "BigService", "target")
	if err != nil {
		t.Fatalf("large real class method lookup failed: %v", err)
	}
	if method.Symbol != "BigService.target" || !strings.HasSuffix(filepath.ToSlash(method.Path), "/BigService.java") {
		t.Fatalf("unexpected large-class method result: %#v", method)
	}
}

func workspaceASTLine(t *testing.T, file, text string, meta map[string]string) []byte {
	t.Helper()
	var line workspaceSGLine
	line.File = file
	line.Text = text
	line.Range.Start.Line = 0
	line.Range.Start.Column = 0
	line.Range.End.Line = strings.Count(text, "\n")
	line.Range.End.Column = len(text)
	line.MetaVariables.Single = map[string]workspaceMetaValue{}
	for key, value := range meta {
		line.MetaVariables.Single[key] = workspaceMetaValue{Text: value}
	}
	encoded, err := json.Marshal(line)
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}
