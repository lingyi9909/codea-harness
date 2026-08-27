package nav_test

import (
	"context"
	"strings"
	"testing"

	"codea-harness-tools/internal/nav"
)

type annotatedControllerRunner153 struct{}

func (annotatedControllerRunner153) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	var pattern string
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--pattern" {
			pattern = args[i+1]
			break
		}
	}

	// The legacy direct FindSymbol method patterns intentionally return no match.
	// The fallback must use the annotation-aware controller endpoint scanner.
	if strings.Contains(pattern, "$M") && strings.Contains(pattern, "@$_ANN") {
		return []byte(`{"file":"src/main/java/com/company/order/XxxController.java","range":{"start":{"line":4,"column":4},"end":{"line":5,"column":43}},"text":"@PostMapping\n    public void submit() { service.submit(); }"}` + "\n"), nil
	}
	if strings.Contains(pattern, "class $C") && strings.Contains(pattern, "@$_ANN") {
		return []byte(`{"file":"src/main/java/com/company/order/XxxController.java","range":{"start":{"line":1,"column":0},"end":{"line":6,"column":1}},"text":"@RestController\nclass XxxController {\n    private XxxService service;\n    @PostMapping\n    public void submit() { service.submit(); }\n}"}` + "\n"), nil
	}
	return nil, nil
}

func Test153FindSymbolFallsBackToAnnotatedControllerEndpoint(t *testing.T) {
	n := nav.Navigator{
		RepoRoot:    `C:\repo`,
		AstGrepPath: `C:\repo\.code-harness\bin\ast-grep.exe`,
		Runner:      annotatedControllerRunner153{},
	}
	got, err := n.FindSymbol(context.Background(), "XxxController.submit", "src/main/java")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Matches) != 1 {
		t.Fatalf("expected one annotated controller endpoint match, got %+v", got)
	}
	if got.Matches[0].Path != "src/main/java/com/company/order/XxxController.java" {
		t.Fatalf("unexpected path: %+v", got.Matches[0])
	}
}
