package reviewprogress

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	StatusPending   = "PENDING"
	StatusRunning   = "RUNNING"
	StatusSucceeded = "SUCCEEDED"
	StatusFailed    = "FAILED"
	StatusBlocked   = "BLOCKED"
)

const (
	StageReviewBegin          = "REVIEW_BEGIN"
	StageSnapshot             = "SNAPSHOT"
	StageChangeAnalysis       = "CHANGE_ANALYSIS"
	StageCertification        = "CERTIFICATION"
	StageReviewPlanning       = "REVIEW_PLANNING"
	StageReviewExecution      = "REVIEW_EXECUTION"
	StageFindingCertification = "FINDING_CERTIFICATION"
	StageReport               = "REPORT"
)

var runIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
var failureCodePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,127}$`)

var canonicalStages = []string{
	StageReviewBegin,
	StageSnapshot,
	StageChangeAnalysis,
	StageCertification,
	StageReviewPlanning,
	StageReviewExecution,
	StageFindingCertification,
	StageReport,
}

type ArtifactIdentity struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type StageState struct {
	Index       int                `json:"index"`
	Total       int                `json:"total"`
	Name        string             `json:"name"`
	Status      string             `json:"status"`
	StartedAt   string             `json:"startedAt,omitempty"`
	CompletedAt string             `json:"completedAt,omitempty"`
	DurationMS  int64              `json:"durationMs,omitempty"`
	Artifacts   []ArtifactIdentity `json:"artifacts,omitempty"`
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
		CurrentStage: StageSnapshot,
		StartedAt:    ts,
		UpdatedAt:    ts,
		Stages:       stages,
		Events: []Event{
			{Sequence: 1, Index: 1, Total: len(canonicalStages), Stage: StageReviewBegin, Status: StatusRunning, Timestamp: ts, Display: "[1/8] REVIEW_BEGIN RUNNING"},
			{Sequence: 2, Index: 1, Total: len(canonicalStages), Stage: StageReviewBegin, Status: StatusSucceeded, Timestamp: ts, Display: "[1/8] REVIEW_BEGIN PASS"},
			{Sequence: 3, Index: 2, Total: len(canonicalStages), Stage: StageSnapshot, Status: StatusRunning, Timestamp: ts, Display: "[2/8] SNAPSHOT RUNNING"},
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

// Advance atomically completes exactly the currently RUNNING stage. Runtime is
// the only authority that can select the next canonical stage.
func Advance(repoRoot, runID, stage string, artifactPaths ...string) (State, error) {
	state, err := Read(repoRoot, runID)
	if err != nil {
		return State{}, err
	}
	if state.Status != StatusRunning {
		return State{}, fmt.Errorf("REVIEW_PROGRESS_TERMINAL: run status %s", state.Status)
	}
	idx := stageIndex(stage)
	if idx < 0 || state.CurrentStage != stage || state.Stages[idx].Status != StatusRunning {
		return State{}, fmt.Errorf("REVIEW_PROGRESS_ILLEGAL_TRANSITION: current=%s requested=%s", state.CurrentStage, stage)
	}
	artifacts, err := artifactIdentities(repoRoot, runID, artifactPaths)
	if err != nil {
		return State{}, err
	}

	now := time.Now().UTC()
	ts := now.Format(time.RFC3339Nano)
	current := &state.Stages[idx]
	current.Status = StatusSucceeded
	current.CompletedAt = ts
	current.DurationMS = durationMS(current.StartedAt, now)
	current.Artifacts = artifacts
	state.Events = append(state.Events, Event{
		Sequence: len(state.Events) + 1,
		Index:    idx + 1,
		Total:    len(canonicalStages),
		Stage:    stage,
		Status:   StatusSucceeded,
		Timestamp: ts,
		Display:  fmt.Sprintf("[%d/%d] %s PASS", idx+1, len(canonicalStages), stage),
	})

	if idx == len(canonicalStages)-1 {
		state.Status = StatusSucceeded
		state.CurrentStage = StageReport
		state.TerminalStage = StageReport
	} else {
		next := &state.Stages[idx+1]
		next.Status = StatusRunning
		next.StartedAt = ts
		state.CurrentStage = next.Name
		state.Events = append(state.Events, Event{
			Sequence:  len(state.Events) + 1,
			Index:     idx + 2,
			Total:     len(canonicalStages),
			Stage:     next.Name,
			Status:    StatusRunning,
			Timestamp: ts,
			Display:   fmt.Sprintf("[%d/%d] %s RUNNING", idx+2, len(canonicalStages), next.Name),
		})
	}
	state.UpdatedAt = ts
	if err := validate(state); err != nil {
		return State{}, err
	}
	if err := persist(repoRoot, runID, state); err != nil {
		return State{}, err
	}
	return state, nil
}

// Fail atomically terminates the currently RUNNING stage and blocks every
// later authoritative stage. It cannot be used to attribute a failure to a
// stage that Runtime has not actually entered.
func Fail(repoRoot, runID, stage, failureCode string) (State, error) {
	state, err := Read(repoRoot, runID)
	if err != nil {
		return State{}, err
	}
	if state.Status != StatusRunning {
		return State{}, fmt.Errorf("REVIEW_PROGRESS_TERMINAL: run status %s", state.Status)
	}
	idx := stageIndex(stage)
	if idx < 0 || state.CurrentStage != stage || state.Stages[idx].Status != StatusRunning {
		return State{}, fmt.Errorf("REVIEW_PROGRESS_ILLEGAL_FAILURE: current=%s requested=%s", state.CurrentStage, stage)
	}
	failureCode = strings.TrimSpace(failureCode)
	if !failureCodePattern.MatchString(failureCode) {
		return State{}, fmt.Errorf("REVIEW_PROGRESS_FAILURE_CODE_INVALID: %q", failureCode)
	}

	now := time.Now().UTC()
	ts := now.Format(time.RFC3339Nano)
	current := &state.Stages[idx]
	current.Status = StatusFailed
	current.CompletedAt = ts
	current.DurationMS = durationMS(current.StartedAt, now)
	for i := idx + 1; i < len(state.Stages); i++ {
		state.Stages[i].Status = StatusBlocked
	}
	state.Status = StatusFailed
	state.CurrentStage = stage
	state.TerminalStage = stage
	state.FailureStage = stage
	state.FailureCode = failureCode
	state.UpdatedAt = ts
	state.Events = append(state.Events, Event{
		Sequence:    len(state.Events) + 1,
		Index:       idx + 1,
		Total:       len(canonicalStages),
		Stage:       stage,
		Status:      StatusFailed,
		Timestamp:   ts,
		FailureCode: failureCode,
		Display:     fmt.Sprintf("[%d/%d] %s FAIL (%s)", idx+1, len(canonicalStages), stage, failureCode),
	})
	if err := validate(state); err != nil {
		return State{}, err
	}
	if err := persist(repoRoot, runID, state); err != nil {
		return State{}, err
	}
	return state, nil
}

func stageIndex(name string) int {
	for i, candidate := range canonicalStages {
		if candidate == name {
			return i
		}
	}
	return -1
}

func artifactIdentities(repoRoot, runID string, paths []string) ([]ArtifactIdentity, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	expectedPrefix := filepath.ToSlash(filepath.Join(".code-harness", "runs", runID)) + "/"
	out := make([]ArtifactIdentity, 0, len(paths))
	seen := map[string]bool{}
	for _, candidate := range paths {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || filepath.IsAbs(candidate) {
			return nil, fmt.Errorf("REVIEW_PROGRESS_ARTIFACT_PATH_INVALID: %q", candidate)
		}
		clean := filepath.ToSlash(filepath.Clean(candidate))
		if clean == "." || strings.HasPrefix(clean, "../") || !strings.HasPrefix(clean, expectedPrefix) {
			return nil, fmt.Errorf("REVIEW_PROGRESS_ARTIFACT_PATH_INVALID: %q", candidate)
		}
		if seen[clean] {
			continue
		}
		data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(clean)))
		if err != nil {
			return nil, fmt.Errorf("REVIEW_PROGRESS_ARTIFACT_READ_FAILED: %s: %w", clean, err)
		}
		digest := sha256.Sum256(data)
		out = append(out, ArtifactIdentity{Path: clean, SHA256: hex.EncodeToString(digest[:])})
		seen[clean] = true
	}
	return out, nil
}

func durationMS(startedAt string, now time.Time) int64 {
	started, err := time.Parse(time.RFC3339Nano, startedAt)
	if err != nil {
		return 0
	}
	ms := now.Sub(started).Milliseconds()
	if ms < 0 {
		return 0
	}
	return ms
}

func validate(state State) error {
	if state.Version != 1 || !runIDPattern.MatchString(state.RunID) {
		return errors.New("REVIEW_PROGRESS_INVALID: identity")
	}
	if state.StartedAt == "" || state.UpdatedAt == "" {
		return errors.New("REVIEW_PROGRESS_INVALID: timestamps")
	}
	if len(state.Stages) != len(canonicalStages) {
		return errors.New("REVIEW_PROGRESS_INVALID: stage count")
	}
	for i, expected := range canonicalStages {
		stage := state.Stages[i]
		if stage.Index != i+1 || stage.Total != len(canonicalStages) || stage.Name != expected {
			return errors.New("REVIEW_PROGRESS_INVALID: canonical stage order")
		}
		if !validStatus(stage.Status) {
			return fmt.Errorf("REVIEW_PROGRESS_INVALID: stage %s status %s", stage.Name, stage.Status)
		}
		for _, artifact := range stage.Artifacts {
			if artifact.Path == "" || len(artifact.SHA256) != 64 {
				return fmt.Errorf("REVIEW_PROGRESS_INVALID: stage %s artifact identity", stage.Name)
			}
		}
	}

	switch state.Status {
	case StatusRunning:
		idx := stageIndex(state.CurrentStage)
		if idx < 1 || state.TerminalStage != "" || state.FailureStage != "" || state.FailureCode != "" {
			return errors.New("REVIEW_PROGRESS_INVALID: running identity")
		}
		for i, stage := range state.Stages {
			expected := StatusPending
			if i < idx {
				expected = StatusSucceeded
			} else if i == idx {
				expected = StatusRunning
			}
			if stage.Status != expected {
				return fmt.Errorf("REVIEW_PROGRESS_INVALID: running stage %s expected %s got %s", stage.Name, expected, stage.Status)
			}
		}
	case StatusSucceeded:
		if state.CurrentStage != StageReport || state.TerminalStage != StageReport || state.FailureStage != "" || state.FailureCode != "" {
			return errors.New("REVIEW_PROGRESS_INVALID: success terminal identity")
		}
		for _, stage := range state.Stages {
			if stage.Status != StatusSucceeded {
				return fmt.Errorf("REVIEW_PROGRESS_INVALID: terminal success stage %s", stage.Name)
			}
		}
	case StatusFailed:
		idx := stageIndex(state.FailureStage)
		if idx < 1 || state.CurrentStage != state.FailureStage || state.TerminalStage != state.FailureStage || !failureCodePattern.MatchString(state.FailureCode) {
			return errors.New("REVIEW_PROGRESS_INVALID: failure terminal identity")
		}
		for i, stage := range state.Stages {
			expected := StatusBlocked
			if i < idx {
				expected = StatusSucceeded
			} else if i == idx {
				expected = StatusFailed
			}
			if stage.Status != expected {
				return fmt.Errorf("REVIEW_PROGRESS_INVALID: failed stage %s expected %s got %s", stage.Name, expected, stage.Status)
			}
		}
	default:
		return fmt.Errorf("REVIEW_PROGRESS_INVALID: run status %s", state.Status)
	}

	for i, event := range state.Events {
		if event.Sequence != i+1 || event.Total != len(canonicalStages) || event.Index < 1 || event.Index > len(canonicalStages) || canonicalStages[event.Index-1] != event.Stage {
			return errors.New("REVIEW_PROGRESS_INVALID: event identity/order")
		}
		if event.Status != StatusRunning && event.Status != StatusSucceeded && event.Status != StatusFailed {
			return errors.New("REVIEW_PROGRESS_INVALID: event status")
		}
		if event.Status == StatusFailed && !failureCodePattern.MatchString(event.FailureCode) {
			return errors.New("REVIEW_PROGRESS_INVALID: event failure code")
		}
		if event.Display == "" || event.Timestamp == "" {
			return errors.New("REVIEW_PROGRESS_INVALID: event observability")
		}
	}
	return nil
}

func validStatus(status string) bool {
	switch status {
	case StatusPending, StatusRunning, StatusSucceeded, StatusFailed, StatusBlocked:
		return true
	default:
		return false
	}
}

func persist(repoRoot, runID string, state State) error {
	rel, err := Path(runID)
	if err != nil {
		return err
	}
	if err := writeState(filepath.Join(repoRoot, rel), state, true); err != nil {
		return fmt.Errorf("REVIEW_PROGRESS_WRITE_FAILED: %w", err)
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
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if replace {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	return nil
}
