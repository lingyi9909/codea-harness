"""One-shot, exact-anchor Task 1B source patch for the isolated Windows development job.

The job applies this to a fresh checkout, verifies the changed source, and only
then makes a normal fast-forward commit on the dedicated Task 1 branch.
"""
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
ANALYSIS = ROOT / '.code-harness/tools-runtime/internal/analysis'
NAV = ROOT / '.code-harness/tools-runtime/internal/nav'


def replace_once(path, old, new):
    text = path.read_text(encoding='utf-8')
    if text.count(old) != 1:
        raise RuntimeError(f'exact patch anchor mismatch: {path.name}: {text.count(old)}')
    path.write_text(text.replace(old, new), encoding='utf-8', newline='\n')


batch = ANALYSIS / 'entrypoint_batch_164.go'
text = batch.read_text(encoding='utf-8')
start = text.index('func (r exactBatchEntrypointRunner164) Run(')
end = text.index('\nfunc buildEntrypointInventoryBatch164(', start)
old = text[start:end]
new = '''func (r exactBatchEntrypointRunner164) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
    if err := validateEntrypointBatchInvocation164(r.dir, r.allowed, args); err != nil {
        return nil, err
    }
    cmd := exec.CommandContext(ctx, name, args...)
    cmd.Dir = r.dir
    return cmd.Output()
}
'''
replace_once(batch, old, new)

helper = ANALYSIS / 'entrypoint_runner_contract_164.go'
if helper.exists():
    raise RuntimeError('runner contract file already exists; refusing overwrite')
helper.write_text(r'''package analysis

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
''', encoding='utf-8', newline='\n')

nav = NAV / 'entrypoint_batch_164.go'
replace_once(nav, 'args = append(args, cleanScopes...)', 'args = append(args, "--")\n\targs = append(args, cleanScopes...)')
replace_once(nav, '''if runErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) || len(output) == 0 {
			return nil, runErr
		}
	}''', '''if runErr != nil {
        return nil, fmt.Errorf("ENTRYPOINT_AST_BATCH_EXECUTION_FAILED: %w", runErr)
    }''')
replace_once(nav, '\n\t"errors"', '')
replace_once(nav, '\n\t"os/exec"', '')

# Preserve the original RED cases and add positive/negative cases using the
# actual delimiter-bearing production command, including a parent symlink.
test = ANALYSIS / 'entrypoint_batch_scope_164_test.go'
text = test.read_text(encoding='utf-8')
text = text.replace('"scan", "--json=stream", name)', '"scan", "--inline-rules", "rule: {}", "--json=stream", "--color", "never", "--", name)')
text += r'''
func Test164Task1BRunnerExactCommandContract(t *testing.T) {
    root := t.TempDir()
    a := "src/main/java/demo/A.java"
    b := "src/main/java/demo/B.java"
    extra := "src/main/java/demo/Extra.java"
    for _, p := range []string{a, b, extra} { writeJava164(t, root, p, "class A {}") }
    r := exactBatchEntrypointRunner164{dir: root, allowed: pathSet164([]string{a, b})}
    prefix := []string{"scan", "--inline-rules", "rule: {}", "--json=stream", "--color", "never", "--"}
    valid := append(append([]string{}, prefix...), a, b)
    if err := validateEntrypointBatchInvocation164(root, r.allowed, valid); err != nil { t.Fatal(err) }
    for _, tc := range []struct { name string; args []string }{
        {"extra-before-targets", append(append([]string{}, prefix...), extra, a, b)},
        {"extra-after-targets", append(append([]string{}, prefix...), a, b, extra)},
        {"duplicate", append(append([]string{}, prefix...), a, a)},
        {"directory", append(append([]string{}, prefix...), "src/main/java", b)},
        {"glob", append(append([]string{}, prefix...), "**/*.java", b)},
        {"missing", append(append([]string{}, prefix...), a, "missing.java")},
        {"extra-option", append([]string{"scan", "--inline-rules", "rule: {}", "--json=stream", "--color", "never", "--no-ignore", "--"}, a, b)},
        {"missing-delimiter", append([]string{"scan", "--inline-rules", "rule: {}", "--json=stream", "--color", "never"}, a, b)},
        {"absolute", append(append([]string{}, prefix...), filepath.Join(root, filepath.FromSlash(a)), b)},
        {"traversal", append(append([]string{}, prefix...), "../A.java", b)},
    } {
        t.Run(tc.name, func(t *testing.T) {
            _, err := r.Run(context.Background(), "must-not-execute", tc.args...)
            if err == nil || !strings.Contains(err.Error(), "ENTRYPOINT_SCAN_SCOPE_WIDENED") { t.Fatalf("scope escape: %v", err) }
        })
    }
}

func Test164Task1BRunnerRejectsParentSymlink(t *testing.T) {
    root, outside := t.TempDir(), t.TempDir()
    name := "src/main/java/demo/A.java"
    writeJava164(t, outside, "demo/A.java", "class A {}")
    parent := filepath.Join(root, "src/main/java")
    if err := os.MkdirAll(filepath.Dir(parent), 0755); err != nil { t.Fatal(err) }
    if err := os.Symlink(outside, parent); err != nil { t.Skipf("symlink unavailable: %v", err) }
    r := exactBatchEntrypointRunner164{dir: root, allowed: pathSet164([]string{name})}
    args := []string{"scan", "--inline-rules", "rule: {}", "--json=stream", "--color", "never", "--", name}
    _, err := r.Run(context.Background(), "must-not-execute", args...)
    if err == nil || !strings.Contains(err.Error(), "ENTRYPOINT_SCAN_SCOPE_WIDENED") { t.Fatalf("parent symlink escaped: %v", err) }
}
'''
test.write_text(text, encoding='utf-8', newline='\n')
print('TASK164_RUNNER_PATCH_APPLIED')
