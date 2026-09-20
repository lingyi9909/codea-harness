//go:build !windows

package chain

import (
	"fmt"
	"os"
	"path/filepath"
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
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod chain temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close chain temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace chain Project State: %w", err)
	}
	return nil
}
