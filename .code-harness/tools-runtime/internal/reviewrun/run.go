package reviewrun

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type runState struct {
	SchemaVersion int      `json:"schemaVersion"`
	RunID         string   `json:"runId"`
	CreatedAt     string   `json:"createdAt"`
	Project       string   `json:"project"`
	ReportPath    string   `json:"reportPath"`
	ReportSHA256  string   `json:"reportSha256,omitempty"`
	ScopeReady    bool     `json:"scopeReady"`
	Coverage      string   `json:"coverage"`
	SelectedIDs   []string `json:"selectedIds"`
	Cancelled     bool     `json:"cancelled"`
	CancelReason  string   `json:"cancelReason"`
	LastError     string   `json:"lastError"`
}

func Start(root string) (Outcome, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return Outcome{}, fmt.Errorf("REVIEW_START_ROOT_FAILED: %w", err)
	}
	runsRoot := filepath.Join(rootAbs, ".code-harness", "runs")
	if err := os.MkdirAll(runsRoot, 0o755); err != nil {
		return Outcome{}, fmt.Errorf("REVIEW_START_RUNS_DIR_FAILED: %w", err)
	}

	for attempt := 0; attempt < 32; attempt++ {
		runID, err := newRunID()
		if err != nil {
			return Outcome{}, err
		}
		runDir := filepath.Join(runsRoot, runID)
		if err := os.Mkdir(runDir, 0o755); err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return Outcome{}, fmt.Errorf("REVIEW_START_RUN_DIR_FAILED: %s: %w", runDir, err)
		}

		reportPath := filepath.Join(runDir, "review.md")
		now := time.Now().UTC().Format(time.RFC3339)
		state := runState{
			SchemaVersion: SchemaVersion,
			RunID:         runID,
			CreatedAt:     now,
			Project:       filepath.Base(rootAbs),
			ReportPath:    reportPath,
			Coverage:      "PARTIAL",
			SelectedIDs:   []string{},
		}
		if err := writeState(runDir, state); err != nil {
			_ = os.RemoveAll(runDir)
			return Outcome{}, err
		}
		report, err := renderReport(reportView{
			StatusText:       "评审未完成",
			RunID:            runID,
			Project:          state.Project,
			ReportPath:       reportPath,
			CreatedAt:        now,
			Execution:        "INCOMPLETE",
			ReviewConclusion: "UNDETERMINED",
			Coverage:         "PARTIAL",
		})
		if err != nil {
			_ = os.RemoveAll(runDir)
			return Outcome{}, err
		}
		if err := atomicWrite(reportPath, report); err != nil {
			_ = os.RemoveAll(runDir)
			return Outcome{}, fmt.Errorf("REVIEW_START_REPORT_WRITE_FAILED: %s: %w", reportPath, err)
		}
		verified, err := os.ReadFile(reportPath)
		if err != nil {
			_ = os.RemoveAll(runDir)
			return Outcome{}, fmt.Errorf("REVIEW_START_REPORT_READBACK_FAILED: %s: %w", reportPath, err)
		}
		meta, err := parseReportMeta(verified)
		if err != nil || meta.RunID != runID || meta.Execution != "INCOMPLETE" {
			_ = os.RemoveAll(runDir)
			if err == nil {
				err = fmt.Errorf("unexpected report metadata")
			}
			return Outcome{}, fmt.Errorf("REVIEW_START_REPORT_VERIFY_FAILED: %w", err)
		}
		return Outcome{
			RunID:            runID,
			Execution:        "INCOMPLETE",
			ReviewConclusion: "UNDETERMINED",
			Coverage:         "PARTIAL",
			ReportPath:       reportPath,
			ReportSHA256:     bytesSHA256(verified),
		}, nil
	}
	return Outcome{}, fmt.Errorf("REVIEW_START_RUN_ID_EXHAUSTED")
}

func Status(root, runID string) (Outcome, error) {
	runDir, state, err := loadRun(root, runID)
	if err != nil {
		return Outcome{}, err
	}
	report, err := os.ReadFile(state.ReportPath)
	if err != nil {
		return Outcome{RunID: runID, Execution: "INCOMPLETE", ReviewConclusion: "UNDETERMINED", Coverage: stateCoverage(state), ReportPath: state.ReportPath}, nil
	}
	reportSHA := bytesSHA256(report)
	meta, err := parseReportMeta(report)
	if err != nil || meta.RunID != runID {
		return Outcome{RunID: runID, Execution: "INCOMPLETE", ReviewConclusion: "UNDETERMINED", Coverage: stateCoverage(state), ReportPath: state.ReportPath, ReportSHA256: reportSHA}, nil
	}
	if meta.Execution == "CANCELLED" || state.Cancelled {
		return Outcome{RunID: runID, Execution: "CANCELLED", ReviewConclusion: "UNDETERMINED", Coverage: stateCoverage(state), ReportPath: state.ReportPath, ReportSHA256: reportSHA}, nil
	}
	if meta.Execution != "COMPLETE" || meta.ResultSHA256 == "" || state.ReportSHA256 == "" || state.ReportSHA256 != reportSHA {
		return Outcome{RunID: runID, Execution: "INCOMPLETE", ReviewConclusion: "UNDETERMINED", Coverage: stateCoverage(state), ReportPath: state.ReportPath, ReportSHA256: reportSHA}, nil
	}

	resultBytes, err := os.ReadFile(filepath.Join(runDir, "result.json"))
	if err != nil || bytesSHA256(resultBytes) != meta.ResultSHA256 {
		return Outcome{RunID: runID, Execution: "INCOMPLETE", ReviewConclusion: "UNDETERMINED", Coverage: stateCoverage(state), ReportPath: state.ReportPath, ReportSHA256: reportSHA}, nil
	}
	var result resultEnvelope
	if err := json.Unmarshal(resultBytes, &result); err != nil || result.RunID != runID {
		return Outcome{RunID: runID, Execution: "INCOMPLETE", ReviewConclusion: "UNDETERMINED", Coverage: stateCoverage(state), ReportPath: state.ReportPath, ReportSHA256: reportSHA}, nil
	}
	return Outcome{RunID: runID, Execution: "COMPLETE", ReviewConclusion: result.ReviewConclusion, Coverage: result.Coverage, ReportPath: state.ReportPath, ReportSHA256: reportSHA}, nil
}

