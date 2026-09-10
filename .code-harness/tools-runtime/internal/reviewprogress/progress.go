package reviewprogress

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

const (
	StatusPending   = "PENDING"
	StatusRunning   = "RUNNING"
	StatusSucceeded = "SUCCEEDED"
	StatusFailed    = "FAILED"
	StatusBlocked   = "BLOCKED"
)

var runIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

var canonicalStages = []string{
	"REVIEW_BEGIN",
	"SNAPSHOT",
	"CHANGE_ANALYSIS",
	"CERTIFICATION",
	"REVIEW_PLANNING",
	"REVIEW_EXECUTION",
	"FINDING_CERTIFICATION",
	"REPORT",
}

type StageState struct {
	Index       int    `json:"index"`
	Total       int    `json:"total"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	StartedAt   string `json:"startedAt,omitempty"`
	CompletedAt string `json:"completedAt,omitempty"`
	DurationMS  int64  `json:"durationMs,omitempty"`
}

type Event struct {
	Sequence    int    `json:"sequence"`
	Index       int    `json:"index"`
	Total       int    `json:"total"`
	Stage       string `json:"stage"`
	Status      string `json:"status"`
	Timestamp   string `json:"timestamp"`
	FailureCode string `json:"failureCode,omitempty"`
	Display     string `json:"display"`
}

type State struct {
	Version       int          `json:"version"`
	RunID         string       `json:"runId"`
	Status        string       `json:"status"`
	CurrentStage  string       `json:"currentStage"`
	TerminalStage string       `json:"terminalStage,omitempty"`
	FailureStage  string       `json:"failureStage,omitempty"`
	FailureCode   string       `json:"failureCode,omitempty"`
	StartedAt     string       `json:"startedAt"`
	UpdatedAt     string       `json:"updatedAt"`
	Stages        []StageState `json:"stages"`
	Events        []Event      `json:"events"`
}

func CanonicalStages() []string {
	return append([]string(nil), canonicalStages...)
}

func Path(runID string) (string, error) {
	if !runIDPattern.MatchString(runID) {
		return "", fmt.Errorf("REVIEW_PROGRESS_RUN_ID_INVALID: %q", runID)
	}
	return filepath.Join(".code-harness", "runs", runID, "runtime", "review-progress.json"), nil
}

func Begin(repoRoot, runID string) (State, error) {
	rel, err := Path(runID)
	if err != nil {
		return State{}, err
	}
	runDir := filepath.Join(repoRoot, ".code-harness", "runs", runID)
	info, err := os.Stat(runDir)
	if err != nil || !info.IsDir() {
		if err == nil {
			err = errors.New("run path is not a directory")
		}
		return State{}, fmt.Errorf("REVIEW_PROGRESS_RUN_UNAVAILABLE: %w", err)
	}

	now := time.Now().UTC()
	ts := now.Format(time.RFC3339Nano)
	stages := make([]StageState, len(canonicalStages))
	for i, name := range canonicalStages {
		stages[i] = StageState{Index: i + 1, Total: len(canonicalStages), Name: name, Status: StatusPending}
	}
	stages[0].Status = StatusSucceeded
	stages[0].StartedAt = ts
	stages[0].CompletedAt = ts
	stages[1].Status = StatusRunning
	stages[1].StartedAt = ts

	state := State{
		Version:      1,
		RunID:        runID,
		Status:       StatusRunning,
		CurrentStage: canonicalStages[1],
		StartedAt:    ts,
		UpdatedAt:    ts,
		Stages:       stages,
		Events: []Event{
			{Sequence: 1, Index: 1, Total: len(canonicalStages), Stage: canonicalStages[0], Status: StatusRunning, Timestamp: ts, Display: "[1/8] REVIEW_BEGIN RUNNING"},
			{Sequence: 2, Index: 1, Total: len(canonicalStages), Stage: canonicalStages[0], Status: StatusSucceeded, Timestamp: ts, Display: "[1/8] REVIEW_BEGIN PASS"},
			{Sequence: 3, Index: 2, Total: len(canonicalStages), Stage: canonicalStages[1], Status: StatusRunning, Timestamp: ts, Display: "[2/8] SNAPSHOT RUNNING"},
		},
	}
	if err := validate(state); err != nil {
		return State{}, err
	}
	if err := writeState(filepath.Join(repoRoot, rel), state, false); err != nil {
		return State{}, fmt.Errorf("REVIEW_PROGRESS_WRITE_FAILED: %w", err)
	}
	return state, nil
}

func Read(repoRoot, runID string) (State, error) {
	rel, err := Path(runID)
	if err != nil {
		return State{}, err
	}
	data, err := os.ReadFile(filepath.Join(repoRoot, rel))
	if err != nil {
		return State{}, fmt.Errorf("REVIEW_PROGRESS_READ_FAILED: %w", err)
	}
	var state State
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&state); err != nil {
		return State{}, fmt.Errorf("REVIEW_PROGRESS_INVALID: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return State{}, errors.New("REVIEW_PROGRESS_INVALID: trailing JSON")
	}
	if state.RunID != runID {
		return State{}, errors.New("REVIEW_PROGRESS_RUN_ID_MISMATCH")
	}
	if err := validate(state); err != nil {
		return State{}, err
	}
	return state, nil
}

func validate(state State) error {
	if state.Version != 1 || !runIDPattern.MatchString(state.RunID) {
		return errors.New("REVIEW_PROGRESS_INVALID: identity")
	}
	if len(state.Stages) != len(canonicalStages) {
		return errors.New("REVIEW_PROGRESS_INVALID: stage count")
	}
	for i, expected := range canonicalStages {
		stage := state.Stages[i]
		if stage.Index != i+1 || stage.Total != len(canonicalStages) || stage.Name != expected {
			return errors.New("REVIEW_PROGRESS_INVALID: canonical stage order")
		}
	}
	return nil
}

func writeState(path string, state State, replace bool) error {
	if !replace {
		if _, err := os.Stat(path); err == nil {
			return os.ErrExist
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".review-progress-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	return nil
}
