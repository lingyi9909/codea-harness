package reviewrun

import (
    "io/fs"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "testing"
)

func copyControllerReviewFixture180(t *testing.T) string {
    t.Helper()
    src := filepath.Join("testdata","controller-review")
    root := t.TempDir()
    err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
        if err != nil { return err }
        rel, err := filepath.Rel(src,path); if err != nil { return err }
        if rel == "." { return nil }
        dst := filepath.Join(root,rel)
        if d.IsDir() { return os.MkdirAll(dst,0o755) }
        data, err := os.ReadFile(path); if err != nil { return err }
        if err := os.MkdirAll(filepath.Dir(dst),0o755); err != nil { return err }
        return os.WriteFile(dst,data,0o600)
    })
    if err != nil { t.Fatal(err) }
    return root
}

func initControllerReviewGitBaseline180(t *testing.T, root string) string {
    t.Helper()
    run := func(env []string, args ...string) string {
        t.Helper()
        cmd := exec.Command("git", args...)
        cmd.Dir = root
        cmd.Env = append(os.Environ(), env...)
        out, err := cmd.CombinedOutput()
        if err != nil { t.Fatalf("git %v failed: %v\n%s", args, err, out) }
        return strings.TrimSpace(string(out))
    }
    run(nil, "init", "--quiet")
    run(nil, "config", "core.autocrlf", "false")
    run(nil, "config", "user.name", "Codea Harness Fixture")
    run(nil, "config", "user.email", "fixture@codea.invalid")
    run(nil, "add", "--all")
    fixed := []string{"GIT_AUTHOR_DATE=2000-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2000-01-01T00:00:00Z"}
    run(fixed, "commit", "--quiet", "-m", "task2 fixture baseline")
    head := run(nil, "rev-parse", "HEAD")
    if len(head) != 40 { t.Fatalf("unexpected fixture baseline SHA %q", head) }
    return head
}
