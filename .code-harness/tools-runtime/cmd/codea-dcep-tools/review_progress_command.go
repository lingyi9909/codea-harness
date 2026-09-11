package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"codea-harness-tools/internal/reviewprogress"
)

func runReviewProgress164(args []string) error {
	fs := flag.NewFlagSet("review progress", flag.ContinueOnError)
	runID := fs.String("run-id", "", "review run id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || strings.TrimSpace(*runID) == "" || *runID != strings.TrimSpace(*runID) {
		return errors.New("review progress requires canonical --run-id")
	}
	state, err := reviewprogress.Read(".", *runID)
	if err != nil {
		return err
	}
	return writeJSONAndStatus(state, true)
}

// runReviewReviewerUnavailable164 is a failure-only Runtime boundary used when
// the OpenCode Reviewer Host cannot resolve, start, invoke, or complete the
// independent Reviewer child session. The caller cannot choose a stage or a
// result: Runtime reads the current stage and only permits the two stages that
// actually depend on a live Reviewer. This signal can deny authority, but it
// can never manufacture PASS or advance the review lifecycle.
func runReviewReviewerUnavailable164(args []string) error {
	fs := flag.NewFlagSet("review reviewer-unavailable", flag.ContinueOnError)
	runID := fs.String("run-id", "", "Runtime review run id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	hardStop := errors.New("REVIEWER_UNAVAILABLE\nMANUAL_ACTION_REQUIRED\nHARD STOP")
	canonicalRunID := strings.TrimSpace(*runID)
	if fs.NArg() != 0 || canonicalRunID == "" || canonicalRunID != *runID {
		return errors.Join(hardStop, errors.New("REVIEWER_RUN_ID_REQUIRED: review reviewer-unavailable requires canonical --run-id"))
	}
	state, err := reviewprogress.Read(".", canonicalRunID)
	if err != nil {
		return errors.Join(hardStop, fmt.Errorf("REVIEW_PROGRESS_READ_FAILED: %w", err))
	}
	if state.Status == reviewprogress.StatusFailed && state.FailureCode == "REVIEWER_UNAVAILABLE" &&
		(state.FailureStage == reviewprogress.StageChangeAnalysis || state.FailureStage == reviewprogress.StageReviewExecution) {
		return hardStop
	}
	if state.Status != reviewprogress.StatusRunning ||
		(state.CurrentStage != reviewprogress.StageChangeAnalysis && state.CurrentStage != reviewprogress.StageReviewExecution) {
		return errors.Join(hardStop, fmt.Errorf("REVIEWER_STAGE_REQUIRED: current=%s status=%s", state.CurrentStage, state.Status))
	}
	return failReviewProgressStage164(canonicalRunID, state.CurrentStage, "REVIEWER_UNAVAILABLE", hardStop)
}

// advanceReviewProgressStage164 keeps historical direct Runtime commands
// backward-compatible: commands only participate in Task 3 progress when the
// same run already owns a Runtime review-progress state. Once such state
// exists, however, an out-of-order transition is a hard Runtime error rather
// than a silent bypass.
func advanceReviewProgressStage164(runID, stage string, artifactPaths ...string) error {
	if strings.TrimSpace(runID) == "" {
		return nil
	}
	state, err := reviewprogress.Read(".", runID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("REVIEW_PROGRESS_READ_FAILED: %w", err)
	}
	if state.Status != reviewprogress.StatusRunning || state.CurrentStage != stage {
		return fmt.Errorf("REVIEW_PROGRESS_STAGE_MISMATCH: current=%s status=%s expected=%s", state.CurrentStage, state.Status, stage)
	}
	if _, err := reviewprogress.Advance(".", runID, stage, artifactPaths...); err != nil {
		return fmt.Errorf("REVIEW_PROGRESS_ADVANCE_FAILED: %w", err)
	}
	return nil
}

// failReviewProgressStage164 preserves the original product error while also
// recording deterministic Runtime failure attribution. Prompt text and Agent
// narration never call reviewprogress.Fail directly and therefore cannot
// manufacture stage authority.
func failReviewProgressStage164(runID, stage, failureCode string, cause error) error {
	if cause == nil || strings.TrimSpace(runID) == "" {
		return cause
	}
	state, err := reviewprogress.Read(".", runID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cause
		}
		return errors.Join(cause, fmt.Errorf("REVIEW_PROGRESS_READ_FAILED: %w", err))
	}
	if state.Status == reviewprogress.StatusFailed && state.FailureStage == stage {
		return cause
	}
	if state.Status != reviewprogress.StatusRunning || state.CurrentStage != stage {
		return errors.Join(cause, fmt.Errorf("REVIEW_PROGRESS_STAGE_MISMATCH: current=%s status=%s expected=%s", state.CurrentStage, state.Status, stage))
	}
	if _, err := reviewprogress.Fail(".", runID, stage, failureCode); err != nil {
		return errors.Join(cause, fmt.Errorf("REVIEW_PROGRESS_FAIL_FAILED: %w", err))
	}
	return cause
}
