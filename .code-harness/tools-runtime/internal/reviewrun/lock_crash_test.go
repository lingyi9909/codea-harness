package reviewrun

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func Test180CrashedLockProcessAllowsSameRunFinishRetry(t *testing.T) {
	root := t.TempDir()
	started := mustPreparedRun(t, root)
	runDir := filepath.Dir(started.ReportPath)
	req := FinishRequest{RunID: started.RunID}

	// Simulate the pre-crash state that Task 1 must repair: result.json is
	// durable, but replacing review.md failed, so the run is still incomplete.
	originalReplace := replaceFile
	failed := false
	replaceFile = func(oldpath, newpath string) error {
		if !failed && filepath.Clean(newpath) == filepath.Clean(started.ReportPath) {
			failed = true
			return errors.New("injected report replace failure before crash")
		}
		return originalReplace(oldpath, newpath)
	}
	if _, err := Finish(context.Background(), root, req); err == nil {
		replaceFile = originalReplace
		t.Fatal("expected initial report replacement to fail")
	}
	replaceFile = originalReplace
	t.Cleanup(func() { replaceFile = originalReplace })
	if _, err := os.Stat(filepath.Join(runDir, "result.json")); err != nil {
		t.Fatalf("result must be durable before crash simulation: %v", err)
	}
	status, err := Status(root, started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if status.Execution != "INCOMPLETE" {
		t.Fatalf("pre-crash run must remain incomplete: %+v", status)
	}

	ready := filepath.Join(t.TempDir(), "lock-ready")
	cmd := exec.Command(os.Args[0], "-test.run=^Test180LockCrashHelperProcess$")
	cmd.Env = append(os.Environ(),
		"CODEA_REVIEW_LOCK_HELPER=1",
		"CODEA_REVIEW_LOCK_RUN_DIR="+runDir,
		"CODEA_REVIEW_LOCK_READY="+ready,
	)
	var childOutput bytes.Buffer
	cmd.Stdout = &childOutput
	cmd.Stderr = &childOutput
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	t.Cleanup(func() {
		if !waited && cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	waitForLockHelperReady(t, ready, &childOutput)

	// A live holder must still be protected; crash recovery must not mean
	// stealing a lock from a process that is still alive.
	busyCtx, busyCancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	_, err = Finish(busyCtx, root, req)
	busyCancel()
	if err == nil || !strings.Contains(err.Error(), "REVIEW_RUN_BUSY") {
		t.Fatalf("live child lock must block same-run finish, err=%v", err)
	}

	// Terminate the process without running its unlock callback. The OS must
	// release ownership so the exact same Finish request can repair review.md.
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	waited = true

	retryCtx, retryCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer retryCancel()
	got, err := Finish(retryCtx, root, req)
	if err != nil {
		t.Fatalf("same-content retry after lock-holder exit must repair report: %v\nchild output:\n%s", err, childOutput.String())
	}
	if got.Execution != "COMPLETE" {
		t.Fatalf("retry did not complete repaired report: %+v", got)
	}
	report, err := os.ReadFile(got.ReportPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(report, []byte("> 状态：评审完成")) {
		t.Fatalf("retry did not repair the completed report:\n%s", report)
	}
}

func Test180LiveLockBackgroundWaitIsBounded(t *testing.T) {
	runDir := t.TempDir()
	unlock, err := acquireRunLock(context.Background(), runDir)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()

	done := make(chan error, 1)
	go func() {
		secondUnlock, err := acquireRunLock(context.Background(), runDir)
		if err == nil {
			secondUnlock()
			done <- fmt.Errorf("unexpectedly acquired live lock")
			return
		}
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "REVIEW_RUN_BUSY") {
			t.Fatalf("background wait must end with bounded busy error, got %v", err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("background lock wait is unbounded")
	}
}

func Test180LockCrashHelperProcess(t *testing.T) {
	if os.Getenv("CODEA_REVIEW_LOCK_HELPER") != "1" {
		return
	}
	runDir := os.Getenv("CODEA_REVIEW_LOCK_RUN_DIR")
	ready := os.Getenv("CODEA_REVIEW_LOCK_READY")
	if runDir == "" || ready == "" {
		t.Fatal("lock helper environment missing")
	}
	unlock, err := acquireRunLock(context.Background(), runDir)
	if err != nil {
		t.Fatal(err)
	}
	_ = unlock // Intentionally never called: parent terminates this process.
	if err := os.WriteFile(ready, []byte("ready\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Second)
}

func waitForLockHelperReady(t *testing.T, ready string, output *bytes.Buffer) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(ready); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("lock helper did not become ready\n%s", output.String())
}
