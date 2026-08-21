package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleRequest() ReviewRequest {
	return ReviewRequest{
		RunID:          "review-20260820-001",
		HarnessVersion: "1.3.0",
		BaseRef:        "origin/develop",
		Head:           "abc123",
		Result:         ResultFailed,
		Scope:          ReviewScope{ChangedFiles: []string{"OrderController.java", "OrderServiceImpl.java", "OrderDTO.java"}},
		Coverage: ReviewCoverage{
			ReviewedFiles:        []string{"OrderController.java", "OrderServiceImpl.java", "OrderDTO.java"},
			CallChain:            []string{"OrderController.approve", "OrderServiceImpl.approve", "OrderRepository.updateStatus"},
			ExternalDependencies: []string{"PaymentRpcClient"},
			Status:               "COMPLETE",
		},
		Findings: []Finding{{ID: "F-001", Severity: "HIGH", File: "src/main/java/OrderServiceImpl.java", Line: 186, Problem: "状态更新前没有校验订单当前状态。", Evidence: "if (...) updateStatus", Impact: "非法状态可能被更新", Recommendation: "更新前校验状态", NeedsTest: true, Confidence: 0.96}},
	}
}

func TestR1PassedWritesReviewMarkdown(t *testing.T) {
	req := sampleRequest()
	req.Result = ResultPassed
	req.Findings = nil
	path, err := Write(t.TempDir(), req)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Result: PASSED") {
		t.Fatalf("unexpected markdown: %s", data)
	}
}

func TestR2FailedWritesFindings(t *testing.T) {
	md, err := Render(sampleRequest())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Result: FAILED", "### F-001 HIGH", "Problem:", "Needs Test:\nYES", "High: 1"} {
		if !strings.Contains(md, want) {
			t.Fatalf("missing %q in %s", want, md)
		}
	}
}

func TestR3PartialWritesManualActionRequired(t *testing.T) {
	req := sampleRequest()
	req.Result = ResultManualActionRequired
	req.Coverage.Status = "PARTIAL"
	req.Findings = nil
	req.Coverage.Unresolved = []string{"OrderService.approve: IMPLEMENTATION_NOT_FOUND"}
	req.Coverage.MissingReviewedFiles = []string{"OrderDTO.java"}
	req.Coverage.RuntimeErrors = []string{"change-analysis contract failed"}
	md, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Result: MANUAL_ACTION_REQUIRED", "Coverage: PARTIAL", "IMPLEMENTATION_NOT_FOUND", "Missing reviewed file: OrderDTO.java", "Runtime Contract validation error: change-analysis contract failed"} {
		if !strings.Contains(md, want) {
			t.Fatalf("missing %q in %s", want, md)
		}
	}
}

func TestR4NoChangesWritesPassedZeroCounts(t *testing.T) {
	req := sampleRequest()
	req.Result = ResultPassed
	req.Scope.ChangedFiles = nil
	req.Coverage.ReviewedFiles = nil
	req.Coverage.CallChain = nil
	req.Coverage.ExternalDependencies = nil
	req.Findings = nil
	md, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Result: PASSED", "Changed Files: 0", "Findings: 0"} {
		if !strings.Contains(md, want) {
			t.Fatalf("missing %q in %s", want, md)
		}
	}
}

func TestR6RunIDEscapeRejected(t *testing.T) {
	req := sampleRequest()
	req.RunID = "../escape"
	if _, err := Write(t.TempDir(), req); err == nil {
		t.Fatal("expected runId escape rejection")
	}
}

func TestR7ReportCannotWriteOutsideRunDirectory(t *testing.T) {
	root := t.TempDir()
	req := sampleRequest()
	path, err := Write(root, req)
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(root, ".code-harness", "runs", req.RunID, "review.md")
	if path != expected {
		t.Fatalf("path=%q want=%q", path, expected)
	}
}

func TestR8SameInputIsDeterministic(t *testing.T) {
	req := sampleRequest()
	a, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatal("same input produced different markdown")
	}
}

func TestRequestTransportMustMatchRunAndIsDeletedAfterSuccess(t *testing.T) {
	root := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()

	req := sampleRequest()
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(".code-harness", "runs", req.RunID, "requests", "review-report.json")
	if err := os.MkdirAll(filepath.Dir(input), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input, data, 0o600); err != nil {
		t.Fatal(err)
	}
	path, err := WriteRequestFile(".", input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(input); !os.IsNotExist(err) {
		t.Fatalf("transport should be deleted, stat err=%v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestRequestTransportRejectsUnknownFields(t *testing.T) {
	req := sampleRequest()
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data[:len(data)-1], []byte(`,"unexpected":true}`)...)
	if _, err := DecodeReviewRequest(data); err == nil {
		t.Fatal("expected unknown field rejection")
	}
}
