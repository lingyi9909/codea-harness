package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"codea-harness-tools/internal/report"
	"codea-harness-tools/internal/reviewselection"
)

func installTask164PinnedNavigation(t *testing.T, testFile string) {
	t.Helper()
	source := filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", "..", "..", "bin", "ast-grep.exe"))
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read packaged pinned ast-grep fixture %s: %v", source, err)
	}
	target := filepath.Join(".code-harness", "bin", "ast-grep.exe")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("create pinned navigation fixture directory: %v", err)
	}
	if err := os.WriteFile(target, data, 0o755); err != nil {
		t.Fatalf("install pinned ast-grep fixture: %v", err)
	}
}

func prepareTask164FindingCertification(t *testing.T, withAuthority bool) string {
	t.Helper()
	analysisPath := setupTask4ReviewContextProject(t)
	// Finding certification resolves every declared symbol through pinned
	// navigation. Materialize the chain before sealing the snapshot so its
	// source bytes and Git identity agree with the analysis fixture.
	for name, source := range map[string]string{
		"OrderController.java": `package com.example.order;
public class OrderController {
    private OrderService service;
    public void approve() { service.approve(); }
}
`,
		"OrderService.java": `package com.example.order;
public interface OrderService {
    void approve();
}
`,
		"OrderServiceImpl.java": `package com.example.order;
public class OrderServiceImpl {
    private OrderMapper mapper;
    public void approve() { mapper.updateStatus(); }
}
`,
		"OrderMapper.java": `package com.example.order;
public interface OrderMapper {
    void updateStatus();
}
`,
	} {
		writeFile(t, filepath.Join("src", "main", "java", "com", "example", "order", name), source)
	}
	writeFile(t, filepath.Join("src", "main", "resources", "mapper", "OrderMapper.xml"), `<mapper namespace="com.example.order.OrderMapper"><update id="updateStatus">UPDATE orders SET status = 'APPROVED'</update></mapper>`)
	installTask153ReviewContextAuthoritySchemas(t)
	installTask153ReviewSelectionSchemas(t)
	prepareCommittedCertifiedAnalysisFixture153(t, "run-task4-review", analysisPath)
	optionsRequest := writeQueryRequest(t, "run-task4-review", "review-options-request.json", `{"runId":"run-task4-review","changeAnalysisPath":".code-harness/runs/run-task4-review/analysis/change-analysis.json"}`)
	if err := run([]string{"review", "options", "--input", optionsRequest}); err != nil {
		t.Fatalf("review options fixture: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(".code-harness", "runs", "run-task4-review", "analysis", "review-options.json"))
	if err != nil {
		t.Fatal(err)
	}
	var options reviewselection.Options
	if err := json.Unmarshal(data, &options); err != nil {
		t.Fatal(err)
	}
	if len(options.AutoSelectionIDs) != 1 {
		t.Fatalf("fixture must produce one AUTO_SINGLE selection: %+v", options)
	}
	selection := writeQueryRequest(t, "run-task4-review", "reviewer-finding-select.json", `{"runId":"run-task4-review","mode":"TARGETED","selectionIds":["`+options.AutoSelectionIDs[0]+`"],"optionsHash":"`+options.OptionsHash+`"}`)
	if err := run([]string{"review", "select", "--input", selection}); err != nil {
		t.Fatalf("review select fixture: %v", err)
	}
	for _, name := range []string{
		"review-unit.schema.json", "rule-dispatch.schema.json", "finding-proposals.schema.json",
		"finding-certify-request.schema.json", "certified-findings.schema.json", "certified-findings-cert.schema.json",
		"report-review-request.schema.json",
	} {
		copyTask153CommandContract(t, ".", name)
	}
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate Task 1.6.4 finding fixture source")
	}
	installTask160DispatchFramework(t, filepath.Dir(testFile))
	if err := run([]string{"review", "units", "--run-id", "run-task4-review"}); err != nil {
		t.Fatalf("review units fixture: %v", err)
	}
	if err := run([]string{"review", "dispatch", "--run-id", "run-task4-review"}); err != nil {
		t.Fatalf("review dispatch fixture: %v", err)
	}
	proposalRel := ".code-harness/runs/run-task4-review/requests/finding-proposals.json"
	writeFile(t, filepath.FromSlash(proposalRel), "[]\n")
	if withAuthority {
		installTask164PinnedNavigation(t, testFile)
		writeReviewerAuthorityTestReceipt(t, ".", "run-task4-review", "findings", proposalRel)
	}
	request := writeQueryRequest(t, "run-task4-review", "finding-certify-reviewer.json", `{"runId":"run-task4-review","proposalsPath":"`+proposalRel+`"}`)
	return request
}

func writeTask164ReportTransport(t *testing.T, runID string) string {
	t.Helper()
	transport := writeReportTransport153(t, runID)
	data, err := os.ReadFile(transport)
	if err != nil {
		t.Fatal(err)
	}
	var req report.ReviewRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatal(err)
	}
	// Echo the Runtime-selected scope; retain the hostile transport metadata
	// so the report test still proves it cannot replace certified authority.
	scopeBytes, err := os.ReadFile(filepath.Join(".code-harness", "runs", runID, "analysis", "review-scope.json"))
	if err != nil {
		t.Fatal(err)
	}
	var scope struct {
		Mode               string               `json:"mode"`
		Target             *report.ReviewTarget `json:"target"`
		ScopedFiles        []string             `json:"scopedFiles"`
		SelectedCallChains []report.CallChain   `json:"selectedCallChains"`
	}
	if err := json.Unmarshal(scopeBytes, &scope); err != nil {
		t.Fatal(err)
	}
	req.Mode, req.Target = scope.Mode, scope.Target
	req.Scope.ScopedFiles = scope.ScopedFiles
	req.Coverage.CallChains = scope.SelectedCallChains
	data, err = json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, transport, string(data))
	return transport
}

