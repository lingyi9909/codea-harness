package schema

import (
	"os"
	"testing"
)

func TestChangeAnalysisAcceptsExactNavigationSymbolLocations(t *testing.T) {
	schemaBytes, err := os.ReadFile("../../../contracts/change-analysis.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	valid := []byte(`{
      "reviewScope":{"currentBranch":"feature","baseRef":"origin/develop","baseCommit":"a","mergeBase":"a","headCommit":"b","includeWorkingTree":true},
      "changedFiles":[{"path":"module-a/src/main/java/OrderService.java","role":"Service","sources":["COMMITTED"]}],
      "affectedControllers":[{"controller":"OrderController","endpoints":["approve"],"impactType":"AFFECTED_BY_CALL_CHAIN","sourceSymbols":["OrderService.approve"]}],
      "callChains":[{"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderService.approve"]}],
      "symbolLocations":[
        {"symbol":"OrderController.approve","path":"module-a/src/main/java/OrderController.java","role":"Controller","source":"FIND_REFERENCES"},
        {"symbol":"OrderService.approve","path":"module-a/src/main/java/OrderService.java","role":"Service","source":"FIND_SYMBOL"}
      ],
      "externalDependencies":[],
      "riskAreas":[],
      "reviewCoverage":{"status":"COMPLETE","reviewedFiles":[{"path":"module-a/src/main/java/OrderService.java","role":"Service","reason":"CHANGED"},{"path":"module-a/src/main/java/OrderController.java","role":"Controller","reason":"CALL_CHAIN"}],"unresolvedSymbols":[]}
    }`)
	if err := ValidateJSON(schemaBytes, valid); err != nil {
		t.Fatalf("valid symbolLocations rejected: %v", err)
	}
}

func TestChangeAnalysisRejectsUnsafeNavigationSymbolPath(t *testing.T) {
	schemaBytes, err := os.ReadFile("../../../contracts/change-analysis.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	invalid := []byte(`{
      "reviewScope":{"currentBranch":"feature","baseRef":"origin/develop","baseCommit":"a","mergeBase":"a","headCommit":"b","includeWorkingTree":true},
      "changedFiles":[{"path":"src/main/java/OrderService.java","role":"Service","sources":["COMMITTED"]}],
      "affectedControllers":[],
      "callChains":[{"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderService.approve"]}],
      "symbolLocations":[{"symbol":"OrderService.approve","path":"../other/OrderService.java","role":"Service","source":"FIND_SYMBOL"}],
      "externalDependencies":[],"riskAreas":[],
      "reviewCoverage":{"status":"COMPLETE","reviewedFiles":[{"path":"src/main/java/OrderService.java","role":"Service","reason":"CHANGED"}],"unresolvedSymbols":[]}
    }`)
	if err := ValidateJSON(schemaBytes, invalid); err == nil {
		t.Fatal("unsafe symbol location path must be rejected")
	}
}
