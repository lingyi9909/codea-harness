package nav

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

type entrypointBatchRunner164 struct {
	calls [][]string
}

func (r *entrypointBatchRunner164) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string(nil), args...))
	switch len(r.calls) {
	case 1:
		return append(
			entrypointBatchJSON164("src/main/java/acme/AController.java", "@RestController\npublic class AController {\n  @GetMapping\n  public String get() { return \"ok\"; }\n}", 1, 20),
			entrypointBatchJSON164("src/main/java/acme/PlainService.java", "public class PlainService { public void work() {} }", 1, 10)...,
		), nil
	case 2:
		return entrypointBatchJSON164("src/main/java/acme/AController.java", "@GetMapping\npublic String get() { return \"ok\"; }", 5, 8), nil
	default:
		return nil, errors.New("unexpected ast-grep process amplification")
	}
}

func Test164EntrypointBatchTwoProcessesAndMethodNarrowsToControllers(t *testing.T) {
	runner := &entrypointBatchRunner164{}
	n := Navigator{RepoRoot: ".", AstGrepPath: "ast-grep", Runner: runner}
	targets := []string{
		"src/main/java/acme/AController.java",
		"src/main/java/acme/PlainService.java",
	}

	got, err := n.FindControllerEndpointsBatch(context.Background(), targets)
	if err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("astGrepProcessCount=%d want=2 calls=%v", len(runner.calls), runner.calls)
	}
	if len(got) != 1 || got[0].Symbol != "AController.get" || got[0].Path != targets[0] {
		t.Fatalf("unexpected endpoints: %+v", got)
	}

	firstTargets := batchCallTargets164(t, runner.calls[0])
	if !reflect.DeepEqual(firstTargets, targets) {
		t.Fatalf("type batch targets=%v want=%v", firstTargets, targets)
	}
	secondTargets := batchCallTargets164(t, runner.calls[1])
	if !reflect.DeepEqual(secondTargets, []string{targets[0]}) {
		t.Fatalf("method batch targets=%v want controller-only=%v", secondTargets, []string{targets[0]})
	}
}

func Test164EntrypointBatchRejectsBroadOrWildcardTarget(t *testing.T) {
	n := Navigator{RepoRoot: ".", AstGrepPath: "ast-grep", Runner: &entrypointBatchRunner164{}}
	for _, targets := range [][]string{{"."}, {"src/main/java"}, {"src/main/java/**/*.java"}} {
		if _, err := n.FindControllerEndpointsBatch(context.Background(), targets); !errors.Is(err, ErrInvalidScope) {
			t.Fatalf("targets=%v must fail closed with ErrInvalidScope, got %v", targets, err)
		}
	}
}

func Test164EntrypointBatchRealPinnedAstGrep(t *testing.T) {
	astPath := os.Getenv("CODEA_AST_GREP_TEST_PATH")
	if astPath == "" {
		t.Skip("real pinned ast-grep path not configured")
	}
	absAst, err := filepath.Abs(astPath)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	controller := "src/main/java/acme/AController.java"
	service := "src/main/java/acme/PlainService.java"
	writeEntrypointBatchJava164(t, root, controller, `package acme;
@RestController
public class AController {
    @GetMapping
    public String get() { return "ok"; }
}
`)
	writeEntrypointBatchJava164(t, root, service, `package acme;
public class PlainService { public void work() {} }
`)

	n := Navigator{RepoRoot: root, AstGrepPath: absAst, Runner: entrypointRootedRunner164{dir: root}}
	got, err := n.FindControllerEndpointsBatch(context.Background(), []string{controller, service})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Symbol != "AController.get" || got[0].Path != controller {
		t.Fatalf("real pinned ast-grep batch semantics drifted: %+v", got)
	}
}

type entrypointRootedRunner164 struct{ dir string }

func (r entrypointRootedRunner164) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = r.dir
	return cmd.Output()
}

func writeEntrypointBatchJava164(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func entrypointBatchJSON164(file, text string, startLine, endLine int) []byte {
	var line sgExtendedLine
	line.File = file
	line.Text = text
	line.Range.Start.Line = startLine - 1
	line.Range.Start.Column = 0
	line.Range.End.Line = endLine - 1
	line.Range.End.Column = 1
	data, _ := json.Marshal(line)
	return append(data, '\n')
}

func batchCallTargets164(t *testing.T, args []string) []string {
	t.Helper()
	marker := -1
	for i, arg := range args {
		if arg == "--json=stream" {
			marker = i
			break
		}
	}
	if marker < 0 || marker+1 >= len(args) {
		t.Fatalf("unexpected batch args: %v", args)
	}
	return append([]string(nil), args[marker+1:]...)
}
