package reviewrun

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A successful CHANGES prepare must not reuse its prior scope after a
// baseRef-only configuration change. The Java and Mapper bytes are identical
// across these attempts, so source fingerprint checks alone cannot detect it.
func Test180ChangesPrepareRechecksGitSnapshotOnSameRun(t *testing.T) {
	root := copyControllerReviewFixture180(t)
	useRealAstGrep180(t, root)
	initControllerReviewGitBaseline180(t, root)
	gitReview180(t, root, "branch", "base")
	gitReview180(t, root, "checkout", "-b", "feature")
	changed := "src/main/java/com/example/OrderController.java"
	replaceT5Fixture180(t, root, changed, "orderService.create();", "orderService.create(); // committed change")
	gitReview180(t, root, "add", changed)
	gitReview180(t, root, "commit", "-m", "feature controller change")
	writeReviewConfig180(t, root, "base", true)

	start, err := Start(root)
	if err != nil {
		t.Fatal(err)
	}
	intent := Intent{Mode: "CHANGES", Target: "OrderController.create"}
	first, err := Prepare(context.Background(), root, start.RunID, intent)
	if err != nil || !first.DiscoveryComplete || len(first.Chains) != 1 {
		t.Fatalf("first prepare must see committed change: %+v %v", first, err)
	}

	// Only review.baseRef changes. A cache keyed solely by source contents
	// would falsely return the prior complete Controller chain.
	writeReviewConfig180(t, root, "HEAD", true)
	noDiff, err := Prepare(context.Background(), root, start.RunID, intent)
	if err != nil || noDiff.DiscoveryComplete || len(noDiff.Chains) != 0 ||
		!strings.Contains(strings.Join(noDiff.Gaps, "\n"), "NO_RELEVANT_CHANGES") {
		t.Fatalf("baseRef change reused stale complete options: %+v %v", noDiff, err)
	}
	if noDiff.Hash == first.Hash {
		t.Fatal("Git snapshot change retained stale optionsHash")
	}
	runDir, state, err := loadRun(root, start.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if state.ScopeReady || state.Coverage != "PARTIAL" {
		t.Fatalf("old scope survived changed baseRef: %+v", state)
	}
	if _, err := os.Stat(filepath.Join(runDir, "scope.json")); !os.IsNotExist(err) {
		t.Fatalf("scope.json remained after changed baseRef: %v", err)
	}
	status, err := Status(root, start.RunID)
	if err != nil || status.Execution != "INCOMPLETE" {
		t.Fatalf("same run must remain INCOMPLETE: %+v %v", status, err)
	}

	// Restore the previous baseRef and retry the original run.
	writeReviewConfig180(t, root, "base", true)
	recovered, err := Prepare(context.Background(), root, start.RunID, intent)
	if err != nil || !recovered.DiscoveryComplete || len(recovered.Chains) != 1 {
		t.Fatalf("same run did not recover after baseRef restoration: %+v %v", recovered, err)
	}
}

func Test180ChangesPrepareInvalidBaseRefInvalidatesCompleteCache(t *testing.T) {
	root := copyControllerReviewFixture180(t)
	useRealAstGrep180(t, root)
	initControllerReviewGitBaseline180(t, root)
	gitReview180(t, root, "branch", "base")
	gitReview180(t, root, "checkout", "-b", "feature")
	changed := "src/main/java/com/example/OrderController.java"
	replaceT5Fixture180(t, root, changed, "orderService.create();", "orderService.create(); // branch change")
	gitReview180(t, root, "add", changed)
	gitReview180(t, root, "commit", "-m", "feature branch change")
	writeReviewConfig180(t, root, "base", true)

	start, err := Start(root)
	if err != nil {
		t.Fatal(err)
	}
	intent := Intent{Mode: "CHANGES", Target: "OrderController.create"}
	first, err := Prepare(context.Background(), root, start.RunID, intent)
	if err != nil || !first.DiscoveryComplete {
		t.Fatalf("expected initial complete scope: %+v %v", first, err)
	}

	writeReviewConfig180(t, root, "branch-does-not-exist", true)
	failed, err := Prepare(context.Background(), root, start.RunID, intent)
	if err != nil || failed.DiscoveryComplete ||
		!strings.Contains(strings.Join(failed.Gaps, "\n"), "CHANGESET_DISCOVERY_FAILED") {
		t.Fatalf("invalid baseRef reused old scope: %+v %v", failed, err)
	}
	_, state, err := loadRun(root, start.RunID)
	if err != nil || state.ScopeReady {
		t.Fatalf("invalid baseRef retained ready scope: %+v %v", state, err)
	}
}
