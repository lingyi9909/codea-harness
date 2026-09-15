package reviewauthority

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// nativeOpenCodeExecutable resolves only the selected PATH launcher. Published
// opencode-ai 1.18.25 installs a native bin/opencode.exe, exposed by npm through
// its non-shebang cmd-shim template. Recognize that template without executing
// or interpreting it; an arbitrary wrapper must never receive opaque IDs via
// cmd.exe or PowerShell. Other installation layouts need a native PATH entry.
func nativeOpenCodeExecutable(launcher string) (string, error) {
	if strings.EqualFold(filepath.Ext(launcher), ".exe") {
		return regularOpenCodeExecutable(launcher)
	}
	if !strings.EqualFold(filepath.Ext(launcher), ".cmd") {
		return "", fmt.Errorf("unsupported OpenCode launcher %q: a native opencode.exe or npm executable shim is required", launcher)
	}
	f, err := os.Open(launcher)
	if err != nil {
		return "", fmt.Errorf("read OpenCode launcher: %w", err)
	}
	defer f.Close()
	const maxShimBytes = 16 * 1024
	contents, err := io.ReadAll(io.LimitReader(f, maxShimBytes+1))
	if err != nil {
		return "", fmt.Errorf("read OpenCode launcher: %w", err)
	}
	if len(contents) <= maxShimBytes {
		const head = "@ECHO off GOTO start :find_dp0 SET dp0=%~dp0 EXIT /b :start SETLOCAL CALL :find_dp0 "
		// These are the global prefix and local node_modules/.bin npm layouts.
		for _, relative := range []string{`node_modules\opencode-ai\bin\opencode.exe`, `..\opencode-ai\bin\opencode.exe`} {
			expected := head + `"%dp0%\` + relative + `" %*`
			if strings.Join(strings.Fields(string(contents)), " ") != expected {
				continue
			}
			target := filepath.Join(filepath.Dir(launcher), filepath.FromSlash(strings.ReplaceAll(relative, `\`, "/")))
			return regularOpenCodeExecutable(target)
		}
	}
	return "", fmt.Errorf("unsupported OpenCode launcher %q: install the native OpenCode executable or its standard npm shim", launcher)
}

func regularOpenCodeExecutable(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("OpenCode native executable unavailable: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("OpenCode native executable %q is not a regular file", path)
	}
	// CreateProcess validates the executable format. Never fall back to a shell
	// if the npm postinstall stub or another non-native file is still present.
	return path, nil
}
