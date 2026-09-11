package main

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"codea-harness-tools/internal/reviewprogress"
)

func setupTask3SnapshotRun(t *testing.T, runID, baseRef string) string {
	t.Helper()
	root := t.TempDir()
	git153Cmd(t, root, "init")
	git153Cmd(t, root, "config", "user.email", "task3@example.test")
	git153Cmd(t, root, "config", "user.name", "Task 3")
	mustWrite153Cmd(t, filepath.Join(root, "src", "main", "java", "acme", "AService.java"), "class AService {}\n")
	git153Cmd(t, root, "add", ".")
	git153Cmd(t, root, "commit", "-m", "base")
	for _, name := range []string{"change-set.schema.json", "change-set-request.schema.json"} {
		copyTask153CommandContract(t, root, name)
	}
	reqBytes, err := json.Marshal(map[string]any{"runId": runID, "baseRef": baseRef, "includeWorkingTree": true})
	if err != nil {
		t.Fatal(err)
	}
	requestPath := filepath.Join(root, ".code-harness", "runs", runID, "requests", "change-set.json")
	mustWrite153Cmd(t, requestPath, string(reqBytes))
	if _, err := reviewprogress.Begin(root, runID); err != nil {
		t.Fatalf("begin progress: %v", err)
	}
	return root
}

func Test164Task3SnapshotSuccessAdvancesRuntimeProgress(t *testing.T) {
	const runID = "review-task3-snapshot-success"
	root := setupTask3SnapshotRun(t, runID, "HEAD")
	withChdir153Cmd(t, root, func() {
		if err := run([]string{"analysis", "snapshot", "--input", ".code-harness/runs/" + runID + "/requests/change-set.json"}); err != nil {
			t.Fatalf("analysis snapshot failed: %v", err)
		}
		state, err := reviewprogress.Read(".", runID)
		if err != nil {
			t.Fatal(err)
		}
		if state.Stages[1].Status != reviewprogress.StatusSucceeded || state.CurrentStage != reviewprogress.StageChangeAnalysis || state.Stages[2].Status != reviewprogress.StatusRunning {
			t.Fatalf("snapshot command did not drive Runtime progress: %+v", state)
		}
		if len(state.Stages[1].Artifacts) != 1 || state.Stages[1].Artifacts[0].Path != ".code-harness/runs/"+runID+"/analysis/change-set.json" {
			t.Fatalf("snapshot stage must bind canonical artifact identity: %+v", state.Stages[1].Artifacts)
		}
	})
}

func Test164Task3SnapshotFailureTerminatesAtSnapshot(t *testing.T) {
	const runID = "review-task3-snapshot-failure"
	root := setupTask3SnapshotRun(t, runID, "refs/heads/does-not-exist")
	withChdir153Cmd(t, root, func() {
		if err := run([]string{"analysis", "snapshot", "--input", ".code-harness/runs/" + runID + "/requests/change-set.json"}); err == nil {
			t.Fatal("invalid base ref must fail snapshot")
		}
		state, err := reviewprogress.Read(".", runID)
		if err != nil {
			t.Fatal(err)
		}
		if state.Status != reviewprogress.StatusFailed || state.FailureStage != reviewprogress.StageSnapshot {
			t.Fatalf("snapshot failure attribution missing: %+v", state)
		}
		for i := 2; i < len(state.Stages); i++ {
			if state.Stages[i].Status != reviewprogress.StatusBlocked {
				t.Fatalf("later stage %s must be BLOCKED, got %s", state.Stages[i].Name, state.Stages[i].Status)
			}
		}
	})
}
