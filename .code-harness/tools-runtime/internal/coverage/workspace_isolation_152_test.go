package coverage_test

import (
	"strings"
	"testing"

	"codea-harness-tools/internal/coverage"
)

func Test152ReviewIsolationKeepsDependencyNodeOutOfFullReviewedFiles(t *testing.T) {
	analysis := []byte(`{
		"changedFiles":[{"path":"src/main/java/com/company/order/XxxServiceImpl.java","role":"Service"}],
		"callChains":[{"entryPoint":"XxxController.submit","chain":["XxxController.submit","XxxServiceImpl.submit","AbstractTemplate.execute","XxxServiceImpl.doExecute"]}],
		"symbolLocations":[
			{"workspace":"current","symbol":"XxxServiceImpl.submit","path":"src/main/java/com/company/order/XxxServiceImpl.java","role":"Service","source":"FIND_IMPLEMENTATIONS"},
			{"workspace":"company-framework","symbol":"AbstractTemplate.execute","path":"src/main/java/com/company/framework/AbstractTemplate.java","role":"Service","source":"WORKSPACE_INHERITANCE","from":"XxxServiceImpl.submit"},
			{"workspace":"current","symbol":"XxxServiceImpl.doExecute","path":"src/main/java/com/company/order/XxxServiceImpl.java","role":"Service","source":"WORKSPACE_INHERITANCE","from":"AbstractTemplate.execute"}
		],
		"reviewCoverage":{"status":"COMPLETE","reviewedFiles":[{"path":"src/main/java/com/company/order/XxxServiceImpl.java","role":"Service"}],"unresolvedSymbols":[]}
	}`)

	result, err := coverage.VerifyAnalysisJSON(analysis)
	if err != nil {
		t.Fatalf("dependency chain node is navigation context only: %v", err)
	}
	if result.Status != "COMPLETE" {
		t.Fatalf("result=%+v", result)
	}
}

func Test152ReviewIsolationRejectsDependencyNodeAsFullReviewedFile(t *testing.T) {
	analysis := []byte(`{
		"changedFiles":[{"path":"src/main/java/com/company/order/XxxServiceImpl.java","role":"Service"}],
		"symbolLocations":[
			{"workspace":"current","symbol":"XxxServiceImpl.submit","path":"src/main/java/com/company/order/XxxServiceImpl.java","role":"Service","source":"FIND_IMPLEMENTATIONS"},
			{"workspace":"company-framework","symbol":"AbstractTemplate.execute","path":"src/main/java/com/company/framework/AbstractTemplate.java","role":"Service","source":"WORKSPACE_INHERITANCE","from":"XxxServiceImpl.submit"}
		],
		"reviewCoverage":{"status":"COMPLETE","reviewedFiles":[
			{"path":"src/main/java/com/company/order/XxxServiceImpl.java","role":"Service"},
			{"path":"src/main/java/com/company/framework/AbstractTemplate.java","role":"Service"}
		],"unresolvedSymbols":[]}
	}`)

	_, err := coverage.VerifyAnalysisJSON(analysis)
	if err == nil || !strings.Contains(err.Error(), "AbstractTemplate.java") {
		t.Fatalf("dependency reviewed file must be rejected, err=%v", err)
	}
}
