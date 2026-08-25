package schema

import (
	"os"
	"testing"
)

func Test152ChangeAnalysisAcceptsWorkspaceNavigationEvidence(t *testing.T) {
	schemaBytes, err := os.ReadFile("../../../contracts/change-analysis.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	analysis := []byte(`{
  "reviewScope": {"currentBranch":"feature","baseRef":"develop","baseCommit":"a","mergeBase":"a","headCommit":"b","includeWorkingTree":true},
  "changedFiles": [{"path":"src/main/java/com/company/XxxServiceImpl.java","role":"Service","sources":["UNSTAGED"]}],
  "affectedControllers": [],
  "callChains": [],
  "symbolLocations": [
    {"workspace":"current","symbol":"XxxServiceImpl.submit","path":"src/main/java/com/company/XxxServiceImpl.java","role":"Service","source":"FIND_SYMBOL"},
    {"workspace":"company-framework","symbol":"AbstractTemplate.execute","path":"src/main/java/com/company/AbstractTemplate.java","role":"Service","source":"WORKSPACE_INHERITANCE","from":"XxxServiceImpl.submit"}
  ],
  "resourceRelations": [],
  "externalDependencies": [],
  "riskAreas": [],
  "reviewCoverage": {"status":"COMPLETE","reviewedFiles":[{"path":"src/main/java/com/company/XxxServiceImpl.java","role":"Service","reason":"CHANGED"}],"unresolvedSymbols":[]}
}`)
	if err := ValidateJSON(schemaBytes, analysis); err != nil {
		t.Fatalf("workspace ChangeAnalysis evidence must validate: %v", err)
	}
}

func Test152LegacyChangeAnalysisWithoutWorkspaceRemainsValid(t *testing.T) {
	schemaBytes, err := os.ReadFile("../../../contracts/change-analysis.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	analysis := []byte(`{
  "reviewScope": {"currentBranch":"feature","baseRef":"develop","baseCommit":"a","mergeBase":"a","headCommit":"b","includeWorkingTree":true},
  "changedFiles": [{"path":"src/main/java/com/company/XxxServiceImpl.java","role":"Service","sources":["UNSTAGED"]}],
  "affectedControllers": [],
  "callChains": [],
  "symbolLocations": [{"symbol":"XxxServiceImpl.submit","path":"src/main/java/com/company/XxxServiceImpl.java","role":"Service","source":"FIND_SYMBOL"}],
  "resourceRelations": [],
  "externalDependencies": [],
  "riskAreas": [],
  "reviewCoverage": {"status":"COMPLETE","reviewedFiles":[{"path":"src/main/java/com/company/XxxServiceImpl.java","role":"Service","reason":"CHANGED"}],"unresolvedSymbols":[]}
}`)
	if err := ValidateJSON(schemaBytes, analysis); err != nil {
		t.Fatalf("1.5.1 ChangeAnalysis without workspace must remain valid: %v", err)
	}
}
