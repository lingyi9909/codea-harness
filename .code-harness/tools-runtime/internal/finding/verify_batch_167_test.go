package finding

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/nav"
)

type verifyBatcher167 struct {
	root   string
	calls  map[string]int
	mutate string
}

func (b *verifyBatcher167) GetSymbolInfos(_ context.Context, symbols []string, scope string) (map[string]nav.SymbolInfo, error) {
	b.calls[scope]++
	if b.mutate == scope {
		_ = os.WriteFile(filepath.Join(b.root, filepath.FromSlash(scope)), []byte("changed"), 0o600)
	}
	out := map[string]nav.SymbolInfo{}
	for _, symbol := range symbols {
		out[symbol] = nav.SymbolInfo{Symbol: symbol, Path: scope, LineStart: 1, LineEnd: 2}
	}
	return out, nil
}

func Test167FindingSymbolAuthorityBatchesOncePerFile(t *testing.T) {
	root := t.TempDir()
	paths := []string{"src/A.java", "src/B.java"}
	for _, p := range paths {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("class X {}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	b := &verifyBatcher167{root: root, calls: map[string]int{}}
	locs := []analysis.SymbolLocation{{Workspace: "current", Path: paths[0], Symbol: "A.one"}, {Workspace: "current", Path: paths[0], Symbol: "A.two"}, {Workspace: "current", Path: paths[1], Symbol: "B.one"}}
	ranges, err := loadSymbolRanges167(root, locs, b)
	if err != nil {
		t.Fatal(err)
	}
	if b.calls[paths[0]] != 1 || b.calls[paths[1]] != 1 {
		t.Fatalf("calls=%v want one scan per file", b.calls)
	}
	if len(ranges) != 6 {
		t.Fatalf("ranges=%v; want exact and uniquely-authoritative bare keys", ranges)
	}
}

func Test167FindingSymbolAuthorityRejectsFileMutationDuringScan(t *testing.T) {
	root := t.TempDir()
	rel := "src/A.java"
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("class A {}"), 0o600); err != nil {
		t.Fatal(err)
	}
	b := &verifyBatcher167{root: root, calls: map[string]int{}, mutate: rel}
	_, err := loadSymbolRanges167(root, []analysis.SymbolLocation{{Workspace: "current", Path: rel, Symbol: "A.one"}}, b)
	if err == nil || !strings.Contains(err.Error(), "changed during pinned navigation") {
		t.Fatalf("mutation must fail closed, got %v", err)
	}
}

func Test167FindingRejectsEarlierFileChangedByLaterScan(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"A.java", "B.java"} {
		if e := os.WriteFile(filepath.Join(root, p), []byte("class X {}"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	locs := []analysis.SymbolLocation{{Path: "A.java", Symbol: "A.one"}, {Path: "B.java", Symbol: "B.one"}}
	_, err := loadSymbolRanges167(root, locs, &laterMutationBatcher167{root: root})
	if err == nil {
		t.Fatal("earlier file mutation was accepted")
	}
}

type laterMutationBatcher167 struct{ root string }

func (b *laterMutationBatcher167) GetSymbolInfos(ctx context.Context, symbols []string, scope string) (map[string]nav.SymbolInfo, error) {
	if scope == "B.java" {
		if e := os.WriteFile(filepath.Join(b.root, "A.java"), []byte("changed"), 0600); e != nil {
			return nil, e
		}
	}
	out := map[string]nav.SymbolInfo{}
	for _, s := range symbols {
		out[s] = nav.SymbolInfo{Symbol: s, Path: scope, LineStart: 1, LineEnd: 1}
	}
	return out, nil
}
