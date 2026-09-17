//go:build windows

package reviewrun

import (
	"context"
	"syscall"
	"testing"
)

func Test180WindowsOccupiedReportPreventsFalseCompleteAndRetryRepairs(t *testing.T) {
	root := t.TempDir()
	started := mustPreparedRun(t, root)
	pathp, err := syscall.UTF16PtrFromString(started.ReportPath)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(pathp, syscall.GENERIC_READ, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	closed := false
	t.Cleanup(func() {
		if !closed {
			_ = syscall.CloseHandle(handle)
		}
	})

	req := FinishRequest{RunID: started.RunID}
	if _, err := Finish(context.Background(), root, req); err == nil {
		t.Fatal("expected occupied Windows report to block replacement")
	}
	status, err := Status(root, started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if status.Execution != "INCOMPLETE" {
		t.Fatalf("occupied report must not become complete: %+v", status)
	}
	if err := syscall.CloseHandle(handle); err != nil {
		t.Fatal(err)
	}
	closed = true
	got, err := Finish(context.Background(), root, req)
	if err != nil {
		t.Fatal(err)
	}
	if got.Execution != "COMPLETE" {
		t.Fatalf("retry after releasing handle did not complete: %+v", got)
	}
}
