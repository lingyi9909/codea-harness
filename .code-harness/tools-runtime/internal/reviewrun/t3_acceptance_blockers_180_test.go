package reviewrun

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test180FinishRejectsFindingFromUnselectedSiblingControllerMethod(t *testing.T) {
	root, started, scope := prepareCreateScope180(t, "CURRENT_IMPLEMENTATION", false)
	cancel := exactLineRead180(t, root, "src/main/java/com/example/OrderController.java", "orderService.cancel();")
	assertFindingOutsideSelectedChain180(t, root, started, scope, cancel, "orderService.cancel();")
}

func Test180FinishRejectsFindingFromUnselectedSiblingServiceMethod(t *testing.T) {
	root, started, scope := prepareCreateScope180(t, "CURRENT_IMPLEMENTATION", false)
	cancel := exactLineRead180(t, root, "src/main/java/com/example/OrderServiceImpl.java", "orderMapper.cancelOrder();")
	assertFindingOutsideSelectedChain180(t, root, started, scope, cancel, "orderMapper.cancelOrder();")
}

func Test180FinishRejectsFindingFromUnselectedSiblingMapperStatement(t *testing.T) {
	root, started, scope := prepareCreateScope180(t, "CURRENT_IMPLEMENTATION", false)
	cancel := exactLineRead180(t, root, "src/main/resources/mapper/OrderMapper.xml", `id="cancelOrder"`)
	assertFindingOutsideSelectedChain180(t, root, started, scope, cancel, `id="cancelOrder"`)
}

func Test180CurrentImplementationRejectsIntroducedByChangeTrue(t *testing.T) {
	root, started, scope := prepareCreateScope180(t, "CURRENT_IMPLEMENTATION", false)
	ref := exactLineRead180(t, root, "src/main/java/com/example/OrderController.java", "orderService.create();")
	introduced := true
	finding := validFinding180("F-current", ref, "orderService.create();")
	finding.IntroducedByChange = &introduced
	reads := append(append([]ReadRef{}, scope.Reads...), ref)
	got, err := Finish(context.Background(), root, FinishRequest{RunID: started.RunID, Reads: reads, Findings: []Finding{finding}})
	if err == nil || !strings.Contains(err.Error(), "CHANGE_ATTRIBUTION") {
		t.Fatalf("CURRENT_IMPLEMENTATION accepted forged introducedByChange=true: got=%+v err=%v", got, err)
	}
}

func Test180ChangesRejectsIntroducedByChangeWithoutDiffEvidence(t *testing.T) {
	root, started, scope := prepareCreateScope180(t, "CHANGES", true)
	ref := exactLineRead180(t, root, "src/main/java/com/example/OrderServiceImpl.java", "orderMapper.insertOrder();")
	introduced := true
	finding := validFinding180("F-unchanged", ref, "orderMapper.insertOrder();")
	finding.IntroducedByChange = &introduced
	reads := append(append([]ReadRef{}, scope.Reads...), ref)
	got, err := Finish(context.Background(), root, FinishRequest{RunID: started.RunID, Reads: reads, Findings: []Finding{finding}})
	if err == nil || !strings.Contains(err.Error(), "CHANGE_ATTRIBUTION") {
		t.Fatalf("CHANGES accepted introducedByChange without changed-line evidence: got=%+v err=%v", got, err)
	}
}

func Test180ChangesAcceptsIntroducedByChangeWithChangedLineEvidence(t *testing.T) {
	root := copyControllerReviewFixture180(t)
	useRealAstGrep180(t, root)
	initControllerReviewGitBaseline180(t, root)
	controllerPath := "src/main/java/com/example/OrderController.java"
	abs := filepath.Join(root, filepath.FromSlash(controllerPath))
	data, err := os.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(data), "orderService.create();", "orderService.create(); // changed create path", 1)
	if changed == string(data) {
		t.Fatal("failed to seed changed create line")
	}
	if err := os.WriteFile(abs, []byte(changed), 0o600); err != nil {
		t.Fatal(err)
	}
	started, err := Start(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(context.Background(), root, started.RunID, Intent{Mode: "CHANGES", Target: "OrderController.create"}); err != nil {
		t.Fatal(err)
	}
	runDir, _, err := loadRun(root, started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := loadScope180(runDir, started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	ref := exactLineRead180(t, root, controllerPath, "changed create path")
	introduced := true
	finding := validFinding180("F-changed", ref, "changed create path")
	finding.IntroducedByChange = &introduced
	reads := append(append([]ReadRef{}, scope.Reads...), ref)
	got, err := Finish(context.Background(), root, FinishRequest{RunID: started.RunID, Reads: reads, Findings: []Finding{finding}})
	if err != nil {
		t.Fatalf("CHANGES rejected real changed-line attribution: %v", err)
	}
	if got.Execution != "COMPLETE" {
		t.Fatalf("changed-line attribution did not complete: %+v", got)
	}
}

func prepareCreateScope180(t *testing.T, mode string, seedChange bool) (string, Outcome, scopeState180) {
	t.Helper()
	root := copyControllerReviewFixture180(t)
	useRealAstGrep180(t, root)
	if mode == "CHANGES" {
		initControllerReviewGitBaseline180(t, root)
		if seedChange {
			path := filepath.Join(root, "src", "main", "java", "com", "example", "OrderController.java")
			f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.WriteString("\n// unrelated controller file change keeps create chain affected\n"); err != nil {
				_ = f.Close()
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
	started, err := Start(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(context.Background(), root, started.RunID, Intent{Mode: mode, Target: "OrderController.create"}); err != nil {
		t.Fatal(err)
	}
	runDir, _, err := loadRun(root, started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := loadScope180(runDir, started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	return root, started, scope
}

func assertFindingOutsideSelectedChain180(t *testing.T, root string, started Outcome, scope scopeState180, ref ReadRef, quote string) {
	t.Helper()
	finding := validFinding180("F-outside", ref, quote)
	reads := append(append([]ReadRef{}, scope.Reads...), ref)
	got, err := Finish(context.Background(), root, FinishRequest{RunID: started.RunID, Reads: reads, Findings: []Finding{finding}})
	if err == nil || !strings.Contains(err.Error(), "FINDING_OUTSIDE_SCOPE") {
		t.Fatalf("finish accepted finding evidence from readable but unselected sibling code: got=%+v err=%v", got, err)
	}
	status, statusErr := Status(root, started.RunID)
	if statusErr != nil {
		t.Fatal(statusErr)
	}
	if status.Execution != "INCOMPLETE" {
		t.Fatalf("out-of-chain finding completed report: %+v", status)
	}
}

func exactLineRead180(t *testing.T, root, rel, needle string) ReadRef {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	line := 0
	for i, raw := range strings.Split(string(data), "\n") {
		if strings.Contains(raw, needle) {
			if line != 0 {
				t.Fatalf("needle %q is ambiguous in %s", needle, rel)
			}
			line = i + 1
		}
	}
	if line == 0 {
		t.Fatalf("needle %q not found in %s", needle, rel)
	}
	return ReadRef{Path: rel, SHA256: bytesSHA256(data), StartLine: line, EndLine: line}
}

func validFinding180(id string, ref ReadRef, quote string) Finding {
	return Finding{
		ID:             id,
		Severity:       "HIGH",
		Problem:        "acceptance blocker fixture problem",
		Impact:         "acceptance blocker fixture impact",
		Recommendation: "fix the scoped problem",
		Verification:   "rerun the scoped regression",
		Evidence:       []Evidence{{Ref: ref, Quote: quote}},
	}
}
