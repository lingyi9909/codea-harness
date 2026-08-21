package nav

import (
	"context"
	"strings"
	"testing"
)

type interfaceRunner struct{}

func (interfaceRunner) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	pattern := ""
	for i, arg := range args {
		if arg == "--pattern" && i+1 < len(args) {
			pattern = args[i+1]
		}
	}
	if strings.Contains(pattern, "OrderService") {
		return []byte(`{"file":"src/main/java/OrderService.java","range":{"start":{"line":0,"column":0},"end":{"line":4,"column":1}},"text":"public interface OrderService { }"}` + "\n"), nil
	}
	return nil, nil
}

func TestSymbolInfoSupportsInterface(t *testing.T) {
	n := Navigator{AstGrepPath: "ast-grep", Runner: interfaceRunner{}}
	got, err := n.GetSymbolInfo(context.Background(), "OrderService", "src/main/java")
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != "INTERFACE" || got.DeclaringType != "OrderService" {
		t.Fatalf("got=%+v", got)
	}
}
