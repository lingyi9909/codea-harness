package nav

import (
	"os"
	"path/filepath"
	"testing"
)

func newReviewFixture170(t *testing.T, files map[string]string) Navigator {
	t.Helper()
	exe := os.Getenv("CODEA_AST_GREP")
	if exe == "" { t.Fatal("CODEA_AST_GREP must point to the approved ast-grep binary") }
	if _, err := os.Stat(exe); err != nil { t.Fatal(err) }
	root := t.TempDir()
	for path, content := range files {
		dst := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil { t.Fatal(err) }
		if err := os.WriteFile(dst, []byte(content), 0644); err != nil { t.Fatal(err) }
	}
	return Navigator{RepoRoot: root, AstGrepPath: exe}
}
