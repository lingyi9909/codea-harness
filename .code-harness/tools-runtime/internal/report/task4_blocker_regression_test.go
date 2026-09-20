package report

import (
	"os"
	"strings"
	"testing"
)

func TestTask4PartialReportDoesNotRequireCertifiedFindings(t *testing.T) {
	root := t.TempDir()
	installReportFindingContracts160(t, root)
	req := task4ReportRequest160("run-partial-no-certified")
	req.Result = ResultPassed
	req.Coverage.Status = "PARTIAL"
	req.Coverage.Unresolved = []string{"OrderService.resolve"}
	req.Findings = []Finding{}

	path, err := Write(root, req)
	if err != nil {
		t.Fatalf("PARTIAL report must render MANUAL_ACTION_REQUIRED without Certified Findings: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil { t.Fatal(err) }
	text := string(data)
	if !strings.Contains(text, "需要人工处理") || strings.Contains(text, "问题数量 | 1") {
		t.Fatalf("PARTIAL report must render manual action with zero findings:\n%s", text)
	}
}
