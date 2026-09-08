package nav

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
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

func Test164EntrypointBatchRejectsProtocolDelimiterBeforeProcess(t *testing.T) {
	for _, tc := range []struct {
		name      string
		delimiter string
	}{
		{name: "NUL", delimiter: "\x00"},
		{name: "CR", delimiter: "\r"},
		{name: "LF", delimiter: "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &entrypointBatchRunner164{}
			n := Navigator{RepoRoot: ".", AstGrepPath: "ast-grep", Runner: runner}
			target := "src/main/java/acme/Bad" + tc.delimiter + "Injected.java"
			if _, err := n.FindControllerEndpointsBatch(context.Background(), []string{target}); !errors.Is(err, ErrInvalidScope) {
				t.Fatalf("batch protocol delimiter %s must fail closed with ErrInvalidScope, got %v", tc.name, err)
			}
			if len(runner.calls) != 0 {
				t.Fatalf("batch protocol delimiter %s launched %d ast-grep processes; want 0", tc.name, len(runner.calls))
			}
		})
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

func Test164EntrypointBatchSemanticParityMatrixRealPinnedAstGrep(t *testing.T) {
	astPath := os.Getenv("CODEA_AST_GREP_TEST_PATH")
	if astPath == "" {
		t.Skip("real pinned ast-grep path not configured")
	}
	absAst, err := filepath.Abs(astPath)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	files := map[string]string{
		"src/main/java/matrix/ControllerCase.java": `package matrix;
@Controller
public class ControllerCase {
    @RequestMapping
    public String request() { return "ok"; }
}
`,
		"src/main/java/matrix/RestControllerCase.java": `package matrix;
@RestController
public class RestControllerCase {
    @GetMapping
    public String get() { return "ok"; }
}
`,
		"src/main/java/matrix/FullyQualifiedCase.java": `package matrix;
@org.springframework.web.bind.annotation.RestController
public class FullyQualifiedCase {
    @org.springframework.web.bind.annotation.PostMapping
    public String post() { return "ok"; }
}
`,
		"src/main/java/matrix/AllMappingsCase.java": `package matrix;
@RestController
public class AllMappingsCase {
    @RequestMapping public String request() { return "ok"; }
    @GetMapping public String get() { return "ok"; }
    @PostMapping public String post() { return "ok"; }
    @PutMapping public String put() { return "ok"; }
    @PatchMapping public String patch() { return "ok"; }
    @DeleteMapping public String delete() { return "ok"; }
}
`,
		"src/main/java/matrix/MultiTypeCase.java": `package matrix;
@Controller
class FirstController {
    @GetMapping public String first() { return "ok"; }
}
class PlainType {
    @PostMapping public String ignored() { return "ignored"; }
}
`,
		"src/main/java/matrix/NestedClassCase.java": `package matrix;
@RestController
public class OuterController {
    @GetMapping public String outer() { return "ok"; }
    @Controller
    static class NestedController {
        @DeleteMapping public String nested() { return "ok"; }
    }
}
`,
		"src/main/java/matrix/AnnotationAddedCase.java": `package matrix;
@RestController
public class AnnotationAddedCase {
    @PatchMapping public String added() { return "ok"; }
}
`,
		"src/main/java/matrix/AnnotationRemovedCase.java": `package matrix;
public class AnnotationRemovedCase {
    @PutMapping public String removed() { return "not-an-entrypoint"; }
}
`,
	}
	targets := make([]string, 0, len(files))
	for p, content := range files {
		writeEntrypointBatchJava164(t, root, p, content)
		targets = append(targets, p)
	}
	sort.Strings(targets)

	runner := entrypointRootedRunner164{dir: root}
	n := Navigator{RepoRoot: root, AstGrepPath: absAst, Runner: runner}
	batch, err := n.FindControllerEndpointsBatch(context.Background(), targets)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := n.FindControllerEndpoints(context.Background(), "src/main/java/matrix")
	if err != nil {
		t.Fatal(err)
	}
	sortEntrypointMatches164(batch)
	sortEntrypointMatches164(legacy)
	if !reflect.DeepEqual(batch, legacy) {
		t.Fatalf("new batch scanner semantic drift vs legacy reference\nbatch=%+v\nlegacy=%+v", batch, legacy)
	}

	gotSymbols := make([]string, 0, len(batch))
	for _, match := range batch {
		gotSymbols = append(gotSymbols, match.Symbol)
	}
	sort.Strings(gotSymbols)
	wantSymbols := []string{
		"AllMappingsCase.delete",
		"AllMappingsCase.get",
		"AllMappingsCase.patch",
		"AllMappingsCase.post",
		"AllMappingsCase.put",
		"AllMappingsCase.request",
		"AnnotationAddedCase.added",
		"ControllerCase.request",
		"FirstController.first",
		"FullyQualifiedCase.post",
		"NestedController.nested",
		"OuterController.outer",
		"RestControllerCase.get",
	}
	sort.Strings(wantSymbols)
	if !reflect.DeepEqual(gotSymbols, wantSymbols) {
		t.Fatalf("semantic parity matrix coverage mismatch\ngot=%v\nwant=%v", gotSymbols, wantSymbols)
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

func sortEntrypointMatches164(matches []ControllerEndpointMatch) {
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Path != matches[j].Path {
			return matches[i].Path < matches[j].Path
		}
		if matches[i].Symbol != matches[j].Symbol {
			return matches[i].Symbol < matches[j].Symbol
		}
		if matches[i].StartLine != matches[j].StartLine {
			return matches[i].StartLine < matches[j].StartLine
		}
		return matches[i].StartColumn < matches[j].StartColumn
	})
}
