package coverage_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/coverage"
	"codea-harness-tools/internal/nav"
)

type fakeAstGrep struct{}

func (fakeAstGrep) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "implements OrderService") {
		return []byte(`{"file":"src/main/java/OrderServiceImpl.java","text":"class OrderServiceImpl implements OrderService","range":{"start":{"line":0,"column":0}}}` + "\n"), nil
	}
	if strings.Contains(joined, "approve(") {
		return []byte(`{"file":"src/main/java/OrderController.java","text":"orderService.approve()","range":{"start":{"line":1,"column":2}}}` + "\n"), nil
	}
	return nil, nil
}
func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReviewGoldenUsesNavigationAndRealReadsBeforeCoverage(t *testing.T) {
	repo := t.TempDir()
	write(t, repo, "src/main/java/OrderController.java", "class OrderController { void approve(){ orderService.approve(); } }")
	write(t, repo, "src/main/java/OrderService.java", "interface OrderService { void approve(); }")
	write(t, repo, "src/main/java/OrderServiceImpl.java", "class OrderServiceImpl implements OrderService { public void approve(){} }")
	n := nav.Navigator{RepoRoot: repo, AstGrepPath: "fake", Runner: fakeAstGrep{}}
	impl, err := n.FindImplementations(context.Background(), "OrderService", "src/main/java")
	if err != nil {
		t.Fatal(err)
	}
	if len(impl.Matches) != 1 {
		t.Fatalf("nav did not find implementation: %+v", impl)
	}
	reviewed := []string{"src/main/java/OrderService.java", impl.Matches[0].Path}
	for _, rel := range reviewed {
		if _, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("read evidence %s: %v", rel, err)
		}
	}
	doc := map[string]any{
		"changedFiles": []any{map[string]any{"path": "src/main/java/OrderService.java"}},
		"reviewCoverage": map[string]any{
			"status":            "COMPLETE",
			"reviewedFiles":     []any{map[string]any{"path": reviewed[0]}, map[string]any{"path": reviewed[1]}},
			"unresolvedSymbols": []any{},
		},
	}
	b, _ := json.Marshal(doc)
	if _, err := coverage.VerifyAnalysisJSON(b); err != nil {
		t.Fatal(err)
	}
}
func TestMachineCoverageRejectsAgentCompleteWhenChangedFileMissing(t *testing.T) {
	b := []byte(`{"changedFiles":[{"path":"A.java"},{"path":"B.java"}],"reviewCoverage":{"status":"COMPLETE","reviewedFiles":[{"path":"A.java"}],"unresolvedSymbols":[]}}`)
	r, err := coverage.VerifyAnalysisJSON(b)
	if err == nil {
		t.Fatal("agent COMPLETE must not be trusted")
	}
	if r.Status != "PARTIAL" || len(r.MissingChangedFiles) != 1 {
		t.Fatalf("result=%+v", r)
	}
}
func TestMachineCoverageRejectsUnresolvedSymbols(t *testing.T) {
	b := []byte(`{"changedFiles":[{"path":"A.java"}],"reviewCoverage":{"status":"PARTIAL","reviewedFiles":[{"path":"A.java"}],"unresolvedSymbols":[{"symbol":"OrderService.approve"}]}}`)
	if _, err := coverage.VerifyAnalysisJSON(b); err == nil {
		t.Fatal("unresolved symbol must block complete review")
	}
}

func TestEvaluateRequiredScopeAllowsUnrelatedChangedFileOutsideTarget(t *testing.T) {
	r := coverage.EvaluateRequired(
		[]string{"src/main/java/OrderController.java", "src/main/java/OrderService.java"},
		[]string{"src/main/java/OrderController.java", "src/main/java/OrderService.java"},
		nil,
	)
	if r.Status != "COMPLETE" || len(r.MissingChangedFiles) != 0 {
		t.Fatalf("result=%+v", r)
	}
}

func TestEvaluateRequiredScopeRejectsMissingScopedFile(t *testing.T) {
	r := coverage.EvaluateRequired(
		[]string{"src/main/java/OrderController.java", "src/main/java/OrderService.java"},
		[]string{"src/main/java/OrderController.java"},
		nil,
	)
	if r.Status != "PARTIAL" || len(r.MissingChangedFiles) != 1 || r.MissingChangedFiles[0] != "src/main/java/OrderService.java" {
		t.Fatalf("result=%+v", r)
	}
}

func TestFullCoverageRejectsUnreadChangedMapperXml(t *testing.T) {
	b := []byte(`{"changedFiles":[{"path":"src/main/java/OrderMapper.java"},{"path":"src/main/resources/mapper/OrderMapper.xml"}],"reviewCoverage":{"status":"COMPLETE","reviewedFiles":[{"path":"src/main/java/OrderMapper.java"}],"unresolvedSymbols":[]}}`)
	r, err := coverage.VerifyAnalysisJSON(b)
	if err == nil {
		t.Fatal("changed Mapper.xml must not be silently skipped")
	}
	if r.Status != "PARTIAL" || len(r.MissingChangedFiles) != 1 || r.MissingChangedFiles[0] != "src/main/resources/mapper/OrderMapper.xml" {
		t.Fatalf("result=%+v", r)
	}
}

func TestFullCoverageRejectsUnreadChangedYaml(t *testing.T) {
	b := []byte(`{"changedFiles":[{"path":"src/main/resources/application.yml"}],"reviewCoverage":{"status":"COMPLETE","reviewedFiles":[],"unresolvedSymbols":[]}}`)
	r, err := coverage.VerifyAnalysisJSON(b)
	if err == nil {
		t.Fatal("changed yml must not be silently skipped")
	}
	if r.Status != "PARTIAL" || len(r.MissingChangedFiles) != 1 || r.MissingChangedFiles[0] != "src/main/resources/application.yml" {
		t.Fatalf("result=%+v", r)
	}
}

func TestFullCoverageAcceptsReviewedMapperAndYaml(t *testing.T) {
	b := []byte(`{"changedFiles":[{"path":"src/main/resources/mapper/OrderMapper.xml"},{"path":"src/main/resources/application.yml"}],"reviewCoverage":{"status":"COMPLETE","reviewedFiles":[{"path":"src/main/resources/mapper/OrderMapper.xml"},{"path":"src/main/resources/application.yml"}],"unresolvedSymbols":[]}}`)
	r, err := coverage.VerifyAnalysisJSON(b)
	if err != nil {
		t.Fatalf("reviewed resources should satisfy FULL coverage: %v", err)
	}
	if r.Status != "COMPLETE" {
		t.Fatalf("result=%+v", r)
	}
}
