//go:build windows

package reviewrun

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

const lockAllBytes = ^uint32(0)

func tryRunFileLock(file *os.File) (bool, error) {
	var overlapped windows.Overlapped
	err := windows.LockFileEx(
		windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		lockAllBytes,
		lockAllBytes,
		&overlapped,
	)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return false, nil
	}
	return false, err
}

func unlockRunFile(file *os.File) error {
	var overlapped windows.Overlapped
	return windows.UnlockFileEx(
		windows.Handle(file.Fd()),
		0,
		lockAllBytes,
		lockAllBytes,
		&overlapped,
	)
}
