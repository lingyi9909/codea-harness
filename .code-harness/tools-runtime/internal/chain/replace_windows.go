//go:build windows

package chain

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func atomicReplace(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".chain-*.tmp")
	if err != nil {
		return fmt.Errorf("create chain temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write chain temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync chain temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close chain temp file: %w", err)
	}
	from, err := windows.UTF16PtrFromString(tmpPath)
	if err != nil {
		return fmt.Errorf("encode chain temp path: %w", err)
	}
	to, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("encode chain destination path: %w", err)
	}
	if err := windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH); err != nil {
		return fmt.Errorf("replace chain Project State: %w", err)
	}
	return nil
}
