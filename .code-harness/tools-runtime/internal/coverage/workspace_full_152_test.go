package coverage_test

import (
	"strings"
	"testing"

	"codea-harness-tools/internal/coverage"
)

func Test152FullCoverageRejectsDependencyWorkspaceReviewedFile(t *testing.T) {
	analysis := []byte(`{
		"changedFiles":[
			{"path":"src/main/java/com/company/order/XxxController.java","role":"Controller"}
		],
		"symbolLocations":[
			{"workspace":"current","symbol":"XxxController.submit","path":"src/main/java/com/company/order/XxxController.java","role":"Controller","source":"FIND_SYMBOL"},
			{"workspace":"company-framework","symbol":"AbstractTemplate.execute","path":"src/main/java/com/company/framework/AbstractTemplate.java","role":"Service","source":"WORKSPACE_INHERITANCE","from":"XxxServiceImpl.submit"}
		],
		"reviewCoverage":{
			"status":"COMPLETE",
			"reviewedFiles":[
				{"path":"src/main/java/com/company/order/XxxController.java","role":"Controller"},
				{"path":"src/main/java/com/company/framework/AbstractTemplate.java","role":"Service"}
			],
			"unresolvedSymbols":[]
		}
	}`)

	_, err := coverage.VerifyAnalysisJSON(analysis)
	if err == nil {
		t.Fatal("dependency workspace source must not enter FULL reviewCoverage.reviewedFiles")
	}
	if !strings.Contains(err.Error(), "src/main/java/com/company/framework/AbstractTemplate.java") {
		t.Fatalf("machine rejection must identify dependency reviewed path, err=%v", err)
	}
}

func Test152FullCoverageAllowsDependencyWorkspaceAsNavigationContextOnly(t *testing.T) {
	analysis := []byte(`{
		"changedFiles":[
			{"path":"src/main/java/com/company/order/XxxController.java","role":"Controller"}
		],
		"symbolLocations":[
			{"workspace":"current","symbol":"XxxController.submit","path":"src/main/java/com/company/order/XxxController.java","role":"Controller","source":"FIND_SYMBOL"},
			{"workspace":"company-framework","symbol":"AbstractTemplate.execute","path":"src/main/java/com/company/framework/AbstractTemplate.java","role":"Service","source":"WORKSPACE_INHERITANCE","from":"XxxServiceImpl.submit"}
		],
		"reviewCoverage":{
			"status":"COMPLETE",
			"reviewedFiles":[
				{"path":"src/main/java/com/company/order/XxxController.java","role":"Controller"}
			],
			"unresolvedSymbols":[]
		}
	}`)

	result, err := coverage.VerifyAnalysisJSON(analysis)
	if err != nil {
		t.Fatalf("dependency workspace may remain navigation/call-chain context only: %v", err)
	}
	if result.Status != "COMPLETE" {
		t.Fatalf("result=%+v", result)
	}
}

func Test152FullCoverageAllowsCurrentWorkspaceCallChainReviewedFile(t *testing.T) {
	analysis := []byte(`{
		"changedFiles":[
			{"path":"src/main/java/com/company/order/XxxController.java","role":"Controller"}
		],
		"symbolLocations":[
			{"workspace":"current","symbol":"XxxController.submit","path":"src/main/java/com/company/order/XxxController.java","role":"Controller","source":"FIND_SYMBOL"},
			{"workspace":"current","symbol":"XxxServiceImpl.submit","path":"src/main/java/com/company/order/XxxServiceImpl.java","role":"Service","source":"FIND_IMPLEMENTATIONS","from":"XxxService.submit"}
		],
		"reviewCoverage":{
			"status":"COMPLETE",
			"reviewedFiles":[
				{"path":"src/main/java/com/company/order/XxxController.java","role":"Controller"},
				{"path":"src/main/java/com/company/order/XxxServiceImpl.java","role":"Service"}
			],
			"unresolvedSymbols":[]
		}
	}`)

	result, err := coverage.VerifyAnalysisJSON(analysis)
	if err != nil {
		t.Fatalf("current-workspace call-chain source must remain legal FULL reviewed context: %v", err)
	}
	if result.Status != "COMPLETE" {
		t.Fatalf("result=%+v", result)
	}
}
