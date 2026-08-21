package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Result string

const (
	ResultPassed               Result = "PASSED"
	ResultFailed               Result = "FAILED"
	ResultManualActionRequired Result = "MANUAL_ACTION_REQUIRED"
)

type ReviewRequest struct {
	RunID          string         `json:"runId"`
	HarnessVersion string         `json:"harnessVersion"`
	BaseRef        string         `json:"baseRef"`
	Head           string         `json:"head"`
	Result         Result         `json:"result"`
	Scope          ReviewScope    `json:"reviewScope"`
	Coverage       ReviewCoverage `json:"reviewCoverage"`
	Findings       []Finding      `json:"findings"`
}

type ReviewScope struct {
	ChangedFiles []string `json:"changedFiles"`
}

type ReviewCoverage struct {
	ReviewedFiles        []string `json:"reviewedFiles"`
	CallChain            []string `json:"callChain"`
	ExternalDependencies []string `json:"externalDependencies"`
	Unresolved           []string `json:"unresolved"`
	MissingReviewedFiles []string `json:"missingReviewedFiles"`
	RuntimeErrors        []string `json:"runtimeErrors"`
	Status               string   `json:"status"`
}

type Finding struct {
	ID             string  `json:"id"`
	Severity       string  `json:"severity"`
	File           string  `json:"file"`
	Line           int     `json:"line,omitempty"`
	Problem        string  `json:"problem"`
	Evidence       string  `json:"evidence"`
	Impact         string  `json:"impact"`
	Recommendation string  `json:"recommendation"`
	NeedsTest      bool    `json:"needsTest"`
	Confidence     float64 `json:"confidence"`
}

func DecodeReviewRequest(data []byte) (ReviewRequest, error) {
	var req ReviewRequest
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return ReviewRequest{}, fmt.Errorf("decode review report request: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return ReviewRequest{}, errors.New("decode review report request: multiple JSON values are not allowed")
		}
		return ReviewRequest{}, fmt.Errorf("decode review report request: %w", err)
	}
	if err := Validate(req); err != nil {
		return ReviewRequest{}, err
	}
	return req, nil
}

func Validate(req ReviewRequest) error {
	if !validArtifactID(req.RunID) {
		return errors.New("invalid review report runId")
	}
	if strings.TrimSpace(req.HarnessVersion) == "" || strings.TrimSpace(req.BaseRef) == "" || strings.TrimSpace(req.Head) == "" {
		return errors.New("review report requires harnessVersion, baseRef, and head")
	}
	switch req.Result {
	case ResultPassed, ResultFailed, ResultManualActionRequired:
	default:
		return fmt.Errorf("invalid review result %q", req.Result)
	}
	switch req.Coverage.Status {
	case "COMPLETE", "PARTIAL":
	default:
		return fmt.Errorf("invalid review coverage status %q", req.Coverage.Status)
	}
	if req.Coverage.Status == "PARTIAL" && req.Result != ResultManualActionRequired {
		return errors.New("PARTIAL coverage requires MANUAL_ACTION_REQUIRED result")
	}
	for i, f := range req.Findings {
		if strings.TrimSpace(f.ID) == "" || strings.TrimSpace(f.File) == "" || strings.TrimSpace(f.Problem) == "" || strings.TrimSpace(f.Evidence) == "" || strings.TrimSpace(f.Impact) == "" || strings.TrimSpace(f.Recommendation) == "" {
			return fmt.Errorf("finding %d has missing required fields", i)
		}
		switch strings.ToUpper(f.Severity) {
		case "CRITICAL", "HIGH", "MEDIUM", "LOW":
		default:
			return fmt.Errorf("finding %q has invalid severity %q", f.ID, f.Severity)
		}
		if f.Line < 0 {
			return fmt.Errorf("finding %q has invalid line", f.ID)
		}
		if f.Confidence < 0 || f.Confidence > 1 {
			return fmt.Errorf("finding %q has invalid confidence", f.ID)
		}
	}
	return nil
}

