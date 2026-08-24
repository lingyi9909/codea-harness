package schema

import (
	"os"
	"testing"
)

func TestChangeAnalysisAcceptsMapperAndYamlResourceEvidence(t *testing.T) {
	schemaBytes, err := os.ReadFile("../../../contracts/change-analysis.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	valid := []byte(`{
      "reviewScope":{"currentBranch":"feature","baseRef":"origin/develop","baseCommit":"a","mergeBase":"a","headCommit":"b","includeWorkingTree":true},
      "changedFiles":[
        {"path":"src/main/resources/mapper/OrderMapper.xml","role":"MapperXml","sources":["COMMITTED"]},
        {"path":"src/main/resources/application.yml","role":"YamlConfig","sources":["COMMITTED"]}
      ],
      "affectedControllers":[],
      "callChains":[{"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderMapper.updateStatus"]}],
      "symbolLocations":[
        {"symbol":"OrderController.approve","path":"src/main/java/OrderController.java","role":"Controller","source":"FIND_SYMBOL"},
        {"symbol":"OrderMapper.updateStatus","path":"src/main/java/OrderMapper.java","role":"Mapper","source":"FIND_SYMBOL"}
      ],
      "resourceRelations":[
        {"path":"src/main/resources/mapper/OrderMapper.xml","role":"MapperXml","resource":"OrderMapper.xml#updateStatus","fromSymbol":"OrderMapper.updateStatus","fromKind":"METHOD","source":"MAPPER_STATEMENT","evidence":"statement id updateStatus matches OrderMapper.updateStatus"},
        {"path":"src/main/resources/application.yml","role":"YamlConfig","resource":"order.timeout-ms","fromSymbol":"OrderController","fromKind":"CLASS","source":"CONFIG_REFERENCE","evidence":"OrderController configuration dependency references order.timeout-ms"}
      ],
      "externalDependencies":[],
      "riskAreas":[],
      "reviewCoverage":{"status":"COMPLETE","reviewedFiles":[
        {"path":"src/main/resources/mapper/OrderMapper.xml","role":"MapperXml","reason":"CHANGED"},
        {"path":"src/main/resources/application.yml","role":"YamlConfig","reason":"CHANGED"}
      ],"unresolvedSymbols":[]}
    }`)
	if err := ValidateJSON(schemaBytes, valid); err != nil {
		t.Fatalf("valid resource evidence rejected: %v", err)
	}
}

func TestResourceRelationRejectsUnsupportedRoleAndSource(t *testing.T) {
	schemaBytes, err := os.ReadFile("../../../contracts/change-analysis.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	invalid := []byte(`{
      "reviewScope":{"currentBranch":"feature","baseRef":"origin/develop","baseCommit":"a","mergeBase":"a","headCommit":"b","includeWorkingTree":true},
      "changedFiles":[],"affectedControllers":[],"callChains":[],
      "resourceRelations":[{"path":"pom.xml","role":"Other","resource":"x","fromSymbol":"A.m","fromKind":"METHOD","source":"ARBITRARY","evidence":"x"}],
      "externalDependencies":[],"riskAreas":[],
      "reviewCoverage":{"status":"COMPLETE","reviewedFiles":[],"unresolvedSymbols":[]}
    }`)
	if err := ValidateJSON(schemaBytes, invalid); err == nil {
		t.Fatal("unsupported resource relation must be rejected")
	}
}
