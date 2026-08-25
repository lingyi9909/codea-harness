package upgrade

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRelease151HistoricalChainDiscoverBootstrapContract(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", "..", ".."))

	mustRead := func(path string) string {
		t.Helper()
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	changelog := mustRead(filepath.Join(repoRoot, "CHANGELOG.md"))
	for _, want := range []string{
		"## 1.5.1 - 2026-08-24",
		"Chain Discover Bootstrap Fix",
		"无需先执行 harness review",
		"1.5.0 → 1.5.1",
	} {
		if !strings.Contains(changelog, want) {
			t.Fatalf("CHANGELOG missing historical 1.5.1 contract %q", want)
		}
	}
}
