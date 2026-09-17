package reviewrun

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func Test180FinishPartialCoverageRemainsIncomplete(t *testing.T) {
	root := t.TempDir()
	started := mustPreparedRun(t, root)
	runDir, state, err := loadRun(root, started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	state.Coverage = "PARTIAL"
	if err := writeState(runDir, state); err != nil {
		t.Fatal(err)
	}

	got, err := Finish(context.Background(), root, FinishRequest{
		RunID: started.RunID,
		Gaps:  []string{"Mapper.xml 未读取"},
	})
	if err == nil {
		t.Fatalf("partial coverage must reject finish, got %+v", got)
	}
	status, statusErr := Status(root, started.RunID)
	if statusErr != nil {
		t.Fatal(statusErr)
	}
	if status.Execution != "INCOMPLETE" || status.Coverage != "PARTIAL" {
		t.Fatalf("partial coverage must remain incomplete: %+v", status)
	}
	report, readErr := os.ReadFile(status.ReportPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if bytes.Contains(report, []byte("> 状态：评审完成")) {
		t.Fatalf("partial coverage produced completed report:\n%s", report)
	}
	if _, statErr := os.Stat(filepath.Join(runDir, "result.json")); !os.IsNotExist(statErr) {
		t.Fatalf("partial coverage must not persist final result, stat err=%v", statErr)
	}
}

func Test180StatusRejectsMetadataOnlyCompletedReport(t *testing.T) {
	root := t.TempDir()
	started := mustPreparedRun(t, root)
	completed, err := Finish(context.Background(), root, FinishRequest{RunID: started.RunID})
	if err != nil {
		t.Fatal(err)
	}
	report, err := os.ReadFile(completed.ReportPath)
	if err != nil {
		t.Fatal(err)
	}
	idx := bytes.LastIndex(report, []byte(reportMetaPrefix))
	if idx < 0 {
		t.Fatal("completed report missing metadata")
	}
	metadataOnly := append([]byte("\n"), report[idx:]...)
	if err := os.WriteFile(completed.ReportPath, metadataOnly, 0o600); err != nil {
		t.Fatal(err)
	}

	status, err := Status(root, started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if status.Execution != "INCOMPLETE" {
		t.Fatalf("metadata-only report must not be complete: %+v\nreport:\n%s", status, metadataOnly)
	}
}

func Test180MovedProjectCancelUsesCurrentProjectReportPath(t *testing.T) {
	parent := t.TempDir()
	oldRoot := filepath.Join(parent, "project-old")
	newRoot := filepath.Join(parent, "project-new")
	if err := os.MkdirAll(oldRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	started, err := Start(oldRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(oldRoot, newRoot); err != nil {
		t.Fatal(err)
	}

	cancelled, err := Cancel(newRoot, started.RunID, "moved project")
	if err != nil {
		t.Fatal(err)
	}
	expectedReport := filepath.Join(newRoot, ".code-harness", "runs", started.RunID, "review.md")
	if filepath.Clean(cancelled.ReportPath) != filepath.Clean(expectedReport) {
		t.Fatalf("cancel wrote to stale report path: got %q want %q", cancelled.ReportPath, expectedReport)
	}
	if _, err := os.Stat(oldRoot); !os.IsNotExist(err) {
		t.Fatalf("cancel must not recreate old project path, stat err=%v", err)
	}
	report, err := os.ReadFile(expectedReport)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(report, []byte("评审已取消")) {
		t.Fatalf("current project report was not cancelled:\n%s", report)
	}
}

func Test180LiveOldLockIsNotStolenByAge(t *testing.T) {
	runDir := t.TempDir()
	unlock, err := acquireRunLock(context.Background(), runDir)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()

	lockPath := filepath.Join(runDir, ".review-180.lock")
	old := time.Now().Add(-3 * time.Minute)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	secondUnlock, err := acquireRunLock(ctx, runDir)
	if err == nil {
		secondUnlock()
		t.Fatal("second writer stole a live lock solely because its mtime was old")
	}
	if !strings.Contains(err.Error(), "REVIEW_RUN_BUSY") {
		t.Fatalf("unexpected lock error: %v", err)
	}
}

func Test180OldUnlockCannotReleaseNewOwner(t *testing.T) {
	runDir := t.TempDir()
	unlockOld, err := acquireRunLock(context.Background(), runDir)
	if err != nil {
		t.Fatal(err)
	}
	unlockOld()

	unlockNew, err := acquireRunLock(context.Background(), runDir)
	if err != nil {
		t.Fatal(err)
	}
	defer unlockNew()

	// A stale callback from the previous owner must be harmless.
	unlockOld()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	thirdUnlock, err := acquireRunLock(ctx, runDir)
	if err == nil {
		thirdUnlock()
		t.Fatal("stale unlock released the current owner's lock")
	}
	if !strings.Contains(err.Error(), "REVIEW_RUN_BUSY") {
		t.Fatalf("unexpected lock error: %v", err)
	}
}
