package reviewrun

import (
    "context"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "reflect"
    "strings"
    "testing"
)

// Use a real Git DAG: base and feature have diverged; HEAD has committed
// changes and an entirely clean index/worktree. No mock Git output.
func Test180CommittedChangesDetectedFromConfiguredBaseRef(t *testing.T) {
    root := copyControllerReviewFixture180(t)
    baseline := initControllerReviewGitBaseline180(t, root)
    gitReview180(t, root, "branch", "base")
    gitReview180(t, root, "checkout", "-b", "feature")

    java := "src/main/java/com/example/OrderController.java"
    xml := "src/main/resources/mapper/OrderMapper.xml"
    appendReviewFile180(t, root, java, "\n// committed branch change\n")
    appendReviewFile180(t, root, xml, "\n<!-- committed mapper change -->\n")
    gitReview180(t, root, "add", java, xml)
    gitReview180(t, root, "commit", "-m", "feature changes")

    gitReview180(t, root, "checkout", "base")
    appendReviewFile180(t, root, "README.md", "\nbase only change\n")
    gitReview180(t, root, "add", "README.md")
    gitReview180(t, root, "commit", "-m", "base advances independently")
    gitReview180(t, root, "checkout", "feature")
    writeReviewConfig180(t, root, "base", true)

    if status := gitReview180(t, root, "status", "--porcelain"); status != "" {
        t.Fatalf("test must have clean tracked worktree, got %q", status)
    }
    paths, err := changedSourceFiles180(context.Background(), root)
    if err != nil {
        t.Fatal(err)
    }
    expected := []string{java, xml}
    if !reflect.DeepEqual(paths, expected) {
        t.Fatalf("committed Java/MapperXML changes lost: got=%v want=%v", paths, expected)
    }
    snapshot, err := reviewChangesSnapshot180(root)
    if err != nil {
        t.Fatal(err)
    }
    if snapshot.MergeBase != baseline {
        t.Fatalf("incorrect merge-base=%q want=%q", snapshot.MergeBase, baseline)
    }
    lineRanges, err := changedLineRangesForPath180(context.Background(), root, java)
    if err != nil || len(lineRanges) == 0 {
        t.Fatalf("committed lines cannot support introducedByChange: ranges=%v err=%v", lineRanges, err)
    }
}

func Test180ReviewBaseRefMissingFailsClosed(t *testing.T) {
    root := copyControllerReviewFixture180(t)
    initControllerReviewGitBaseline180(t, root)
    writeReviewConfig180(t, root, "missing-local-base", true)
    _, err := changedSourceFiles180(context.Background(), root)
    if err == nil || !strings.Contains(err.Error(), "CHANGE_SET_BASE_REF_NOT_FOUND") {
        t.Fatalf("missing baseRef silently passed: %v", err)
    }
}

func Test180ReviewChangesWithoutConfiguredBasePreservesWorkingTreeMode(t *testing.T) {
    root := copyControllerReviewFixture180(t)
    initControllerReviewGitBaseline180(t, root)
    java := "src/main/java/com/example/OrderController.java"
    appendReviewFile180(t, root, java, "\n// uncommitted\n")
    paths, err := changedSourceFiles180(context.Background(), root)
    if err != nil || !reflect.DeepEqual(paths, []string{java}) {
        t.Fatalf("legacy CHANGES working-tree path drifted: %v err=%v", paths, err)
    }
}

func Test180ReviewIncludeWorkingTreeFalseHonored(t *testing.T) {
    root := copyControllerReviewFixture180(t)
    baseline := initControllerReviewGitBaseline180(t, root)
    writeReviewConfig180(t, root, baseline, false)
    java := "src/main/java/com/example/OrderController.java"
    appendReviewFile180(t, root, java, "\n// uncommitted\n")
    paths, err := changedSourceFiles180(context.Background(), root)
    if err != nil || len(paths) != 0 {
        t.Fatalf("working tree included when disabled: %v err=%v", paths, err)
    }
    appendReviewFile180(t, root, java, "\n// committed\n")
    gitReview180(t, root, "add", java)
    gitReview180(t, root, "commit", "-m", "now committed")
    paths, err = changedSourceFiles180(context.Background(), root)
    if err != nil || !reflect.DeepEqual(paths, []string{java}) {
        t.Fatalf("committed changes missing when working tree disabled: %v err=%v", paths, err)
    }
}

func Test180CommittedBranchPrepareDetectsAffectedChain(t *testing.T) {
    root := copyControllerReviewFixture180(t)
    useRealAstGrep180(t, root)
    initControllerReviewGitBaseline180(t, root)
    gitReview180(t, root, "branch", "base")
    gitReview180(t, root, "checkout", "-b", "feature")
    path := "src/main/java/com/example/OrderController.java"
    appendReviewFile180(t, root, path, "\n// committed only\n")
    gitReview180(t, root, "add", path)
    gitReview180(t, root, "commit", "-m", "change controller")
    writeReviewConfig180(t, root, "base", true)
    started, err := Start(root)
    if err != nil { t.Fatal(err) }
    opts, err := Prepare(context.Background(), root, started.RunID, Intent{Mode:"CHANGES", Target:"OrderController.create"})
    if err != nil || !opts.DiscoveryComplete || len(opts.Chains) != 1 {
        t.Fatalf("clean worktree committed branch review blocked: opts=%+v err=%v", opts, err)
    }
}

func gitReview180(t *testing.T, root string, args ...string) string {
    t.Helper()
    cmd := exec.Command("git", args...)
    cmd.Dir = root
    out, err := cmd.CombinedOutput()
    if err != nil { t.Fatalf("git %v failed: %v: %s", args, err, out) }
    return strings.TrimSpace(string(out))
}

func appendReviewFile180(t *testing.T, root, path, text string) {
    t.Helper()
    file := filepath.Join(root, filepath.FromSlash(path))
    f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
    if err != nil { t.Fatal(err) }
    if _, err := f.WriteString(text); err != nil { _ = f.Close(); t.Fatal(err) }
    if err := f.Close(); err != nil { t.Fatal(err) }
}

func writeReviewConfig180(t *testing.T, root, baseRef string, working bool) {
    t.Helper()
    path := filepath.Join(root, ".code-harness", "harness.yaml")
    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { t.Fatal(err) }
    content := fmt.Sprintf("review:\n  baseRef: %q\n  includeWorkingTree: %t\n", baseRef, working)
    if err := os.WriteFile(path, []byte(content), 0o600); err != nil { t.Fatal(err) }
}
