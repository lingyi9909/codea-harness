package nav

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type batchRunner167 struct {
	calls int
	out   []byte
	err   error
}

func (r *batchRunner167) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	r.calls++
	if len(args) < 4 || args[0] != "scan" || args[1] != "--inline-rules" || args[3] != "--json=stream" {
		return nil, errors.New("navigation did not use an inline AST batch")
	}
	return r.out, r.err
}

func Test167GetSymbolInfosUsesOneDeclarationProcessForSameFile(t *testing.T) {
	r := &batchRunner167{out: append(
		batchJSON167("src/main/java/acme/OrderController.java", "@RestController\npublic class OrderController {\n @GetMapping\n public String get() { return \"ok\"; }\n private static final int LIMIT = 5;\n}", 1, 6),
		append(batchJSON167("src/main/java/acme/OrderController.java", "@GetMapping\npublic String get() { return \"ok\"; }", 3, 4), batchJSON167("src/main/java/acme/OrderController.java", "private static final int LIMIT = 5;", 5, 5)...)...,
	)}
	n := Navigator{AstGrepPath: "ast-grep", Runner: r}
	symbols := []string{"OrderController", "OrderController.get", "OrderController.LIMIT"}
	for i := 0; i < 200; i++ {
		symbols = append(symbols, "OrderController.get")
	}
	got, err := n.GetSymbolInfos(context.Background(), symbols, "src/main/java/acme/OrderController.java")
	if err != nil {
		t.Fatal(err)
	}
	if r.calls != 1 {
		t.Fatalf("astGrepProcessCount=%d want=1", r.calls)
	}
	if got["OrderController.get"].Signature != "get()" || got["OrderController.LIMIT"].Kind != "FIELD" || got["OrderController"].Kind != "CLASS" {
		t.Fatalf("unexpected batch symbol info: %+v", got)
	}
}

func Test167FindByAnnotationUsesOneProcess(t *testing.T) {
	r := &batchRunner167{out: append(
		batchJSON167("src/main/java/acme/Outer.java", "class Outer { @PostMapping public void save() {} }", 1, 5),
		batchJSON167("src/main/java/acme/Outer.java", "@PostMapping\npublic void save() {}", 2, 3)...,
	)}
	n := Navigator{AstGrepPath: "ast-grep", Runner: r}
	got, err := n.FindByAnnotation(context.Background(), "PostMapping", "src/main/java")
	if err != nil {
		t.Fatal(err)
	}
	if r.calls != 1 {
		t.Fatalf("astGrepProcessCount=%d want=1", r.calls)
	}
	if len(got.Matches) != 1 || got.Matches[0].Symbol != "Outer.save" {
		t.Fatalf("matches=%+v", got.Matches)
	}
}

func Test167BatchedNavigationFailsClosed(t *testing.T) {
	tests := []struct {
		name string
		out  []byte
		err  error
	}{
		{name: "malformed", out: []byte("not-json\n")},
		{name: "exit", err: &exec.ExitError{}},
		{name: "cancel", err: context.Canceled},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &batchRunner167{out: tc.out, err: tc.err}
			n := Navigator{AstGrepPath: "ast-grep", Runner: r}
			if _, err := n.GetSymbolInfo(context.Background(), "OrderController", "src/main/java"); err == nil {
				t.Fatal("process failure or malformed output was treated as an empty successful scan")
			}
		})
	}
}

func Test167BatchedNavigationRealPinnedAstGrepParity(t *testing.T) {
	ast := os.Getenv("CODEA_AST_GREP_TEST_PATH")
	if ast == "" {
		t.Skip("real pinned ast-grep path not configured")
	}
	root := t.TempDir()
	rel := "src/main/java/acme/Outer.java"
	source := `package acme;
public class Outer {
  private static final int LIMIT = 5;
  private Object service = new Object();
  private Object other = factory();
  private Runnable action = () -> {};
  public static class Nested {
    @PostMapping("/one") public String run(String value) { return value; }
    public String run(int value) { return "x"; }
  }
}`
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	n := Navigator{RepoRoot: root, AstGrepPath: ast, Runner: entrypointRootedRunner164{dir: root}}
	info, err := n.GetSymbolInfo(context.Background(), "Nested.run", rel)
	if !errors.Is(err, ErrAmbiguousSymbol) {
		t.Fatalf("overload ambiguity changed: info=%+v err=%v", info, err)
	}
	field, err := n.GetSymbolInfo(context.Background(), "Outer.LIMIT", rel)
	if err != nil || field.Kind != "FIELD" || field.ReturnType != "int" {
		t.Fatalf("field=%+v err=%v", field, err)
	}
	for _, name := range []string{"service", "other", "action"} {
		got, e := n.GetSymbolInfo(context.Background(), "Outer."+name, rel)
		if e != nil || got.Kind != "FIELD" {
			t.Fatalf("initialized field %s: %+v %v", name, got, e)
		}
	}
	anns, err := n.FindByAnnotation(context.Background(), "PostMapping", rel)
	if err != nil || len(anns.Matches) != 1 || anns.Matches[0].Symbol != "Nested.run" {
		t.Fatalf("annotations=%+v err=%v", anns, err)
	}
}

func batchJSON167(file, source string, start, end int) []byte {
	q := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "\n", "\\n", "\r", "\\r", "\t", "\\t").Replace(source)
	return []byte(`{"file":"` + file + `","text":"` + q + `","range":{"start":{"line":` + itoa167(start-1) + `,"column":0},"end":{"line":` + itoa167(end-1) + `,"column":0}}}` + "\n")
}

func itoa167(v int) string {
	if v == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}

func Test167BatchRejectsMissingAndReversedRanges(t *testing.T) {
	for _, data := range []string{
		`{"file":"src/A.java","text":"class A {}"}`,
		`{"file":"src/A.java","text":"class A {}","range":{}}`,
		`{"file":"src/A.java","text":"class A {}","range":{"start":{"line":0,"column":4},"end":{"line":0,"column":2}}}`,
		`{"file":"src/A.java","text":"class A {}","range":{"start":{"line":2,"column":0},"end":{"line":0,"column":2}}}`,
	} {
		n := Navigator{AstGrepPath: "ast-grep", Runner: &batchRunner167{out: []byte(data + "\n")}}
		if _, err := n.GetSymbolInfo(context.Background(), "A", "src/A.java"); err == nil {
			t.Fatalf("accepted malformed range: %s", data)
		}
	}
}

func Test167BatchRejectsResultsOutsideExactScope(t *testing.T) {
	n := Navigator{AstGrepPath: "ast-grep", Runner: &batchRunner167{out: batchJSON167("src/B.java", "class A {}", 1, 1)}}
	if _, err := n.GetSymbolInfo(context.Background(), "A", "src/A.java"); err == nil {
		t.Fatal("out-of-scope result accepted")
	}
}