func Cancel(root, runID, reason string) (Outcome, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	runDir, state, err := loadRun(root, runID)
	if err != nil {
		return Outcome{}, err
	}
	unlock, err := acquireRunLock(ctx, runDir)
	if err != nil {
		return Outcome{}, err
	}
	defer unlock()
	_, state, err = loadRun(root, runID)
	if err != nil {
		return Outcome{}, err
	}
	status, err := Status(root, runID)
	if err == nil && status.Execution == "COMPLETE" {
		return Outcome{}, fmt.Errorf("REVIEW_CANCEL_ALREADY_COMPLETE")
	}
	state.Cancelled = true
	state.CancelReason = strings.TrimSpace(reason)
	state.LastError = ""
	if err := writeState(runDir, state); err != nil {
		return Outcome{}, err
	}
	report, err := renderReport(reportView{StatusText: "评审已取消", RunID: runID, Project: state.Project, ReportPath: state.ReportPath, CreatedAt: state.CreatedAt, Execution: "CANCELLED", ReviewConclusion: "UNDETERMINED", Coverage: stateCoverage(state), CancelReason: state.CancelReason})
	if err != nil {
		return Outcome{}, err
	}
	if err := atomicWrite(state.ReportPath, report); err != nil {
		return Outcome{}, fmt.Errorf("REVIEW_CANCEL_REPORT_WRITE_FAILED: %w", err)
	}
	return Status(root, runID)
}

func loadRun(root, runID string) (string, runState, error) {
	if err := validateRunID(runID); err != nil {
		return "", runState{}, err
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", runState{}, fmt.Errorf("REVIEW_ROOT_FAILED: %w", err)
	}
	runDir := filepath.Join(rootAbs, ".code-harness", "runs", runID)
	data, err := os.ReadFile(filepath.Join(runDir, "run.json"))
	if err != nil {
		return "", runState{}, fmt.Errorf("REVIEW_RUN_READ_FAILED: %w", err)
	}
	var state runState
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&state); err != nil {
		return "", runState{}, fmt.Errorf("REVIEW_RUN_INVALID: %w", err)
	}
	if state.SchemaVersion != SchemaVersion || state.RunID != runID {
		return "", runState{}, fmt.Errorf("REVIEW_RUN_INVALID")
	}
	state.ReportPath = filepath.Join(runDir, "review.md")
	return runDir, state, nil
}

func writeState(runDir string, state runState) error {
	if state.SelectedIDs == nil {
		state.SelectedIDs = []string{}
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("REVIEW_RUN_ENCODE_FAILED: %w", err)
	}
	data = append(data, '\n')
	if err := atomicWrite(filepath.Join(runDir, "run.json"), data); err != nil {
		return fmt.Errorf("REVIEW_RUN_WRITE_FAILED: %w", err)
	}
	return nil
}

func newRunID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("REVIEW_START_RANDOM_FAILED: %w", err)
	}
	return "review-" + hex.EncodeToString(b[:]), nil
}

func validateRunID(runID string) error {
	if !strings.HasPrefix(runID, "review-") || len(runID) != len("review-")+32 {
		return fmt.Errorf("REVIEW_RUN_ID_INVALID: %q", runID)
	}
	for _, r := range runID[len("review-"):] {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return fmt.Errorf("REVIEW_RUN_ID_INVALID: %q", runID)
		}
	}
	return nil
}

func stateCoverage(state runState) string {
	if state.Coverage == "COMPLETE" {
		return "COMPLETE"
	}
	return "PARTIAL"
}

var createTemp = os.CreateTemp
var replaceFile = os.Rename

func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := createTemp(filepath.Dir(path), ".review-180-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := replaceFile(tmpPath, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

const defaultRunLockWait = 2 * time.Second

func acquireRunLock(ctx context.Context, runDir string) (func(), error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultRunLockWait)
		defer cancel()
	}

	lockPath := filepath.Join(runDir, ".review-180.lock")
	file, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("REVIEW_RUN_LOCK_FAILED: %w", err)
	}

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = file.Close()
			return nil, fmt.Errorf("REVIEW_RUN_BUSY: %w", ctx.Err())
		default:
		}

		acquired, lockErr := tryRunFileLock(file)
		if lockErr != nil {
			_ = file.Close()
			return nil, fmt.Errorf("REVIEW_RUN_LOCK_FAILED: %w", lockErr)
		}
		if acquired {
			var once sync.Once
			return func() {
				once.Do(func() {
					_ = unlockRunFile(file)
					_ = file.Close()
				})
			}, nil
		}

		select {
		case <-ctx.Done():
			_ = file.Close()
			return nil, fmt.Errorf("REVIEW_RUN_BUSY: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}
