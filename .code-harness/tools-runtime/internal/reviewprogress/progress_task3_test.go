package reviewprogress

import (
	"os"
	"path/filepath"
	"testing"
)

func newTask3Run(t *testing.T, runID string) string {
	t.Helper()
	root := t.TempDir()
	runDir := filepath.Join(root, ".code-harness", "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("create run dir: %v", err)
	}
	if _, err := Begin(root, runID); err != nil {
		t.Fatalf("begin progress: %v", err)
	}
	return root
}

func Test164Task3RuntimeOwnsCanonicalStageOrder(t *testing.T) {
	root := newTask3Run(t, "review-task3-order")
	stages := CanonicalStages()
	for _, stage := range stages[1:] {
		state, err := Advance(root, "review-task3-order", stage)
		if err != nil {
			t.Fatalf("advance %s: %v", stage, err)
		}
		if stage != "REPORT" && state.Status != StatusRunning {
			t.Fatalf("after %s expected RUNNING, got %s", stage, state.Status)
		}
	}
	state, err := Read(root, "review-task3-order")
	if err != nil {
		t.Fatalf("read final state: %v", err)
	}
	if state.Status != StatusSucceeded || state.TerminalStage != "REPORT" {
		t.Fatalf("expected terminal success at REPORT, got %+v", state)
	}
	for _, stage := range state.Stages {
		if stage.Status != StatusSucceeded {
			t.Fatalf("stage %s not succeeded: %s", stage.Name, stage.Status)
		}
	}
}

func Test164Task3IllegalTransitionFailsClosedAndDoesNotMutate(t *testing.T) {
	root := newTask3Run(t, "review-task3-skip")
	before, err := Read(root, "review-task3-skip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Advance(root, "review-task3-skip", "CERTIFICATION"); err == nil {
		t.Fatal("skipping SNAPSHOT/CHANGE_ANALYSIS must be rejected")
	}
	after, err := Read(root, "review-task3-skip")
	if err != nil {
		t.Fatal(err)
	}
	if after.CurrentStage != before.CurrentStage || after.Status != before.Status || len(after.Events) != len(before.Events) {
		t.Fatalf("illegal transition mutated state: before=%+v after=%+v", before, after)
	}
}

func Test164Task3FailureAttributionBlocksAllLaterStages(t *testing.T) {
	root := newTask3Run(t, "review-task3-failure")
	state, err := Fail(root, "review-task3-failure", "SNAPSHOT", "SNAPSHOT_CAPTURE_FAILED")
	if err != nil {
		t.Fatalf("fail snapshot: %v", err)
	}
	if state.Status != StatusFailed || state.FailureStage != "SNAPSHOT" || state.FailureCode != "SNAPSHOT_CAPTURE_FAILED" {
		t.Fatalf("wrong failure attribution: %+v", state)
	}
	if state.Stages[1].Status != StatusFailed {
		t.Fatalf("snapshot must be FAILED, got %s", state.Stages[1].Status)
	}
	for i := 2; i < len(state.Stages); i++ {
		if state.Stages[i].Status != StatusBlocked {
			t.Fatalf("later stage %s must be BLOCKED, got %s", state.Stages[i].Name, state.Stages[i].Status)
		}
	}
	if _, err := Advance(root, "review-task3-failure", "SNAPSHOT"); err == nil {
		t.Fatal("terminal failed run must reject later advance")
	}
}