func validArtifactID(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	for i := 0; i < len(value); i++ {
		b := value[i]
		if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '.' || b == '_' || b == '-' {
			continue
		}
		return false
	}
	return true
}

func Render(req ReviewRequest) (string, error) {
	if err := Validate(req); err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintln(&b, "# Code Review Report")
	fmt.Fprintf(&b, "Run ID: %s\n", singleLine(req.RunID))
	fmt.Fprintf(&b, "Harness Version: %s\n", singleLine(req.HarnessVersion))
	fmt.Fprintf(&b, "Base Ref: %s\n", singleLine(req.BaseRef))
	fmt.Fprintf(&b, "HEAD: %s\n", singleLine(req.Head))
	fmt.Fprintf(&b, "Result: %s\n", req.Result)

	fmt.Fprintln(&b, "## Review Scope")
	fmt.Fprintf(&b, "Changed Files: %d\n", len(req.Scope.ChangedFiles))
	for _, f := range req.Scope.ChangedFiles {
		fmt.Fprintf(&b, "- %s\n", singleLine(f))
	}

	fmt.Fprintln(&b, "## Review Coverage")
	fmt.Fprintln(&b, "Reviewed Files:")
	writeList(&b, req.Coverage.ReviewedFiles, "无")
	fmt.Fprintln(&b, "Call Chain:")
	if len(req.Coverage.CallChain) == 0 {
		fmt.Fprintln(&b, "无")
	} else {
		for i, symbol := range req.Coverage.CallChain {
			prefix := ""
			if i > 0 {
				prefix = "→ "
			}
			fmt.Fprintf(&b, "%s%s\n", prefix, singleLine(symbol))
		}
	}
	fmt.Fprintln(&b, "External Dependencies:")
	writeList(&b, req.Coverage.ExternalDependencies, "无")
	fmt.Fprintln(&b, "Unresolved:")
	unresolved := append([]string{}, req.Coverage.Unresolved...)
	for _, f := range req.Coverage.MissingReviewedFiles {
		unresolved = append(unresolved, "Missing reviewed file: "+f)
	}
	for _, e := range req.Coverage.RuntimeErrors {
		unresolved = append(unresolved, "Runtime Contract validation error: "+e)
	}
	writeList(&b, unresolved, "无")
	fmt.Fprintf(&b, "Coverage: %s\n", req.Coverage.Status)

	fmt.Fprintln(&b, "## Review Findings")
	if len(req.Findings) == 0 {
		fmt.Fprintln(&b, "无")
	} else {
		for _, f := range req.Findings {
			fmt.Fprintf(&b, "### %s %s\n", singleLine(f.ID), strings.ToUpper(f.Severity))
			fmt.Fprintln(&b, "File:")
			location := singleLine(f.File)
			if f.Line > 0 {
				location += ":" + strconv.Itoa(f.Line)
			}
			fmt.Fprintln(&b, location)
			writeSection(&b, "Problem", f.Problem)
			writeSection(&b, "Evidence", f.Evidence)
			writeSection(&b, "Impact", f.Impact)
			writeSection(&b, "Recommendation", f.Recommendation)
			fmt.Fprintln(&b, "Needs Test:")
			if f.NeedsTest {
				fmt.Fprintln(&b, "YES")
			} else {
				fmt.Fprintln(&b, "NO")
			}
			fmt.Fprintln(&b, "Confidence:")
			fmt.Fprintln(&b, strconv.FormatFloat(f.Confidence, 'f', -1, 64))
		}
	}

	counts := map[string]int{"CRITICAL": 0, "HIGH": 0, "MEDIUM": 0, "LOW": 0}
	for _, f := range req.Findings {
		counts[strings.ToUpper(f.Severity)]++
	}
	fmt.Fprintln(&b, "## Summary")
	fmt.Fprintf(&b, "Result: %s\n", req.Result)
	fmt.Fprintf(&b, "Findings: %d\n", len(req.Findings))
	fmt.Fprintf(&b, "Critical: %d\n", counts["CRITICAL"])
	fmt.Fprintf(&b, "High: %d\n", counts["HIGH"])
	fmt.Fprintf(&b, "Medium: %d\n", counts["MEDIUM"])
	fmt.Fprintf(&b, "Low: %d\n", counts["LOW"])
	return b.String(), nil
}

