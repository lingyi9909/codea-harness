package reviewrun

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type resultEnvelope struct {
	SchemaVersion    int       `json:"schemaVersion"`
	RunID            string    `json:"runId"`
	RequestSHA256    string    `json:"requestSha256"`
	ReviewConclusion string    `json:"reviewConclusion"`
	Coverage         string    `json:"coverage"`
	Reads            []ReadRef `json:"reads"`
	Findings         []Finding `json:"findings"`
	PendingRisks     []string  `json:"pendingRisks"`
	Gaps             []string  `json:"gaps"`
}

func Finish(ctx context.Context, root string, req FinishRequest) (Outcome, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateRunID(req.RunID); err != nil {
		return Outcome{}, err
	}
	normalizeFinishRequest(&req)
	runDir, state, err := loadRun(root, req.RunID)
	if err != nil {
		return Outcome{}, err
	}
	unlock, err := acquireRunLock(ctx, runDir)
	if err != nil {
		return Outcome{}, err
	}
	defer unlock()

	_, state, err = loadRun(root, req.RunID)
	if err != nil {
		return Outcome{}, err
	}
	if state.Cancelled {
		return Outcome{}, fmt.Errorf("REVIEW_FINISH_CANCELLED")
	}
	if !state.ScopeReady {
		return Outcome{}, fmt.Errorf("REVIEW_FINISH_SCOPE_NOT_READY")
	}

	scope, err := loadScope180(runDir, req.RunID)
	if err != nil {
		state.LastError = err.Error()
		_ = writeState(runDir, state)
		return Outcome{}, err
	}
	if err := validateFinishReadsWithinScope180(req.Reads, scope.Reads); err != nil {
		state.LastError = err.Error()
		_ = writeState(runDir, state)
		return Outcome{}, err
	}
	for _, chain := range scope.Chains {
		req.Gaps = append(req.Gaps, chain.Unresolved...)
	}
	req.Gaps = uniqueStrings180(req.Gaps)
	coverage := stateCoverage(state)
	if coverage != "COMPLETE" && coverage != "PARTIAL" {
		err := fmt.Errorf("REVIEW_FINISH_COVERAGE_INVALID: %q", coverage)
		state.LastError = err.Error()
		_ = writeState(runDir, state)
		return Outcome{}, err
	}
	if scope.Coverage == "PARTIAL" || len(req.Gaps) > 0 {
		coverage = "PARTIAL"
	}
	if coverage == "PARTIAL" && len(state.SelectedIDs) == 0 {
		err := fmt.Errorf("REVIEW_FINISH_PARTIAL_SCOPE_NOT_ACCEPTED")
		state.LastError = err.Error()
		_ = writeState(runDir, state)
		return Outcome{}, err
	}

	if err := validateFinishRequest(root, req); err != nil {
		state.LastError = err.Error()
		_ = writeState(runDir, state)
		return Outcome{}, err
	}
	if err := validateFindingsWithinSelectedScope180(root, req.Findings, scope.Chains); err != nil {
		state.LastError = err.Error()
		_ = writeState(runDir, state)
		return Outcome{}, err
	}
	if err := validateChangeAttribution180(ctx, root, runDir, req); err != nil {
		state.LastError = err.Error()
		_ = writeState(runDir, state)
		return Outcome{}, err
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return Outcome{}, fmt.Errorf("REVIEW_FINISH_REQUEST_ENCODE_FAILED: %w", err)
	}
	requestSHA := bytesSHA256(reqBytes)

	existing, _, existingErr := loadExistingResult(runDir)
	if existingErr == nil {
		if existing.RequestSHA256 != requestSHA {
			return Outcome{}, fmt.Errorf("REVIEW_FINISH_OVERWRITE_REJECTED")
		}
		status, statusErr := Status(root, req.RunID)
		if statusErr == nil && status.Execution == "COMPLETE" {
			return status, nil
		}
	} else if !os.IsNotExist(existingErr) {
		return Outcome{}, existingErr
	}

	conclusion := conclusionFor(coverage, req)
	result := resultEnvelope{
		SchemaVersion:    SchemaVersion,
		RunID:            req.RunID,
		RequestSHA256:    requestSHA,
		ReviewConclusion: conclusion,
		Coverage:         coverage,
		Reads:            req.Reads,
		Findings:         req.Findings,
		PendingRisks:     req.PendingRisks,
		Gaps:             req.Gaps,
	}
	resultBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return Outcome{}, fmt.Errorf("REVIEW_RESULT_ENCODE_FAILED: %w", err)
	}
	resultBytes = append(resultBytes, '\n')
	resultPath := filepath.Join(runDir, "result.json")
	if existingErr != nil {
		if err := atomicWrite(resultPath, resultBytes); err != nil {
			state.LastError = "REVIEW_RESULT_WRITE_FAILED: " + err.Error()
			_ = writeState(runDir, state)
			return Outcome{}, fmt.Errorf("REVIEW_RESULT_WRITE_FAILED: %w", err)
		}
	} else {
		disk, err := os.ReadFile(resultPath)
		if err != nil {
			return Outcome{}, fmt.Errorf("REVIEW_RESULT_READ_FAILED: %w", err)
		}
		resultBytes = disk
	}
	resultSHA := bytesSHA256(resultBytes)

	reportIntent := Intent{}
	reportChains := append([]Chain{}, scope.Chains...)
	if prepared, preparedErr := loadPreparedOptions180(runDir); preparedErr == nil {
		reportIntent = prepared.Intent
		reportChains = append([]Chain{}, prepared.Options.Chains...)
	}

	report, err := renderReport(reportView{
		StatusText:       "评审完成",
		RunID:            req.RunID,
		Project:          state.Project,
		ReportPath:       state.ReportPath,
		CreatedAt:        state.CreatedAt,
		CompletedAt:      time.Now().UTC().Format(time.RFC3339),
		Execution:        "COMPLETE",
		ReviewConclusion: conclusion,
		Coverage:         coverage,
		ResultSHA256:     resultSHA,
		Findings:         req.Findings,
		PendingRisks:     req.PendingRisks,
		Gaps:             req.Gaps,
		Intent:           reportIntent,
		Chains:           reportChains,
		SelectedIDs:      append([]string{}, scope.SelectedIDs...),
		ReadFileCount:    countReadFiles180(scope.Reads),
	})
	if err != nil {
		return Outcome{}, err
	}
	if err := atomicWrite(state.ReportPath, report); err != nil {
		state.LastError = "REVIEW_REPORT_WRITE_FAILED: " + err.Error()
		_ = writeState(runDir, state)
		return Outcome{}, fmt.Errorf("REVIEW_REPORT_WRITE_FAILED: %w", err)
	}
	readback, err := os.ReadFile(state.ReportPath)
	if err != nil {
		return Outcome{}, fmt.Errorf("REVIEW_REPORT_READBACK_FAILED: %w", err)
	}
	meta, err := parseReportMeta(readback)
	if err != nil || meta.RunID != req.RunID || meta.Execution != "COMPLETE" || meta.ResultSHA256 != resultSHA || !bytes.Equal(readback, report) {
		if err == nil {
			err = fmt.Errorf("report content or metadata mismatch")
		}
		return Outcome{}, fmt.Errorf("REVIEW_REPORT_VERIFY_FAILED: %w", err)
	}
	reportSHA := bytesSHA256(readback)
	state.ReportSHA256 = reportSHA
	state.Coverage = coverage
	state.LastError = ""
	if err := writeState(runDir, state); err != nil {
		return Outcome{}, fmt.Errorf("REVIEW_RUN_FINAL_STATE_WRITE_FAILED: %w", err)
	}
	return Outcome{RunID: req.RunID, Execution: "COMPLETE", ReviewConclusion: conclusion, Coverage: coverage, ReportPath: state.ReportPath, ReportSHA256: reportSHA}, nil
}

