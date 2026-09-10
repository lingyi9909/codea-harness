package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type task3ProgressStage struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type task3ProgressState struct {
	RunID        string               `json:"runId"`
	Status       string               `json:"status"`
	CurrentStage string               `json:"currentStage"`
	Stages       []task3ProgressStage `json:"stages"`
}

func Test164Task3ReviewBeginCreatesRuntimeOwnedProgress(t *testing.T) {
	withTempProject(t)

	if err := run([]string{"review", "begin"}); err != nil {
		t.Fatalf("review begin failed: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(".code-harness", "runs"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one fresh review run, entries=%v err=%v", entries, err)
	}
	runID := entries[0].Name()
	progressPath := filepath.Join(".code-harness", "runs", runID, "runtime", "review-progress.json")
	data, err := os.ReadFile(progressPath)
	if err != nil {
		t.Fatalf("review begin must persist Runtime-owned progress state: %v", err)
	}
	var state task3ProgressState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("decode progress state: %v", err)
	}
	if state.RunID != runID || state.Status != "RUNNING" || state.CurrentStage != "SNAPSHOT" {
		t.Fatalf("unexpected initial progress state: %+v", state)
	}
	if len(state.Stages) != 8 {
		t.Fatalf("expected canonical 8 stages, got %d", len(state.Stages))
	}
	if state.Stages[0].Name != "REVIEW_BEGIN" || state.Stages[0].Status != "SUCCEEDED" {
		t.Fatalf("stage 1 must be Runtime-completed at review begin: %+v", state.Stages[0])
	}
	if state.Stages[1].Name != "SNAPSHOT" || state.Stages[1].Status != "RUNNING" {
		t.Fatalf("stage 2 must be Runtime-started after review begin: %+v", state.Stages[1])
	}
	for i := 2; i < len(state.Stages); i++ {
		if state.Stages[i].Status != "PENDING" {
			t.Fatalf("later stage %s must remain PENDING, got %s", state.Stages[i].Name, state.Stages[i].Status)
		}
	}

	if err := run([]string{"review", "progress", "--run-id", runID}); err != nil {
		t.Fatalf("review progress must expose Runtime state read-only: %v", err)
	}
}
