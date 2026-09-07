package analysis

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// validateEntrypointBatchInvocation164 is the second, execution-boundary
// authority check. Only the fixed multi-rule command and the complete exact
// allowlist are accepted. No implicit scope, extra positional target, glob,
// directory, symlink or junction is allowed to reach the process.
func validateEntrypointBatchInvocation164(root string, allowed map[string]bool, args []string) error {
	if len(allowed) == 0 || len(args) != 7+len(allowed) ||
		args[0] != "scan" || args[1] != "--inline-rules" || strings.TrimSpace(args[2]) == "" ||
		args[3] != "--json=stream" || args[4] != "--color" || args[5] != "never" || args[6] != "--" {
		return errors.New("ENTRYPOINT_SCAN_SCOPE_WIDENED: invalid exact batch invocation")
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: root: %w", err)
	}
	realRoot, err = filepath.Abs(realRoot)
	if err != nil {
		return fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: root: %w", err)
	}
	seen := make(map[string]bool, len(allowed))
	for _, raw := range args[7:] {
		clean, ok := exactProductionJavaPath164(raw)
		if !ok || clean != raw || !allowed[raw] || seen[raw] {
			return fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: target=%q", raw)
		}
		if err := validateEntrypointBatchFile164(realRoot, raw); err != nil {
			return err
		}
		seen[raw] = true
	}
	for p, enabled := range allowed {
		clean, ok := exactProductionJavaPath164(p)
		if !enabled || !ok || clean != p || !seen[p] {
			return fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: allowlist=%q", p)
		}
	}
	return nil
}

func validateEntrypointBatchFile164(root, p string) error {
	current := root
	parts := strings.Split(p, "/")
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: target=%q: %w", p, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: symlink target=%q", p)
		}
		if i < len(parts)-1 && !info.IsDir() {
			return fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: non-directory component=%q", p)
		}
		if i == len(parts)-1 && !info.Mode().IsRegular() {
			return fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: non-regular target=%q", p)
		}
	}
	resolved, err := filepath.EvalSymlinks(current)
	if err != nil {
		return fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: target=%q: %w", p, err)
	}
	expected := filepath.Join(root, filepath.FromSlash(p))
	if !sameEntrypointPath164(resolved, expected) {
		return fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: resolved target=%q", p)
	}
	return nil
}

func sameEntrypointPath164(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