func loadScope180(runDir, runID string) (scopeState180, error) {
	data, err := os.ReadFile(filepath.Join(runDir, "scope.json"))
	if err != nil {
		return scopeState180{}, fmt.Errorf("REVIEW_FINISH_SCOPE_READ_FAILED: %w", err)
	}
	var scope scopeState180
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&scope); err != nil {
		return scopeState180{}, fmt.Errorf("REVIEW_FINISH_SCOPE_INVALID: %w", err)
	}
	if scope.SchemaVersion != SchemaVersion || scope.RunID != runID || (scope.Coverage != "COMPLETE" && scope.Coverage != "PARTIAL") {
		return scopeState180{}, fmt.Errorf("REVIEW_FINISH_SCOPE_INVALID")
	}
	return scope, nil
}

func validateFinishReadsWithinScope180(reads, allowed []ReadRef) error {
	byPath := map[string][]ReadRef{}
	for _, ref := range allowed {
		key := filepath.ToSlash(filepath.Clean(ref.Path))
		byPath[key] = append(byPath[key], ref)
	}
	for _, ref := range reads {
		key := filepath.ToSlash(filepath.Clean(ref.Path))
		ok := false
		for _, scopeRef := range byPath[key] {
			if ref.SHA256 == scopeRef.SHA256 && ref.StartLine >= scopeRef.StartLine && ref.EndLine <= scopeRef.EndLine {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("REVIEW_FINISH_READ_OUTSIDE_SCOPE: %s", ref.Path)
		}
	}
	for _, scopeRef := range allowed {
		covered := false
		for _, ref := range reads {
			if filepath.ToSlash(filepath.Clean(ref.Path)) == filepath.ToSlash(filepath.Clean(scopeRef.Path)) &&
				ref.SHA256 == scopeRef.SHA256 && ref.StartLine <= scopeRef.StartLine && ref.EndLine >= scopeRef.EndLine {
				covered = true
				break
			}
		}
		if !covered {
			return fmt.Errorf("REVIEW_FINISH_SCOPE_NOT_READ: %s", scopeRef.Path)
		}
	}
	return nil
}

func loadExistingResult(runDir string) (resultEnvelope, []byte, error) {
	data, err := os.ReadFile(filepath.Join(runDir, "result.json"))
	if err != nil {
		return resultEnvelope{}, nil, err
	}
	var result resultEnvelope
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&result); err != nil {
		return resultEnvelope{}, nil, fmt.Errorf("REVIEW_RESULT_INVALID: %w", err)
	}
	if result.SchemaVersion != SchemaVersion || result.RunID == "" || result.RequestSHA256 == "" {
		return resultEnvelope{}, nil, fmt.Errorf("REVIEW_RESULT_INVALID")
	}
	return result, data, nil
}

