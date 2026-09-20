package reviewauthority

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// A copied test executable supplies a real native child process on Windows.
// This route exists only in the test binary, never in the production exporter.
func init() {
	if os.Getenv("CODEA_TEST_EXPORT_ARGV") != "1" || (filepath.Base(os.Args[0]) != "opencode.exe" && filepath.Base(os.Args[0]) != "opencode") {
		return
	}
	if err := json.NewEncoder(os.Stdout).Encode(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func copyExportTestExecutable(t *testing.T, target string) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, data, 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestExportOpenCodeSessionPreservesOpaqueArguments(t *testing.T) {
	layouts := []string{"native"}
	if runtime.GOOS == "windows" {
		layouts = append(layouts, "npm-global", "npm-local")
	}
	for _, layout := range layouts {
		t.Run(layout, func(t *testing.T) {
			testExportOpaqueArguments(t, layout)
		})
	}
}

func testExportOpaqueArguments(t *testing.T, layout string) {
	t.Helper()
	root := t.TempDir()
	binDir := filepath.Join(root, "native path & spaces")
	name := "opencode"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	target := filepath.Join(binDir, name)
	if layout != "native" {
		relative := `node_modules\opencode-ai\bin\opencode.exe`
		if layout == "npm-local" {
			binDir = filepath.Join(binDir, "node_modules", ".bin")
			relative = `..\opencode-ai\bin\opencode.exe`
		}
		writeNativeLauncherFixture(t, filepath.Join(binDir, "opencode.cmd"), npmNativeShimFixture(relative))
		target = filepath.Join(binDir, filepath.FromSlash(strings.ReplaceAll(relative, `\`, "/")))
	}
	copyExportTestExecutable(t, target)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CODEA_TEST_EXPORT_ARGV", "1")
	t.Setenv("CODEA_TEST_OPAQUE_EXPANSION", "must-not-expand")
	ids := []string{
		"opaque-session", "--leading-dash", "white space", "审查会话🙂",
		`quotes"and\backslashes\\`, `%CODEA_TEST_OPAQUE_EXPANSION% !CODEA_TEST_OPAQUE_EXPANSION! ^caret`,
		`opaque& echo compromised>injection-marker.txt`, `opaque| echo compromised>injection-marker.txt`,
		`opaque" & echo compromised>injection-marker.txt & rem "`,
	}
	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			out, err := exportOpenCodeSession(root, id)
			if _, markerErr := os.Stat(filepath.Join(root, "injection-marker.txt")); !os.IsNotExist(markerErr) {
				t.Fatalf("session ID created a shell side effect: %v", markerErr)
			}
			if err != nil {
				t.Fatal(err)
			}
			var args []string
			if err := json.Unmarshal(out, &args); err != nil {
				t.Fatalf("invalid child output %q: %v", out, err)
			}
			if want := []string{"export", "--sessionID=" + id}; !reflect.DeepEqual(args, want) {
				t.Fatalf("child argv = %#v, want %#v", args, want)
			}
		})
	}
}
