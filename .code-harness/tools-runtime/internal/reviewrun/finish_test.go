package reviewrun

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func Test180FinishWithoutScopeKeepsReport(t *testing.T) {
	root := t.TempDir()
	started, err := Start(root)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(started.ReportPath)
	if err != nil {
		t.Fatal(err)
	}

	got, err := Finish(context.Background(), root, FinishRequest{RunID: started.RunID})
	if err == nil {
		t.Fatalf("expected finish rejection without prepared scope, got %+v", got)
	}
	after, readErr := os.ReadFile(started.ReportPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("failed finish must preserve existing report\nbefore:\n%s\nafter:\n%s", before, after)
	}
	status, statusErr := Status(root, started.RunID)
	if statusErr != nil {
		t.Fatal(statusErr)
	}
	if status.Execution != "INCOMPLETE" {
		t.Fatalf("failed finish must remain INCOMPLETE: %+v", status)
	}
}

func Test180FinishPreparedStateWritesDurableReport(t *testing.T) {
	root := t.TempDir()
	started := mustPreparedRun(t, root)
	source := filepath.Join(root, "OrderService.java")
	content := []byte("class OrderService { void pay() {} }\n")
	if err := os.WriteFile(source, content, 0o600); err != nil {
		t.Fatal(err)
	}
	ref := ReadRef{Path: "OrderService.java", SHA256: bytesSHA256(content), StartLine: 1, EndLine: 1}
	introduced := true
	req := FinishRequest{RunID: started.RunID, Reads: []ReadRef{ref}, Findings: []Finding{{ID: "F1", Severity: "HIGH", Problem: "缺少幂等保护", Impact: "重复请求可能重复执行", Recommendation: "增加幂等键", Verification: "增加重复请求测试", Evidence: []Evidence{{Ref: ref, Quote: "void pay()"}}, IntroducedByChange: &introduced}}}

	got, err := Finish(context.Background(), root, req)
	if err != nil {
		t.Fatal(err)
	}
	if got.Execution != "COMPLETE" || got.ReviewConclusion != "BLOCKING" || got.Coverage != "COMPLETE" {
		t.Fatalf("unexpected outcome: %+v", got)
	}
	runDir := filepath.Dir(started.ReportPath)
	if _, err := os.Stat(filepath.Join(runDir, "result.json")); err != nil {
		t.Fatal(err)
	}
	report, err := os.ReadFile(started.ReportPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(report, []byte("缺少幂等保护")) || !bytes.Contains(report, []byte(`"execution":"COMPLETE"`)) {
		t.Fatalf("unexpected report:\n%s", report)
	}
	status, err := Status(root, started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if status.Execution != "COMPLETE" || status.ReportSHA256 != got.ReportSHA256 {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func Test180FinishSameContentRetryIsIdempotent(t *testing.T) {
	root := t.TempDir()
	started := mustPreparedRun(t, root)
	req := FinishRequest{RunID: started.RunID}
	first, err := Finish(context.Background(), root, req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Finish(context.Background(), root, req)
	if err != nil {
		t.Fatal(err)
	}
	if first.ReportSHA256 != second.ReportSHA256 || second.Execution != "COMPLETE" {
		t.Fatalf("retry changed completed result: first=%+v second=%+v", first, second)
	}
}

func Test180FinishDifferentContentRejectsOverwrite(t *testing.T) {
	root := t.TempDir()
	started := mustPreparedRun(t, root)
	first := FinishRequest{RunID: started.RunID}
	if _, err := Finish(context.Background(), root, first); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(started.ReportPath)
	if err != nil {
		t.Fatal(err)
	}
	second := FinishRequest{RunID: started.RunID, PendingRisks: []string{"new risk"}}
	if _, err := Finish(context.Background(), root, second); err == nil {
		t.Fatal("expected overwrite rejection")
	}
	after, err := os.ReadFile(started.ReportPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("overwrite rejection changed completed report")
	}
}

func Test180ResultSavedReportFailureSameContentRetryRepairs(t *testing.T) {
	root := t.TempDir()
	started := mustPreparedRun(t, root)
	originalReplace := replaceFile
	failed := false
	replaceFile = func(oldpath, newpath string) error {
		if !failed && filepath.Clean(newpath) == filepath.Clean(started.ReportPath) {
			failed = true
			return errors.New("injected report replace failure")
		}
		return originalReplace(oldpath, newpath)
	}
	t.Cleanup(func() { replaceFile = originalReplace })

	req := FinishRequest{RunID: started.RunID}
	if _, err := Finish(context.Background(), root, req); err == nil {
		t.Fatal("expected first finish to fail")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(started.ReportPath), "result.json")); err != nil {
		t.Fatal("result must already be durable before report replace", err)
	}
	status, err := Status(root, started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if status.Execution != "INCOMPLETE" {
		t.Fatalf("report failure must remain incomplete: %+v", status)
	}

	replaceFile = originalReplace
	got, err := Finish(context.Background(), root, req)
	if err != nil {
		t.Fatal(err)
	}
	if got.Execution != "COMPLETE" {
		t.Fatalf("retry did not repair report: %+v", got)
	}
}

func Test180TempFileFailureKeepsIncompleteReport(t *testing.T) {
	root := t.TempDir()
	started := mustPreparedRun(t, root)
	before, err := os.ReadFile(started.ReportPath)
	if err != nil {
		t.Fatal(err)
	}
	originalCreateTemp := createTemp
	createTemp = func(dir, pattern string) (*os.File, error) {
		return nil, errors.New("injected temp creation failure")
	}
	t.Cleanup(func() { createTemp = originalCreateTemp })
	if _, err := Finish(context.Background(), root, FinishRequest{RunID: started.RunID}); err == nil {
		t.Fatal("expected finish temp-file failure")
	}
	createTemp = originalCreateTemp
	after, err := os.ReadFile(started.ReportPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("temp-file failure changed report")
	}
	status, err := Status(root, started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if status.Execution != "INCOMPLETE" {
		t.Fatalf("temp-file failure must remain incomplete: %+v", status)
	}
}

func Test180ConcurrentFinishSameRunSerializes(t *testing.T) {
	root := t.TempDir()
	started := mustPreparedRun(t, root)
	req := FinishRequest{RunID: started.RunID}
	const count = 6
	var wg sync.WaitGroup
	outcomes := make(chan Outcome, count)
	errs := make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := Finish(context.Background(), root, req)
			if err != nil {
				errs <- err
				return
			}
			outcomes <- got
		}()
	}
	wg.Wait()
	close(outcomes)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	var hash string
	for got := range outcomes {
		if got.Execution != "COMPLETE" {
			t.Fatalf("unexpected outcome: %+v", got)
		}
		if hash == "" {
			hash = got.ReportSHA256
		} else if hash != got.ReportSHA256 {
			t.Fatalf("concurrent finishes produced different reports: %s vs %s", hash, got.ReportSHA256)
		}
	}
}

func Test180CancelRejectsFinish(t *testing.T) {
	root := t.TempDir()
	started := mustPreparedRun(t, root)
	cancelled, err := Cancel(root, started.RunID, "user cancelled")
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Execution != "CANCELLED" {
		t.Fatalf("unexpected cancel outcome: %+v", cancelled)
	}
	if _, err := Finish(context.Background(), root, FinishRequest{RunID: started.RunID}); err == nil {
		t.Fatal("cancelled run must reject finish")
	}
	status, err := Status(root, started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if status.Execution != "CANCELLED" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func Test180FinishRejectsStaleRead(t *testing.T) {
	root := t.TempDir()
	started := mustPreparedRun(t, root)
	path := filepath.Join(root, "A.java")
	first := []byte("class A {}\n")
	if err := os.WriteFile(path, first, 0o600); err != nil {
		t.Fatal(err)
	}
	ref := ReadRef{Path: "A.java", SHA256: bytesSHA256(first), StartLine: 1, EndLine: 1}
	if err := os.WriteFile(path, []byte("class A { int x; }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Finish(context.Background(), root, FinishRequest{RunID: started.RunID, Reads: []ReadRef{ref}}); err == nil {
		t.Fatal("expected stale read rejection")
	}
	status, err := Status(root, started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if status.Execution != "INCOMPLETE" {
		t.Fatalf("stale read must remain incomplete: %+v", status)
	}
}

func mustPreparedRun(t *testing.T, root string) Outcome {
	t.Helper()
	started, err := Start(root)
	if err != nil {
		t.Fatal(err)
	}
	runDir, state, err := loadRun(root, started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	state.ScopeReady = true
	state.Coverage = "COMPLETE"
	if err := writeState(runDir, state); err != nil {
		t.Fatal(err)
	}
	return started
}
