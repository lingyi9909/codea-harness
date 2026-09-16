package reviewrun

import (
    "io/fs"
    "os"
    "path/filepath"
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