func normalizeFinishRequest(req *FinishRequest) {
	if req.Reads == nil {
		req.Reads = []ReadRef{}
	}
	if req.Findings == nil {
		req.Findings = []Finding{}
	}
	if req.PendingRisks == nil {
		req.PendingRisks = []string{}
	}
	if req.Gaps == nil {
		req.Gaps = []string{}
	}
	for i := range req.Findings {
		if req.Findings[i].Evidence == nil {
			req.Findings[i].Evidence = []Evidence{}
		}
	}
}

func validateFinishRequest(root string, req FinishRequest) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("REVIEW_FINISH_ROOT_FAILED: %w", err)
	}
	reads := make(map[string]ReadRef, len(req.Reads))
	for _, ref := range req.Reads {
		key, err := validateReadRef(rootAbs, ref)
		if err != nil {
			return err
		}
		if _, exists := reads[key]; exists {
			return fmt.Errorf("REVIEW_FINISH_DUPLICATE_READ: %s", ref.Path)
		}
		reads[key] = ref
	}
	seenFinding := map[string]bool{}
	for _, finding := range req.Findings {
		if strings.TrimSpace(finding.ID) == "" || seenFinding[finding.ID] {
			return fmt.Errorf("REVIEW_FINISH_FINDING_ID_INVALID")
		}
		seenFinding[finding.ID] = true
		switch finding.Severity {
		case "CRITICAL", "HIGH", "MEDIUM", "LOW":
		default:
			return fmt.Errorf("REVIEW_FINISH_SEVERITY_INVALID: %q", finding.Severity)
		}
		if strings.TrimSpace(finding.Problem) == "" || strings.TrimSpace(finding.Impact) == "" || strings.TrimSpace(finding.Recommendation) == "" || strings.TrimSpace(finding.Verification) == "" {
			return fmt.Errorf("REVIEW_FINISH_FINDING_FIELDS_REQUIRED: %s", finding.ID)
		}
		if len(finding.Evidence) == 0 {
			return fmt.Errorf("REVIEW_FINISH_EVIDENCE_REQUIRED: %s", finding.ID)
		}
		for _, ev := range finding.Evidence {
			if !evidenceReadDeclared180(ev.Ref, req.Reads) {
				return fmt.Errorf("REVIEW_FINISH_EVIDENCE_READ_NOT_DECLARED: %s", ev.Ref.Path)
			}
			if strings.TrimSpace(ev.Quote) == "" {
				return fmt.Errorf("REVIEW_FINISH_EVIDENCE_QUOTE_REQUIRED: %s", finding.ID)
			}
			if err := verifyEvidenceQuote(rootAbs, ev); err != nil {
				return err
			}
		}
	}
	return nil
}

