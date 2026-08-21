package report

import (
	"strings"
	"testing"
)

func TestTargetedReportRejectsFindingOutsideVerifiedScopedFiles(t *testing.T) {
	req := sampleRequest()
	req.Mode = "TARGETED"
	req.Target = &ReviewTarget{Symbol: "OrderController.approve", Kind: "METHOD"}
	req.Scope.ScopedFiles = []string{"src/main/java/OrderController.java", "src/main/java/OrderService.java"}
	req.Coverage.CallChains = []CallChain{{EntryPoint: "OrderController.approve", Chain: []string{"OrderController.approve", "OrderService.approve"}}}
	req.Findings = []Finding{{
		ID: "F-OUT", Category: "PRODUCTION_CODE", Severity: "HIGH",
		File: "src/main/java/UnrelatedService.java", Line: 10,
		Problem: "范围外问题", Evidence: "范围外证据", Impact: "范围外影响", Recommendation: "不应进入报告",
		IntroducedByChange: true, Confidence: 0.9,
	}}
	if _, err := Render(req); err == nil || !strings.Contains(err.Error(), "outside verified scopedFiles") {
		t.Fatalf("TARGETED report must reject scope-out finding, err=%v", err)
	}
}

func TestTargetedReportAcceptsFindingInsideVerifiedScopedFiles(t *testing.T) {
	req := sampleRequest()
	req.Mode = "TARGETED"
	req.Target = &ReviewTarget{Symbol: "OrderController.approve", Kind: "METHOD"}
	req.Scope.ScopedFiles = []string{"src/main/java/OrderController.java", "src/main/java/OrderService.java"}
	req.Coverage.CallChains = []CallChain{{EntryPoint: "OrderController.approve", Chain: []string{"OrderController.approve", "OrderService.approve"}}}
	req.Findings = []Finding{{
		ID: "F-IN", Category: "PRODUCTION_CODE", Severity: "HIGH",
		File: "src/main/java/OrderService.java", Line: 10,
		Problem: "范围内问题", Evidence: "范围内证据", Impact: "影响", Recommendation: "修复",
		IntroducedByChange: true, Confidence: 0.9,
	}}
	if _, err := Render(req); err != nil {
		t.Fatalf("scope-in finding should render: %v", err)
	}
}
