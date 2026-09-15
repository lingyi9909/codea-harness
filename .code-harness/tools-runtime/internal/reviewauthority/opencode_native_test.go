package reviewauthority

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// npm/cmd-shim's non-shebang template, used by the published opencode-ai
// 1.18.25 bin/opencode.exe (both its preinstall stub and installed native PE).
func npmNativeShimFixture(relativeTarget string) string {
	return "@ECHO off\r\nGOTO start\r\n:find_dp0\r\nSET dp0=%~dp0\r\nEXIT /b\r\n:start\r\nSETLOCAL\r\nCALL :find_dp0\r\n\"%dp0%\\" + relativeTarget + "\"   %*\r\n"
}

func writeNativeLauncherFixture(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestNativeOpenCodeExecutableResolvesSelectedLauncher(t *testing.T) {
	for _, relative := range []string{`node_modules\opencode-ai\bin\opencode.exe`, `..\opencode-ai\bin\opencode.exe`} {
		t.Run(relative, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "npm prefix & spaces", "bin")
			launcher := filepath.Join(dir, "opencode.cmd")
			target := filepath.Clean(filepath.Join(dir, filepath.FromSlash(strings.ReplaceAll(relative, `\`, "/"))))
			writeNativeLauncherFixture(t, launcher, npmNativeShimFixture(relative))
			writeNativeLauncherFixture(t, target, "native executable placeholder")
			got, err := nativeOpenCodeExecutable(launcher)
			if err != nil || got != target {
				t.Fatalf("resolved executable = %q, %v; want %q", got, err, target)
			}
		})
	}
	t.Run("native", func(t *testing.T) {
		target := filepath.Join(t.TempDir(), "opencode.EXE")
		writeNativeLauncherFixture(t, target, "native executable placeholder")
		got, err := nativeOpenCodeExecutable(target)
		if err != nil || got != target {
			t.Fatalf("resolved executable = %q, %v; want %q", got, err, target)
		}
	})
}

func TestNativeOpenCodeExecutableRejectsUnsupportedLaunchers(t *testing.T) {
	for _, tc := range []struct {
		name, extension, script string
		missingTarget           bool
	}{
		{"unknown cmd", ".cmd", "@echo off\r\nopencode.exe %*\r\n", false},
		{"bat wrapper", ".bat", npmNativeShimFixture(`node_modules\opencode-ai\bin\opencode.exe`), false},
		{"powershell wrapper", ".ps1", "& opencode.exe @args", false},
		{"modified npm shim", ".cmd", npmNativeShimFixture(`node_modules\opencode-ai\bin\opencode.exe`) + "echo side-effect>marker\r\n", false},
		{"different npm package", ".cmd", npmNativeShimFixture(`node_modules\other\bin\opencode.exe`), false},
		{"node npm launcher", ".cmd", "@echo off\r\nnode \"%dp0%\\node_modules\\opencode-ai\\bin\\opencode\" %*\r\n", false},
		{"missing native target", ".cmd", npmNativeShimFixture(`node_modules\opencode-ai\bin\opencode.exe`), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			launcher := filepath.Join(dir, "opencode"+tc.extension)
			writeNativeLauncherFixture(t, launcher, tc.script)
			if !tc.missingTarget {
				writeNativeLauncherFixture(t, filepath.Join(dir, "node_modules", "opencode-ai", "bin", "opencode.exe"), "native executable placeholder")
			}
			if got, err := nativeOpenCodeExecutable(launcher); err == nil {
				t.Fatalf("unsupported launcher resolved to %q", got)
			}
		})
	}
}
