package report

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/finding"
)

func TestReportRejectsRawAgentFindingWithoutCertifiedSet(t *testing.T) {
	root := t.TempDir()
	req := task4ReportRequest160("run-report-raw")
	req.Findings = []Finding{{ID: "RAW-1", Category: "PRODUCTION_CODE", Severity: "high", File: "src/main/java/com/acme/OrderServiceImpl.java", Problem: "raw", Evidence: "raw", Impact: "raw", Recommendation: "raw", Confidence: 0.9}}
	if _, err := WriteCertifiedReport(root, req); err == nil {
		t.Fatal("formal report must reject raw Agent findings without same-run Certified Findings")
	}
}

func TestReportUsesCertifiedFindingAnchor(t *testing.T) {
	root := t.TempDir()
	runID := "run-report-anchor"
	writeReportCertifiedFixture160(t, root, runID, []finding.CertifiedFinding{{
		ID: "CF-ANCHOR",
		RuleID: "SPRING-TX-001",
		ReviewUnitID: "RU-TASK4",
		Category: "PRODUCTION_CODE",
		Severity: "high",
		Anchor: finding.Anchor{Kind: finding.AnchorSymbol, Path: "src/main/java/com/acme/OrderServiceImpl.java", Symbol: "OrderServiceImpl.approve"},
		EvidenceRefs: []finding.EvidenceRef{{Kind: "SYMBOL", Value: "OrderServiceImpl.approve", Path: "src/main/java/com/acme/OrderServiceImpl.java"}},
		Problem: "事务代理语义存在风险",
		Impact: "事务可能不生效",
		Recommendation: "通过代理边界调用",
		NeedsTest: true,
		Confidence: 0.95,
	}})
	req := task4ReportRequest160(runID)
	path, err := WriteCertifiedReport(root, req)
	if err != nil { t.Fatalf("write certified report: %v", err) }
	data, err := os.ReadFile(path)
	if err != nil { t.Fatal(err) }
	text := string(data)
	if !strings.Contains(text, "OrderServiceImpl.approve") || strings.Contains(text, "OrderServiceImpl.java:0") {
		t.Fatalf("SYMBOL anchor must render path + symbol without invented line:\n%s", text)
	}
}

func TestReportCanPassWithZeroCertifiedFindings(t *testing.T) {
	root := t.TempDir()
	runID := "run-report-zero"
	writeReportCertifiedFixture160(t, root, runID, nil)
	req := task4ReportRequest160(runID)
	req.Result = ResultPassed
	path, err := WriteCertifiedReport(root, req)
	if err != nil { t.Fatalf("zero Certified Findings must permit a complete passing report: %v", err) }
	data, err := os.ReadFile(path)
	if err != nil { t.Fatal(err) }
	if !strings.Contains(string(data), "本次评审通过") {
		t.Fatalf("expected passing report, got:\n%s", data)
	}
}

func task4ReportRequest160(runID string) ReviewRequest {
	return ReviewRequest{
		RunID: runID,
		HarnessVersion: "1.6.0",
		BaseRef: "main",
		Head: "abc123",
		Result: ResultPassed,
		Mode: "FULL",
		Scope: ReviewScope{ChangedFiles: []string{"src/main/java/com/acme/OrderServiceImpl.java"}},
		Coverage: ReviewCoverage{ReviewedFiles: []string{"src/main/java/com/acme/OrderServiceImpl.java"}, ExternalDependencies: []string{}, Unresolved: []string{}, MissingReviewedFiles: []string{}, RuntimeErrors: []string{}, Status: "COMPLETE"},
		Findings: []Finding{},
	}
}

func writeReportCertifiedFixture160(t *testing.T, root, runID string, findings []finding.CertifiedFinding) {
	t.Helper()
	analysisDir := filepath.Join(root, ".code-harness", "runs", runID, "analysis")
	requestsDir := filepath.Join(root, ".code-harness", "runs", runID, "requests")
	if err := os.MkdirAll(analysisDir, 0o755); err != nil { t.Fatal(err) }
	if err := os.MkdirAll(requestsDir, 0o755); err != nil { t.Fatal(err) }
	authorities := map[string][]byte{
		filepath.Join(analysisDir, "change-analysis.json"): []byte("{\"authority\":\"analysis\"}\n"),
		filepath.Join(analysisDir, "review-units.json"): []byte("{\"authority\":\"units\"}\n"),
		filepath.Join(analysisDir, "rule-dispatch.json"): []byte("{\"authority\":\"dispatch\"}\n"),
		filepath.Join(requestsDir, "finding-proposals.json"): []byte("[]\n"),
	}
	for p, data := range authorities {
		if err := os.WriteFile(p, data, 0o644); err != nil { t.Fatal(err) }
	}
	set := finding.CertifiedSet{RunID: runID, HarnessVersion: "1.6.0", ChangeSetSHA256: strings.Repeat("a", 64), ChangeAnalysisSHA256: task4SHA160(authorities[filepath.Join(analysisDir, "change-analysis.json")]), ReviewUnitsSHA256: task4SHA160(authorities[filepath.Join(analysisDir, "review-units.json")]), RuleDispatchSHA256: task4SHA160(authorities[filepath.Join(analysisDir, "rule-dispatch.json")]), FindingProposalsSHA256: task4SHA160(authorities[filepath.Join(requestsDir, "finding-proposals.json")]), Findings: findings}
	cert := finding.Certificate{RunID: runID, ChangeSetSHA256: set.ChangeSetSHA256, ChangeAnalysisSHA256: set.ChangeAnalysisSHA256, ReviewUnitsSHA256: set.ReviewUnitsSHA256, RuleDispatchSHA256: set.RuleDispatchSHA256, FindingProposalsSHA256: set.FindingProposalsSHA256, Mode: "FULL"}
	if err := finding.WriteCertified(root, set, cert); err != nil { t.Fatalf("write certified fixture: %v", err) }
}

func task4SHA160(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }
