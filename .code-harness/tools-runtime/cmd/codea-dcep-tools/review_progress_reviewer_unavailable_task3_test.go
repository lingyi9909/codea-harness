package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/reviewprogress"
)

func task164Task3StartProgressAt(t *testing.T, runID, stage string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(".code-harness", "runs", runID), 0o755); err != nil {
		t.Fatalf("create review run: %v", err)
	}
	if _, err := reviewprogress.Begin(".", runID); err != nil {
		t.Fatalf("begin review progress: %v", err)
	}
	for {
		state, err := reviewprogress.Read(".", runID)
		if err != nil {
			t.Fatalf("read review progress: %v", err)
		}
		if state.CurrentStage == stage {
			return
		}
		if _, err := reviewprogress.Advance(".", runID, state.CurrentStage); err != nil {
			t.Fatalf("advance to %s from %s: %v", stage, state.CurrentStage, err)
		}
	}
}

func task164Task3AssertReviewerUnavailable(t *testing.T, runID, stage string) {
	t.Helper()
	err := run([]string{"review", "reviewer-unavailable", "--run-id", runID})
	if err == nil {
		t.Fatal("Reviewer Host failure must return a hard-stop error")
	}
	for _, marker := range []string{"REVIEWER_UNAVAILABLE", "MANUAL_ACTION_REQUIRED", "HARD STOP"} {
		if !strings.Contains(err.Error(), marker) {
			t.Fatalf("Reviewer Host failure missing %s: %v", marker, err)
		}
	}
	state, readErr := reviewprogress.Read(".", runID)
	if readErr != nil {
		t.Fatalf("read failed progress: %v", readErr)
	}
	if state.Status != reviewprogress.StatusFailed || state.CurrentStage != stage || state.TerminalStage != stage || state.FailureStage != stage || state.FailureCode != "REVIEWER_UNAVAILABLE" {
		t.Fatalf("wrong Reviewer Host failure attribution: %+v", state)
	}
	foundFailure := false
	for i, candidate := range reviewprogress.CanonicalStages() {
		if candidate == stage {
			if state.Stages[i].Status != reviewprogress.StatusFailed {
				t.Fatalf("failing stage %s status=%s", stage, state.Stages[i].Status)
			}
			for j := i + 1; j < len(state.Stages); j++ {
				if state.Stages[j].Status != reviewprogress.StatusBlocked {
					t.Fatalf("later stage %s must be BLOCKED, got %s", state.Stages[j].Name, state.Stages[j].Status)
				}
			}
		}
	}
	for _, event := range state.Events {
		if event.Stage == stage && event.Status == reviewprogress.StatusSucceeded {
			t.Fatalf("Reviewer Host failure emitted false PASS for %s: %+v", stage, event)
		}
		if event.Stage == stage && event.Status == reviewprogress.StatusFailed && event.FailureCode == "REVIEWER_UNAVAILABLE" {
			foundFailure = true
		}
	}
	if !foundFailure {
		t.Fatalf("missing Runtime REVIEWER_UNAVAILABLE failure event for %s", stage)
	}
}

func Test164Task3ReviewerUnavailableFailsChangeAnalysisAtHostBoundary(t *testing.T) {
	withTempProject(t)
	runID := "task164-host-change-analysis"
	task164Task3StartProgressAt(t, runID, reviewprogress.StageChangeAnalysis)
	task164Task3AssertReviewerUnavailable(t, runID, reviewprogress.StageChangeAnalysis)

	for _, forbidden := range []string{
		filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json"),
		filepath.Join(".code-harness", "runs", runID, "analysis", "review-options.json"),
		filepath.Join(".code-harness", "runs", runID, "review.md"),
	} {
		if _, err := os.Stat(forbidden); !os.IsNotExist(err) {
			t.Fatalf("Reviewer Host hard stop published downstream authority artifact %s", forbidden)
		}
	}
}

func Test164Task3ReviewerUnavailableFailsReviewExecutionAtHostBoundary(t *testing.T) {
	withTempProject(t)
	runID := "task164-host-review-execution"
	task164Task3StartProgressAt(t, runID, reviewprogress.StageReviewExecution)
	task164Task3AssertReviewerUnavailable(t, runID, reviewprogress.StageReviewExecution)

	for _, forbidden := range []string{
		filepath.Join(".code-harness", "runs", runID, "analysis", "certified-findings.json"),
		filepath.Join(".code-harness", "runs", runID, "analysis", "certified-findings.cert.json"),
		filepath.Join(".code-harness", "runs", runID, "review.md"),
	} {
		if _, err := os.Stat(forbidden); !os.IsNotExist(err) {
			t.Fatalf("Reviewer Host hard stop published downstream authority artifact %s", forbidden)
		}
	}
}

func Test164Task3ReviewerUnavailableCannotFailNonReviewerStage(t *testing.T) {
	withTempProject(t)
	runID := "task164-host-snapshot"
	if err := os.MkdirAll(filepath.Join(".code-harness", "runs", runID), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := reviewprogress.Begin(".", runID); err != nil {
		t.Fatal(err)
	}
	err := run([]string{"review", "reviewer-unavailable", "--run-id", runID})
	if err == nil || !strings.Contains(err.Error(), "REVIEWER_STAGE_REQUIRED") {
		t.Fatalf("non-Reviewer stage must reject Host failure attribution, got %v", err)
	}
	state, err := reviewprogress.Read(".", runID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != reviewprogress.StatusRunning || state.CurrentStage != reviewprogress.StageSnapshot {
		t.Fatalf("rejected Host failure mutated Runtime progress: %+v", state)
	}
}

func Test164Task3OpenCodeHostFailureReportsRuntimeProgressBeforeHardStop(t *testing.T) {
	bootstrapPath := filepath.Clean(filepath.Join("..", "..", "..", "bootstrap.md"))
	data, err := os.ReadFile(bootstrapPath)
	if err != nil {
		t.Fatalf("read bootstrap: %v", err)
	}
	text := string(data)
	for _, required := range []string{
		"codea-dcep-tools.exe review reviewer-unavailable --run-id <runId>",
		"resolve/start/invoke/complete",
		"REVIEWER_UNAVAILABLE",
		"不得继续 Runtime `analysis certify` 或 `review certify-findings`",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("bootstrap missing Reviewer Host failure progress contract %q", required)
		}
	}
}
