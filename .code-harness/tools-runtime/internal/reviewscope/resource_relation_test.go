package reviewscope_test

import (
	"strings"
	"testing"

	"codea-harness-tools/internal/reviewscope"
)

const resourceAnalysis = `{
  "changedFiles":[
    {"path":"src/main/java/OrderController.java"},
    {"path":"src/main/java/OrderMapper.java"},
    {"path":"src/main/resources/mapper/OrderMapper.xml"},
    {"path":"src/main/resources/mapper/UserMapper.xml"},
    {"path":"src/main/resources/application.yml"}
  ],
  "callChains":[
    {"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderMapper.updateStatus"]}
  ],
  "symbolLocations":[
    {"symbol":"OrderController.approve","path":"src/main/java/OrderController.java","role":"Controller","source":"FIND_SYMBOL"},
    {"symbol":"OrderMapper.updateStatus","path":"src/main/java/OrderMapper.java","role":"Mapper","source":"FIND_SYMBOL"}
  ],
  "resourceRelations":[
    {"path":"src/main/resources/mapper/OrderMapper.xml","role":"MapperXml","resource":"OrderMapper.xml#updateStatus","fromSymbol":"OrderMapper.updateStatus","fromKind":"METHOD","source":"MAPPER_STATEMENT","evidence":"statement id updateStatus matches selected Mapper method"},
    {"path":"src/main/resources/mapper/UserMapper.xml","role":"MapperXml","resource":"UserMapper.xml#disable","fromSymbol":"UserMapper.disable","fromKind":"METHOD","source":"MAPPER_STATEMENT","evidence":"unrelated mapper statement"},
    {"path":"src/main/resources/application.yml","role":"YamlConfig","resource":"order.timeout-ms","fromSymbol":"OrderController","fromKind":"CLASS","source":"CONFIG_REFERENCE","evidence":"selected Controller consumes order.timeout-ms"}
  ],
  "reviewCoverage":{"reviewedFiles":[
    {"path":"src/main/java/OrderController.java"},
    {"path":"src/main/java/OrderMapper.java"},
    {"path":"src/main/resources/mapper/OrderMapper.xml"},
    {"path":"src/main/resources/application.yml"}
  ],"unresolvedSymbols":[]}
}`

func TestTargetedScopeRequiresChangedMapperRelatedToSelectedChain(t *testing.T) {
	selection := []byte(`{
      "mode":"TARGETED",
      "target":{"symbol":"OrderController.approve","kind":"METHOD"},
      "selectedCallChains":[{"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderMapper.updateStatus"]}],
      "scopedFiles":["src/main/java/OrderController.java","src/main/java/OrderMapper.java","src/main/resources/application.yml"]
    }`)
	if _, err := reviewscope.Verify(selection, []byte(resourceAnalysis)); err == nil || !strings.Contains(err.Error(), "OrderMapper.xml") {
		t.Fatalf("related changed Mapper.xml must be required, err=%v", err)
	}
}

func TestTargetedScopeRequiresChangedYamlWithSelectedClassEvidence(t *testing.T) {
	selection := []byte(`{
      "mode":"TARGETED",
      "target":{"symbol":"OrderController.approve","kind":"METHOD"},
      "selectedCallChains":[{"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderMapper.updateStatus"]}],
      "scopedFiles":["src/main/java/OrderController.java","src/main/java/OrderMapper.java","src/main/resources/mapper/OrderMapper.xml"]
    }`)
	if _, err := reviewscope.Verify(selection, []byte(resourceAnalysis)); err == nil || !strings.Contains(err.Error(), "application.yml") {
		t.Fatalf("related changed YML must be required, err=%v", err)
	}
}

func TestTargetedScopeExcludesUnrelatedChangedMapper(t *testing.T) {
	selection := []byte(`{
      "mode":"TARGETED",
      "target":{"symbol":"OrderController.approve","kind":"METHOD"},
      "selectedCallChains":[{"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderMapper.updateStatus"]}],
      "scopedFiles":[
        "src/main/java/OrderController.java",
        "src/main/java/OrderMapper.java",
        "src/main/resources/mapper/OrderMapper.xml",
        "src/main/resources/application.yml",
        "src/main/resources/mapper/UserMapper.xml"
      ]
    }`)
	if _, err := reviewscope.Verify(selection, []byte(resourceAnalysis)); err == nil || !strings.Contains(err.Error(), "UserMapper.xml") {
		t.Fatalf("unrelated changed Mapper.xml must be rejected from targeted scope, err=%v", err)
	}
}

func TestTargetedScopePassesWithOnlyEvidenceRelatedResources(t *testing.T) {
	selection := []byte(`{
      "mode":"TARGETED",
      "target":{"symbol":"OrderController.approve","kind":"METHOD"},
      "selectedCallChains":[{"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderMapper.updateStatus"]}],
      "scopedFiles":[
        "src/main/java/OrderController.java",
        "src/main/java/OrderMapper.java",
        "src/main/resources/mapper/OrderMapper.xml",
        "src/main/resources/application.yml"
      ]
    }`)
	if _, err := reviewscope.Verify(selection, []byte(resourceAnalysis)); err != nil {
		t.Fatalf("evidence-related resource scope should pass: %v", err)
	}
}
