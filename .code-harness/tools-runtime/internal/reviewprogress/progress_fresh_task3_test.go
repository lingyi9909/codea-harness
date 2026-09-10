package reviewprogress

import "testing"

func Test164Task3FreshRunsAreIsolated(t *testing.T) {
	root := t.TempDir()
	for _, runID := range []string{"review-task3-fresh-a", "review-task3-fresh-b"} {
		createTask3RunDir(t, root, runID)
		if _, err := Begin(root, runID); err != nil {
			t.Fatalf("begin %s: %v", runID, err)
		}
	}

	if _, err := Advance(root, "review-task3-fresh-a", StageSnapshot); err != nil {
		t.Fatalf("advance run A: %v", err)
	}
	if _, err := Fail(root, "review-task3-fresh-b", StageSnapshot, "SNAPSHOT_CAPTURE_FAILED"); err != nil {
		t.Fatalf("fail run B: %v", err)
	}

	a, err := Read(root, "review-task3-fresh-a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Read(root, "review-task3-fresh-b")
	if err != nil {
		t.Fatal(err)
	}
	if a.Status != StatusRunning || a.CurrentStage != StageChangeAnalysis || a.FailureStage != "" {
		t.Fatalf("run A was contaminated by run B: %+v", a)
	}
	if b.Status != StatusFailed || b.CurrentStage != StageSnapshot || b.FailureStage != StageSnapshot {
		t.Fatalf("run B failure attribution invalid: %+v", b)
	}
}

func Test164Task3RuntimeEventsAreMonotonicAndDisplayable(t *testing.T) {
	root := newTask3Run(t, "review-task3-events")
	if _, err := Advance(root, "review-task3-events", StageSnapshot); err != nil {
		t.Fatal(err)
	}
	if _, err := Fail(root, "review-task3-events", StageChangeAnalysis, "CHANGE_ANALYSIS_AUTHORITY_FAILED"); err != nil {
		t.Fatal(err)
	}
	state, err := Read(root, "review-task3-events")
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Events) < 6 {
		t.Fatalf("expected Runtime lifecycle events, got %d", len(state.Events))
	}
	for i, event := range state.Events {
		if event.Sequence != i+1 {
			t.Fatalf("event sequence at %d = %d", i, event.Sequence)
		}
		if event.Display == "" || event.Stage == "" || event.Status == "" {
			t.Fatalf("event %d is not displayable Runtime evidence: %+v", i, event)
		}
	}
	last := state.Events[len(state.Events)-1]
	if last.Stage != StageChangeAnalysis || last.Status != StatusFailed || last.FailureCode != "CHANGE_ANALYSIS_AUTHORITY_FAILED" {
		t.Fatalf("terminal event not Runtime-derived failure evidence: %+v", last)
	}
}

func createTask3RunDir(t *testing.T, root, runID string) {
	t.Helper()
	rel, err := Path(runID)
	if err != nil {
		t.Fatal(err)
	}
	// Path returns the progress file; Begin only requires its owning run directory.
	if err := ensureDirForTask3Test(root, rel); err != nil {
		t.Fatal(err)
	}
}
