//go:build windows

package reviewauthority

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsExportRejectsUnknownWrapperWithoutFallback(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "unknown wrapper")
	writeNativeLauncherFixture(t, filepath.Join(binDir, "opencode.cmd"), "@echo off\r\necho compromised>injection-marker.txt\r\n")
	later := filepath.Join(root, "later native")
	copyExportTestExecutable(t, filepath.Join(later, "opencode.exe"))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+later)
	t.Setenv("CODEA_TEST_EXPORT_ARGV", "1")
	_, err := exportOpenCodeSession(root, "opaque & echo compromised>injection-marker.txt")
	if _, markerErr := os.Stat(filepath.Join(root, "injection-marker.txt")); !os.IsNotExist(markerErr) {
		t.Fatalf("unsupported wrapper created a side effect: %v", markerErr)
	}
	if err == nil || !strings.Contains(err.Error(), "unsupported OpenCode launcher") {
		t.Fatalf("expected selected wrapper to fail closed without falling through PATH, got %v", err)
	}
}

// CI installs the actual pinned npm package in an isolated prefix for this
// smoke test. The ordinary process tests need no OpenCode installation.
func TestWindowsPinnedOpenCodeExportSmoke(t *testing.T) {
	prefix := os.Getenv("CODEA_OPENCODE_INTEGRATION_PREFIX")
	if prefix == "" {
		t.Skip("requires CI's pinned opencode-ai@1.18.25 npm installation")
	}
	shim := filepath.Join(prefix, "opencode.cmd")
	if _, err := os.Stat(shim); err != nil {
		t.Fatalf("required installed npm shim unavailable: %v", err)
	}
	root := t.TempDir()
	t.Setenv("PATH", prefix+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("OPENCODE_PURE", "1")
	t.Setenv("OPENCODE_DISABLE_MODELS_FETCH", "true")
	for _, key := range []string{"XDG_DATA_HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME"} {
		t.Setenv(key, filepath.Join(root, key))
	}
	selected, err := exec.LookPath("opencode")
	if err != nil || !strings.EqualFold(selected, shim) {
		t.Fatalf("selected launcher = %q, %v; want installed npm shim %q", selected, err, shim)
	}
	t.Setenv("CODEA_TEST_OPAQUE_EXPANSION", "must-not-expand")
	id := `ses_ci_missing_opaque & echo compromised>injection-marker.txt | "quote" ^ %CODEA_TEST_OPAQUE_EXPANSION% !CODEA_TEST_OPAQUE_EXPANSION! 审查`
	_, err = exportOpenCodeSession(root, id)
	if _, markerErr := os.Stat(filepath.Join(root, "injection-marker.txt")); !os.IsNotExist(markerErr) {
		t.Fatalf("session ID created a shell side effect: %v", markerErr)
	}
	if err == nil || !strings.Contains(err.Error(), "Exporting session: "+id) {
		t.Fatalf("pinned Host must receive the exact requested ID and reject the missing session, got %v", err)
	}
}
