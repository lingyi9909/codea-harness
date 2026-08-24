package upgrade

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func parkRunningExecutableOutsideSource(sourceDir, running string) (string, error) {
	if running == "" {
		exe, err := os.Executable()
		if err != nil {
			return "", fmt.Errorf("resolve running executable for source cleanup: %w", err)
		}
		running = exe
	}
	inside, err := pathInside(sourceDir, running)
	if err != nil {
		return "", err
	}
	if !inside {
		return "", nil
	}

	parent := filepath.Dir(filepath.Clean(sourceDir))
	parking := filepath.Join(parent, fmt.Sprintf(".codea-upgrade-running-%d.exe", time.Now().UnixNano()))
	if err := os.Rename(running, parking); err != nil {
		return "", fmt.Errorf("move running upgrade executable outside source package: %w", err)
	}
	return parking, nil
}

func pathInside(root, candidate string) (bool, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false, fmt.Errorf("resolve source package path: %w", err)
	}
	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return false, fmt.Errorf("resolve running executable path: %w", err)
	}
	if strings.EqualFold(filepath.Clean(rootAbs), filepath.Clean(candidateAbs)) {
		return true, nil
	}
	rel, err := filepath.Rel(rootAbs, candidateAbs)
	if err != nil {
		return false, nil
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)), nil
}
