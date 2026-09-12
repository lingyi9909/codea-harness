package chain

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/nav"
)

func Test170ProjectDiscoveryFailsClosedOnAmbiguousOverload(t *testing.T) {
	exe := os.Getenv("CODEA_AST_GREP")
	if exe == "" {
		t.Fatal("CODEA_AST_GREP must point to the approved ast-grep binary")
	}
	if _, err := os.Stat(exe); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	files := map[string]string{
		"src/main/java/demo/Example.java": `package demo;
class Service {
    void send(String value) {}
    void send(Long value) {}
}
class Example {
    Service service;
    void run() { service.send(null); }
}
`,
	}
	for path, content := range files {
		dst := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	navigator := nav.Navigator{RepoRoot: root, AstGrepPath: exe}
	paths, unresolved, err := projectWalkMethod163(context.Background(), root, navigator, "Example.run", map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 0 {
		t.Fatalf("ambiguous overload must not produce a guessed project path: %+v", paths)
	}
	if len(unresolved) != 1 || !strings.Contains(unresolved[0], "PROJECT_CALL_TARGET_UNRESOLVED: Example.run") {
		t.Fatalf("ambiguous overload must remain fail-closed: %+v", unresolved)
	}
}
