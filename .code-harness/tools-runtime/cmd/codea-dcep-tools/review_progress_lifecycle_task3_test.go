package main

import (
	"os"
	"path/filepath"
	"testing"

	"codea-harness-tools/internal/reviewprogress"
)

func task3AdvanceFixtureTo(t *testing.T, runID, target string) {
	t.Helper()
	if _, err := reviewprogress.Begin(".", runID); err != nil {
		t.Fatalf("begin Task 3 progress fixture: %v", err)
	}
	for {
		state, err := reviewprogress.Read(".", runID)
		if err != nil {
			t.Fatal(err)
		}
		if state.CurrentStage == target {
			return
		}
		if _, err := reviewprogress.Advance(".", runID, state.CurrentStage); err != nil {
			t.Fatalf("advance Task 3 fixture from %s to %s: %v", state.CurrentStage, target, err)
		}
	}
}

func task3AssertStage(t *testing.T, runID, current, status string) reviewprogress.State {
	t.Helper()
	state, err := reviewprogress.Read(".", runID)
	if err != nil {
		t.Fatal(err)
	}
	if state.CurrentStage != current || state.Status != status {
		t.Fatalf("progress current/status = %s/%s, want %s/%s: %+v", state.CurrentStage, state.Status, current, status, state)
	}
	return state
}

func Test164Task3ReviewPlanningCommandsAdvanceRuntimeProgress(t *testing.T) {
	withTempProject(t)
	const runID = "run-task4-review"
	_ = prepareTask164FindingCertification(t, false)
	for _, name := range []string{"review-units.json", "rule-dispatch.json"} {
		if err := os.Remove(filepath.Join(".code-harness", "runs", runID, "analysis", name)); err != nil {
			t.Fatalf("reset %s fixture: %v", name, err)
		}
	}
	task3AdvanceFixtureTo(t, runID, reviewprogress.StageReviewPlanning)

	if err := run([]string{"review", "units", "--run-id", runID}); err != nil {
		t.Fatalf("review units failed: %v", err)
	}
	task3AssertStage(t, runID, reviewprogress.StageReviewPlanning, reviewprogress.StatusRunning)
	if err := run([]string{"review", "dispatch", "--run-id", runID}); err != nil {
		t.Fatalf("review dispatch failed: %v", err)
	}
	state := task3AssertStage(t, runID, reviewprogress.StageReviewExecution, reviewprogress.StatusRunning)
	planning := state.Stages[4]
	if planning.Status != reviewprogress.StatusSucceeded || len(planning.Artifacts) != 2 {
		t.Fatalf("planning must bind review-units + rule-dispatch authority: %+v", planning)
	}
}

func Test164Task3ReviewPlanningFailureBlocksLaterStages(t *testing.T) {
	withTempProject(t)
	const runID = "run-task4-review"
	_ = prepareTask164FindingCertification(t, false)
	for _, name := range []string{"review-units.json", "rule-dispatch.json"} {
		_ = os.Remove(filepath.Join(".code-harness", "runs", runID, "analysis", name))
	}
	task3AdvanceFixtureTo(t, runID, reviewprogress.StageReviewPlanning)
	if err := run([]string{"review", "units", "--run-id", runID}); err != nil {
		t.Fatalf("review units fixture failed: %v", err)
	}
	if err := os.Remove(filepath.Join(".code-harness", "review-rules", "spring-v1.yaml")); err != nil {
		t.Fatalf("remove dispatch rules for failure injection: %v", err)
	}
	if err := run([]string{"review", "dispatch", "--run-id", runID}); err == nil {
		t.Fatal("missing review rules must fail planning")
	}
	state := task3AssertStage(t, runID, reviewprogress.StageReviewPlanning, reviewprogress.StatusFailed)
	if state.FailureStage != reviewprogress.StageReviewPlanning || state.Stages[5].Status != reviewprogress.StatusBlocked {
		t.Fatalf("planning failure must block execution: %+v", state)
	}
}