func evidenceReadDeclared180(evidence ReadRef, reads []ReadRef) bool {
	if evidence.StartLine < 1 || evidence.EndLine < evidence.StartLine {
		return false
	}
	path := filepath.ToSlash(filepath.Clean(evidence.Path))
	for _, ref := range reads {
		if filepath.ToSlash(filepath.Clean(ref.Path)) == path &&
			ref.SHA256 == evidence.SHA256 &&
			evidence.StartLine >= ref.StartLine &&
			evidence.EndLine <= ref.EndLine {
			return true
		}
	}
	return false
}

func validateReadRef(rootAbs string, ref ReadRef) (string, error) {
	if strings.TrimSpace(ref.Path) == "" || filepath.IsAbs(ref.Path) {
		return "", fmt.Errorf("REVIEW_FINISH_READ_PATH_INVALID: %q", ref.Path)
	}
	clean := filepath.Clean(filepath.FromSlash(ref.Path))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("REVIEW_FINISH_READ_PATH_INVALID: %q", ref.Path)
	}
	abs := filepath.Join(rootAbs, clean)
	rel, err := filepath.Rel(rootAbs, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("REVIEW_FINISH_READ_PATH_INVALID: %q", ref.Path)
	}
	if strings.HasPrefix(filepath.ToSlash(rel), ".code-harness/runs/") {
		return "", fmt.Errorf("REVIEW_FINISH_READ_PATH_INVALID: %q", ref.Path)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", fmt.Errorf("REVIEW_FINISH_READ_FAILED: %s: %w", ref.Path, err)
	}
	if ref.SHA256 != bytesSHA256(data) {
		return "", fmt.Errorf("REVIEW_FINISH_READ_STALE: %s", ref.Path)
	}
	lines := bytes.Split(data, []byte("\n"))
	if ref.StartLine < 1 || ref.EndLine < ref.StartLine || ref.EndLine > len(lines) {
		return "", fmt.Errorf("REVIEW_FINISH_READ_RANGE_INVALID: %s", ref.Path)
	}
	return readKey(ref), nil
}

func verifyEvidenceQuote(rootAbs string, ev Evidence) error {
	data, err := os.ReadFile(filepath.Join(rootAbs, filepath.Clean(filepath.FromSlash(ev.Ref.Path))))
	if err != nil {
		return err
	}
	lines := bytes.Split(data, []byte("\n"))
	segment := evidenceVisibleSegment180(lines, ev.Ref.StartLine, ev.Ref.EndLine)
	if !bytes.Contains(segment, normalizeEvidenceQuote180(ev.Quote)) {
		return fmt.Errorf("REVIEW_FINISH_EVIDENCE_QUOTE_MISMATCH: %s", ev.Ref.Path)
	}
	return nil
}

func readKey(ref ReadRef) string {
	return fmt.Sprintf("%s|%s|%d|%d", filepath.ToSlash(filepath.Clean(ref.Path)), ref.SHA256, ref.StartLine, ref.EndLine)
}

func conclusionFor(coverage string, req FinishRequest) string {
	if coverage != "COMPLETE" {
		return "UNDETERMINED"
	}
	severities := make([]string, 0, len(req.Findings))
	for _, finding := range req.Findings {
		severities = append(severities, finding.Severity)
	}
	sort.Strings(severities)
	for _, s := range severities {
		if s == "CRITICAL" || s == "HIGH" {
			return "BLOCKING"
		}
	}
	if len(req.Findings) > 0 || len(req.PendingRisks) > 0 {
		return "ACTION_REQUIRED"
	}
	return "NO_ISSUES_FOUND"
}