func Test164FindingCertificationRequiresReviewerHostAuthority(t *testing.T) {
	withTempProject(t)
	request := prepareTask164FindingCertification(t, false)
	err := run([]string{"review", "certify-findings", "--input", request})
	if err == nil {
		t.Fatal("finding certification without Reviewer Host authority must fail closed")
	}
	for _, marker := range []string{"REVIEWER_UNAVAILABLE", "MANUAL_ACTION_REQUIRED", "HARD STOP"} {
		if !strings.Contains(err.Error(), marker) {
			t.Fatalf("missing fail-closed marker %s: %v", marker, err)
		}
	}
	for _, name := range []string{"certified-findings.json", "certified-findings.cert.json"} {
		if _, statErr := os.Stat(filepath.Join(".code-harness", "runs", "run-task4-review", "analysis", name)); !os.IsNotExist(statErr) {
			t.Fatalf("missing Reviewer authority published %s: %v", name, statErr)
		}
	}
}

func Test164ReviewerCertifiedEmptyFindingsDrivePassedReport(t *testing.T) {
	withTempProject(t)
	request := prepareTask164FindingCertification(t, true)
	if err := run([]string{"review", "certify-findings", "--input", request}); err != nil {
		t.Fatalf("Reviewer-authorized finding certification failed: %v", err)
	}
	transport := writeTask164ReportTransport(t, "run-task4-review")
	if err := run([]string{"report", "review", "--input", transport}); err != nil {
		t.Fatalf("report must consume Runtime-certified Reviewer findings: %v", err)
	}
	out, err := os.ReadFile(filepath.Join(".code-harness", "runs", "run-task4-review", "review.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if !strings.Contains(text, "| 评审结果 | ✅ 通过 |") || !strings.Contains(text, "| 问题数量 | 0 |") {
		t.Fatalf("zero Runtime-certified Reviewer findings must drive PASSED report: %s", text)
	}
	for _, bad := range []string{"agent-version", "agent-base", "agent-head", "src/main/java/Evil.java"} {
		if strings.Contains(text, bad) {
			t.Fatalf("transport authority leaked into final report: %s", bad)
		}
	}
}
