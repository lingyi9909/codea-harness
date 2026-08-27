package chain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test153RealRegressionInventoryPassSentinelIsCaptureable(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", "..", ".."))
	childPath := filepath.Join(repoRoot, ".github", "scripts", "task153-task1-real-entrypoint-inventory.ps1")
	parentPath := filepath.Join(repoRoot, ".github", "scripts", "task153-real-review-chain-regression.ps1")

	childBytes, err := os.ReadFile(childPath)
	if err != nil {
		t.Fatal(err)
	}
	parentBytes, err := os.ReadFile(parentPath)
	if err != nil {
		t.Fatal(err)
	}
	child := string(childBytes)
	parent := string(parentBytes)
	const sentinel = "TASK153_TASK1_REAL_ENTRYPOINT_INVENTORY PASS"

	childEmitsSuccessStream := strings.Contains(child, "Write-Output '"+sentinel+"'")
	parentCapturesInformation := strings.Contains(parent, "6>&1") || strings.Contains(parent, "*>&1")
	if !childEmitsSuccessStream && !parentCapturesInformation {
		t.Fatalf("%s must be machine-captureable: use Write-Output for the sentinel or capture the Information stream", sentinel)
	}
}
