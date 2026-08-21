package reviewscope_test

import (
	"strings"
	"testing"

	"codea-harness-tools/internal/reviewscope"
)

const changeAnalysis = `{
  "changedFiles":[
    {"path":"src/main/java/OrderController.java"},
    {"path":"src/main/java/OrderService.java"},
    {"path":"src/main/java/UnrelatedService.java"}
  ],
  "callChains":[
    {"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderService.approve"]},
    {"entryPoint":"OrderController.cancel","chain":["OrderController.cancel","OrderService.cancel"]}
  ],
  "reviewCoverage":{
    "reviewedFiles":[
      {"path":"src/main/java/OrderController.java"},
      {"path":"src/main/java/OrderService.java"}
    ],
    "unresolvedSymbols":[]
  }
}`

func targetedSelection(scopedFiles string) []byte {
	return []byte(`{
	  "mode":"TARGETED",
	  "target":{"symbol":"OrderController.approve","kind":"METHOD"},
	  "selectedCallChains":[{"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderService.approve"]}],
	  "scopedFiles":` + scopedFiles + `
	}`)
}

func TestVerifyRejectsSelectedChainNotInChangeAnalysis(t *testing.T) {
	selection := []byte(`{
	  "mode":"TARGETED",
	  "target":{"symbol":"OrderController.refund","kind":"METHOD"},
	  "selectedCallChains":[{"entryPoint":"OrderController.refund","chain":["OrderController.refund","OrderService.refund"]}],
	  "scopedFiles":["src/main/java/OrderController.java","src/main/java/OrderService.java"]
	}`)
	if _, err := reviewscope.Verify(selection, []byte(changeAnalysis)); err == nil || !strings.Contains(err.Error(), "selected call chain") {
		t.Fatalf("expected selected chain rejection, err=%v", err)
	}
}

func TestVerifyRejectsUnjustifiedScopedFile(t *testing.T) {
	selection := targetedSelection(`["src/main/java/OrderController.java","src/main/java/UnrelatedService.java"]`)
	if _, err := reviewscope.Verify(selection, []byte(changeAnalysis)); err == nil || !strings.Contains(err.Error(), "not justified") {
		t.Fatalf("expected unjustified scoped file rejection, err=%v", err)
	}
}

func TestTargetedCoverageIsPartialWhenScopedFileMissing(t *testing.T) {
	selection, err := reviewscope.Verify(targetedSelection(`["src/main/java/OrderController.java","src/main/java/OrderService.java"]`), []byte(changeAnalysis))
	if err != nil {
		t.Fatal(err)
	}
	result := reviewscope.ComputeCoverage(selection, []string{"src/main/java/OrderController.java"})
	if result.Status != "PARTIAL" || len(result.MissingFiles) != 1 || result.MissingFiles[0] != "src/main/java/OrderService.java" {
		t.Fatalf("result=%+v", result)
	}
}

func TestTargetedCoverageAllowsUnrelatedChangedFileOutsideScope(t *testing.T) {
	selection, err := reviewscope.Verify(targetedSelection(`["src/main/java/OrderController.java","src/main/java/OrderService.java"]`), []byte(changeAnalysis))
	if err != nil {
		t.Fatal(err)
	}
	result := reviewscope.ComputeCoverage(selection, []string{"src/main/java/OrderController.java", "src/main/java/OrderService.java"})
	if result.Status != "COMPLETE" || len(result.MissingFiles) != 0 {
		t.Fatalf("result=%+v", result)
	}
}

func TestFullCoverageStillRequiresEveryChangedFile(t *testing.T) {
	selection, err := reviewscope.Verify([]byte(`{"mode":"FULL","selectedCallChains":[],"scopedFiles":[]}`), []byte(changeAnalysis))
	if err != nil {
		t.Fatal(err)
	}
	result := reviewscope.ComputeCoverage(selection, []string{"src/main/java/OrderController.java", "src/main/java/OrderService.java"})
	if result.Status != "PARTIAL" || len(result.MissingFiles) != 1 || result.MissingFiles[0] != "src/main/java/UnrelatedService.java" {
		t.Fatalf("result=%+v", result)
	}
}

func TestTargetMustBelongToSelectedChain(t *testing.T) {
	selection := []byte(`{
	  "mode":"TARGETED",
	  "target":{"symbol":"PaymentController.pay","kind":"METHOD"},
	  "selectedCallChains":[{"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderService.approve"]}],
	  "scopedFiles":["src/main/java/OrderController.java","src/main/java/OrderService.java"]
	}`)
	if _, err := reviewscope.Verify(selection, []byte(changeAnalysis)); err == nil || !strings.Contains(err.Error(), "target") {
		t.Fatalf("expected target relation rejection, err=%v", err)
	}
}