func Test164Task3ReviewerAuthorityFailureTerminatesReviewExecution(t *testing.T) {
	withTempProject(t)
	const runID = "run-task4-review"
	request := prepareTask164FindingCertification(t, false)
	task3AdvanceFixtureTo(t, runID, reviewprogress.StageReviewExecution)
	if err := run([]string{"review", "certify-findings", "--input", request}); err == nil {
		t.Fatal("missing Reviewer Host authority must fail review execution")
	}
	state := task3AssertStage(t, runID, reviewprogress.StageReviewExecution, reviewprogress.StatusFailed)
	if state.FailureStage != reviewprogress.StageReviewExecution || state.Stages[6].Status != reviewprogress.StatusBlocked {
		t.Fatalf("execution failure must block finding certification: %+v", state)
	}
}

func Test164Task3FindingCertificationFailureIsAttributedAfterReviewerAuthority(t *testing.T) {
	withTempProject(t)
	const runID = "run-task4-review"
	request := prepareTask164FindingCertification(t, true)
	if err := os.Remove(filepath.Join(".code-harness", "bin", "ast-grep.exe")); err != nil {
		t.Fatalf("remove pinned navigation for certification failure injection: %v", err)
	}
	task3AdvanceFixtureTo(t, runID, reviewprogress.StageReviewExecution)
	if err := run([]string{"review", "certify-findings", "--input", request}); err == nil {
		t.Fatal("missing Runtime verification dependency must fail finding certification")
	}
	state := task3AssertStage(t, runID, reviewprogress.StageFindingCertification, reviewprogress.StatusFailed)
	if state.Stages[5].Status != reviewprogress.StatusSucceeded || state.FailureStage != reviewprogress.StageFindingCertification || state.Stages[7].Status != reviewprogress.StatusBlocked {
		t.Fatalf("failure after Reviewer authority must be attributed to finding certification: %+v", state)
	}
}

func Test164Task3FindingCertificationAndReportCompleteRuntimeProgress(t *testing.T) {
	withTempProject(t)
	const runID = "run-task4-review"
	request := prepareTask164FindingCertification(t, true)
	task3AdvanceFixtureTo(t, runID, reviewprogress.StageReviewExecution)
	if err := run([]string{"review", "certify-findings", "--input", request}); err != nil {
		t.Fatalf("finding certification failed: %v", err)
	}
	state := task3AssertStage(t, runID, reviewprogress.StageReport, reviewprogress.StatusRunning)
	if state.Stages[5].Status != reviewprogress.StatusSucceeded || state.Stages[6].Status != reviewprogress.StatusSucceeded {
		t.Fatalf("Reviewer execution and finding certification must both be Runtime-completed: %+v", state)
	}
	transport := writeTask164ReportTransport(t, runID)
	if err := run([]string{"report", "review", "--input", transport}); err != nil {
		t.Fatalf("report review failed: %v", err)
	}
	state = task3AssertStage(t, runID, reviewprogress.StageReport, reviewprogress.StatusSucceeded)
	if state.TerminalStage != reviewprogress.StageReport || state.Stages[7].Status != reviewprogress.StatusSucceeded || len(state.Stages[7].Artifacts) != 1 {
		t.Fatalf("report must terminally complete the Runtime state machine: %+v", state)
	}
}

func Test164Task3ReportFailureTerminatesAtReport(t *testing.T) {
	withTempProject(t)
	const runID = "run-task4-review"
	request := prepareTask164FindingCertification(t, true)
	task3AdvanceFixtureTo(t, runID, reviewprogress.StageReviewExecution)
	if err := run([]string{"review", "certify-findings", "--input", request}); err != nil {
		t.Fatalf("finding certification fixture failed: %v", err)
	}
	if err := os.Remove(filepath.Join(".code-harness", "runs", runID, "analysis", "certified-findings.json")); err != nil {
		t.Fatalf("remove certified findings for report failure injection: %v", err)
	}
	transport := writeTask164ReportTransport(t, runID)
	if err := run([]string{"report", "review", "--input", transport}); err == nil {
		t.Fatal("missing certified findings must fail report")
	}
	state := task3AssertStage(t, runID, reviewprogress.StageReport, reviewprogress.StatusFailed)
	if state.FailureStage != reviewprogress.StageReport {
		t.Fatalf("report failure attribution missing: %+v", state)
	}
}
