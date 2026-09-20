package schema

import (
	"os"
	"testing"
)

func TestSymbolLocationsRejectResourceRoles(t *testing.T) {
	schemaBytes, err := os.ReadFile("../../../contracts/change-analysis.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	invalid := []byte(`{
      "reviewScope":{"currentBranch":"feature","baseRef":"origin/develop","baseCommit":"a","mergeBase":"a","headCommit":"b","includeWorkingTree":true},
      "changedFiles":[],"affectedControllers":[],"callChains":[],
      "symbolLocations":[{"symbol":"OrderMapper.xml#updateStatus","path":"src/main/resources/mapper/OrderMapper.xml","role":"MapperXml","source":"FIND_SYMBOL"}],
      "externalDependencies":[],"riskAreas":[],
      "reviewCoverage":{"status":"COMPLETE","reviewedFiles":[],"unresolvedSymbols":[]}
    }`)
	if err := ValidateJSON(schemaBytes, invalid); err == nil {
		t.Fatal("MapperXml/YamlConfig must use resourceRelations, not symbolLocations")
	}
}
