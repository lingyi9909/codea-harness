package analysis

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	certifyPerformanceSchemaVersion164 = 1
	certifyPerformanceComplete164      = "COMPLETE"
	certifyPerformanceFailed164        = "FAILED"
)

var certifyPerformanceErrorCode164 = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

type certifyPerformanceCounts164 struct {
	ChangedFiles           int `json:"changedFiles"`
	ProductionJavaCurrent  int `json:"productionJavaCurrent"`
	ProductionJavaBase     int `json:"productionJavaBase"`
	ControllerFilesCurrent int `json:"controllerFilesCurrent"`
	ControllerFilesBase    int `json:"controllerFilesBase"`
}

type certifyPerformanceTiming164 struct {
	SnapshotFreshness       int64 `json:"snapshotFreshness"`
	SnapshotSchemaAndDecode int64 `json:"snapshotSchemaAndDecode"`
	ProposalSchema          int64 `json:"proposalSchema"`
	AnalysisAssembly        int64 `json:"analysisAssembly"`
	EntrypointInventory     int64 `json:"entrypointInventory"`
	CurrentTypeAst          int64 `json:"currentTypeAst"`
	CurrentMethodAst        int64 `json:"currentMethodAst"`
	BaseSourceLoad          int64 `json:"baseSourceLoad"`
	BaseTypeAst             int64 `json:"baseTypeAst"`
	BaseMethodAst           int64 `json:"baseMethodAst"`
	EntrypointVerification  int64 `json:"entrypointVerification"`
	EvidenceValidation      int64 `json:"evidenceValidation"`
	CoverageValidation      int64 `json:"coverageValidation"`
	Publish                 int64 `json:"publish"`
	Total                   int64 `json:"total"`
}

type certifyPerformanceProcesses164 struct {
	CurrentTypeAst   int `json:"currentTypeAst"`
	CurrentMethodAst int `json:"currentMethodAst"`
	BaseTypeAst      int `json:"baseTypeAst"`
	BaseMethodAst    int `json:"baseMethodAst"`
	BaseCatFile      int `json:"baseCatFile"`
}

type certifyPerformance164 struct {
	SchemaVersion  int                            `json:"schemaVersion"`
	RunID          string                         `json:"runId"`
	SnapshotSHA256 string                         `json:"snapshotSha256"`
	Status         string                         `json:"status"`
	ErrorCode      string                         `json:"errorCode,omitempty"`
	Counts         certifyPerformanceCounts164    `json:"counts"`
	TimingMS       certifyPerformanceTiming164    `json:"timingMs"`
	Processes      certifyPerformanceProcesses164 `json:"processes"`
}

type certifyPerformanceRecorder164 struct {
	root    string
	started time.Time
	doc     certifyPerformance164
}

func newCertifyPerformanceRecorder164(root, runID, snapshotSHA256 string) *certifyPerformanceRecorder164 {
	return &certifyPerformanceRecorder164{
		root:    root,
		started: time.Now(),
		doc: certifyPerformance164{
			SchemaVersion:  certifyPerformanceSchemaVersion164,
			RunID:          runID,
			SnapshotSHA256: strings.TrimSpace(snapshotSHA256),
		},
	}
}

func (r *certifyPerformanceRecorder164) finish(formalErr error) {
	if r == nil {
		return
	}
	r.doc.TimingMS.Total = elapsedMillis164(r.started)
	if formalErr != nil {
		r.doc.Status = certifyPerformanceFailed164
		r.doc.ErrorCode = formalErrorCode164(formalErr)
	} else {
		r.doc.Status = certifyPerformanceComplete164
		r.doc.ErrorCode = ""
	}
	_ = writeCertifyPerformance164(r.root, r.doc)
}

func writeCertifyPerformance164(root string, doc certifyPerformance164) error {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	analysisDir := filepath.Join(root, ".code-harness", "runs", doc.RunID, "analysis")
	if err := os.MkdirAll(analysisDir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(analysisDir, ".certify-performance-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, filepath.Join(analysisDir, "certify-performance.json"))
}

func formalErrorCode164(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if idx := strings.IndexByte(message, ':'); idx >= 0 {
		message = strings.TrimSpace(message[:idx])
	} else if idx := strings.IndexAny(message, " \t\r\n"); idx >= 0 {
		message = strings.TrimSpace(message[:idx])
	}
	if certifyPerformanceErrorCode164.MatchString(message) {
		return message
	}
	return "ANALYSIS_CERTIFY_FAILED"
}

func elapsedMillis164(started time.Time) int64 {
	if started.IsZero() {
		return 0
	}
	elapsed := time.Since(started)
	if elapsed < 0 {
		return 0
	}
	ms := elapsed.Milliseconds()
	if elapsed > 0 && ms == 0 {
		return 1
	}
	return ms
}
