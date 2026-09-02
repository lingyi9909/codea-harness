package analysis

import (
	"strings"
	"testing"

	"codea-harness-tools/internal/changeset"
)

func Test162HotfixLegacyCertRejectsForgedCurrentBranchBaseCommitAndMergeBase(t *testing.T) {
	root := t.TempDir()
	copyAnalysisContract153(t, root, "change-analysis.schema.json")
	copyAnalysisContract153(t, root, "entrypoint-inventory.schema.json")

	snapshot := changeset.Snapshot{
		RequestedBaseRef: "develop", ResolvedBaseCommit: "base-real", MergeBase: "merge-real", HeadCommit: "head153", CurrentBranch: "feature", IncludeWorkingTree: true,
		BaseRef: "develop", Head: "head153", SHA256: strings.Repeat("a", 64), SnapshotSHA256: strings.Repeat("a", 64), Files: []changeset.File{},
	}
	inventory := EntrypointInventory{RunID: "r153", Status: "COMPLETE", ChangeSetSHA256: snapshot.SHA256}
	draft := validCertificationDraft153([]string{})
	scope := draft["reviewScope"].(map[string]any)
	scope["currentBranch"] = "forged-branch"
	scope["baseCommit"] = "forged-base"
	scope["mergeBase"] = "forged-merge"
	scope["headCommit"] = "head153"
	writeCertificationDraft153(t, root, "r153", draft)

	_, err := certifyWithRuntime153(root, CertifyRequest{
		RunID: "r153", DraftPath: ".code-harness/runs/r153/requests/change-analysis-draft.json",
		BaseRef: "develop", IncludeWorkingTree: true, Intent: Intent{Mode: "FULL"},
	}, fakeCertificationRuntime153{snapshot: snapshot, inventory: inventory})
	if err == nil || !strings.Contains(err.Error(), "CHANGE_SET_MISMATCH") {
		t.Fatalf("forged deterministic Git identity must fail closed before semantic certification, got %v", err)
	}
}