func Write(repoRoot string, req ReviewRequest) (string, error) {
	markdown, err := Render(req)
	if err != nil {
		return "", err
	}
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", err
	}
	runsRoot := filepath.Join(root, ".code-harness", "runs")
	runDir := filepath.Join(runsRoot, req.RunID)
	rel, err := filepath.Rel(runsRoot, runDir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("review report path escapes runs directory")
	}
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return "", fmt.Errorf("create review report directory: %w", err)
	}
	realRunsRoot, err := filepath.EvalSymlinks(runsRoot)
	if err != nil {
		return "", fmt.Errorf("resolve runs directory: %w", err)
	}
	realRunDir, err := filepath.EvalSymlinks(runDir)
	if err != nil {
		return "", fmt.Errorf("resolve review report directory: %w", err)
	}
	realRel, err := filepath.Rel(realRunsRoot, realRunDir)
	if err != nil || realRel == ".." || strings.HasPrefix(realRel, ".."+string(filepath.Separator)) {
		return "", errors.New("review report path escapes runs directory")
	}
	path := filepath.Join(runDir, "review.md")
	if err := os.WriteFile(path, []byte(markdown), 0o600); err != nil {
		return "", fmt.Errorf("write review report: %w", err)
	}
	return path, nil
}

func WriteRequestFile(repoRoot, inputPath string) (string, error) {
	if filepath.IsAbs(inputPath) || !strings.EqualFold(filepath.Ext(inputPath), ".json") {
		return "", errors.New("review report input must be a JSON request under .code-harness/runs/<runId>/requests")
	}
	cleanInput := filepath.Clean(inputPath)
	runsRootRelative := filepath.Clean(filepath.Join(".code-harness", "runs"))
	runsRel, err := filepath.Rel(runsRootRelative, cleanInput)
	if err != nil || runsRel == "." || runsRel == ".." || strings.HasPrefix(runsRel, ".."+string(filepath.Separator)) {
		return "", errors.New("review report input must be under .code-harness/runs/<runId>/requests")
	}
	data, err := os.ReadFile(cleanInput)
	if err != nil {
		return "", fmt.Errorf("read review report request: %w", err)
	}
	req, err := DecodeReviewRequest(data)
	if err != nil {
		return "", err
	}
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", err
	}
	inputAbs, err := filepath.Abs(cleanInput)
	if err != nil {
		return "", err
	}
	requestRoot := filepath.Join(root, ".code-harness", "runs", req.RunID, "requests")
	rel, err := filepath.Rel(requestRoot, inputAbs)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("review report input must be under .code-harness/runs/<runId>/requests")
	}
	realRequestRoot, err := filepath.EvalSymlinks(requestRoot)
	if err != nil {
		return "", fmt.Errorf("resolve review report request directory: %w", err)
	}
	realInput, err := filepath.EvalSymlinks(inputAbs)
	if err != nil {
		return "", fmt.Errorf("resolve review report input: %w", err)
	}
	realRel, err := filepath.Rel(realRequestRoot, realInput)
	if err != nil || realRel == "." || realRel == ".." || strings.HasPrefix(realRel, ".."+string(filepath.Separator)) {
		return "", errors.New("review report input must be under .code-harness/runs/<runId>/requests")
	}
	path, err := Write(root, req)
	if err != nil {
		return "", err
	}
	if err := os.Remove(inputAbs); err != nil {
		return "", fmt.Errorf("remove review report transport after success: %w", err)
	}
	return path, nil
}

func writeList(b *strings.Builder, values []string, empty string) {
	if len(values) == 0 {
		fmt.Fprintln(b, empty)
		return
	}
	for _, v := range values {
		fmt.Fprintf(b, "- %s\n", singleLine(v))
	}
}

func writeSection(b *strings.Builder, title, content string) {
	fmt.Fprintf(b, "%s:\n", title)
	fmt.Fprintln(b, strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n")))
}

func singleLine(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}
