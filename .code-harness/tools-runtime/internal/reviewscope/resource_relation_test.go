package reviewscope_test

import (
	"strings"
	"testing"

	"codea-harness-tools/internal/reviewscope"
)

const resourceAnalysis = `{
  "changedFiles":[
    {"path":"src/main/java/OrderController.java","role":"Controller"},
    {"path":"src/main/java/OrderMapper.java","role":"Mapper"},
    {"path":"src/main/resources/mapper/OrderMapper.xml","role":"MapperXml"},
    {"path":"src/main/resources/mapper/UserMapper.xml","role":"MapperXml"},
    {"path":"src/main/resources/application.yml","role":"YamlConfig"}
  ],
  "callChains":[
    {"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderMapper.updateStatus"]}
  ],
  "symbolLocations":[
    {"symbol":"OrderController.approve","path":"src/main/java/OrderController.java","role":"Controller","source":"FIND_SYMBOL"},
    {"symbol":"OrderController","path":"src/main/java/OrderController.java","role":"Controller","source":"FIND_SYMBOL"},
    {"symbol":"OrderMapper.updateStatus","path":"src/main/java/OrderMapper.java","role":"Mapper","source":"FIND_SYMBOL"}
  ],
  "resourceRelations":[
    {"path":"src/main/resources/mapper/OrderMapper.xml","role":"MapperXml","resource":"OrderMapper.xml#updateStatus","fromSymbol":"OrderMapper.updateStatus","fromKind":"METHOD","source":"MAPPER_STATEMENT","evidence":"statement id updateStatus matches selected Mapper method"},
    {"path":"src/main/resources/mapper/UserMapper.xml","role":"MapperXml","resource":"UserMapper.xml#disable","fromSymbol":"UserMapper.disable","fromKind":"METHOD","source":"MAPPER_STATEMENT","evidence":"unrelated mapper statement"},
    {"path":"src/main/resources/application.yml","role":"YamlConfig","resource":"order.timeout-ms","fromSymbol":"OrderController","fromKind":"CLASS","source":"CONFIG_REFERENCE","evidence":"selected Controller consumes order.timeout-ms"}
  ],
  "reviewCoverage":{"reviewedFiles":[
    {"path":"src/main/java/OrderController.java","role":"Controller"},
    {"path":"src/main/java/OrderMapper.java","role":"Mapper"},
    {"path":"src/main/resources/mapper/OrderMapper.xml","role":"MapperXml"},
    {"path":"src/main/resources/application.yml","role":"YamlConfig"}
  ],"unresolvedSymbols":[]}
}`

func targetedResourceSelection() []byte {
	return []byte(`{
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
}

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
	if _, err := reviewscope.Verify(targetedResourceSelection(), []byte(resourceAnalysis)); err != nil {
		t.Fatalf("evidence-related resource scope should pass: %v", err)
	}
}

func TestResourceRelationRoleMustMatchChangedFileRole(t *testing.T) {
	analysis := strings.Replace(resourceAnalysis,
		`"path":"src/main/resources/mapper/OrderMapper.xml","role":"MapperXml","resource":"OrderMapper.xml#updateStatus"`,
		`"path":"src/main/resources/mapper/OrderMapper.xml","role":"YamlConfig","resource":"OrderMapper.xml#updateStatus"`, 1)
	if _, err := reviewscope.Verify(targetedResourceSelection(), []byte(analysis)); err == nil || !strings.Contains(err.Error(), "changed file role") {
		t.Fatalf("resource relation role mismatch must be rejected, err=%v", err)
	}
}

func TestYamlConfigRejectsJavaPathMasquerade(t *testing.T) {
	analysis := strings.Replace(resourceAnalysis,
		`"path":"src/main/resources/application.yml","role":"YamlConfig","resource":"order.timeout-ms"`,
		`"path":"src/main/java/OrderController.java","role":"YamlConfig","resource":"order.timeout-ms"`, 1)
	if _, err := reviewscope.Verify(targetedResourceSelection(), []byte(analysis)); err == nil || !strings.Contains(err.Error(), "*.yml") {
		t.Fatalf("Java path must not masquerade as YamlConfig, err=%v", err)
	}
}

func TestYamlConfigRejectsMapperXmlMasquerade(t *testing.T) {
	analysis := strings.Replace(resourceAnalysis,
		`"path":"src/main/resources/application.yml","role":"YamlConfig","resource":"order.timeout-ms"`,
		`"path":"src/main/resources/mapper/OrderMapper.xml","role":"YamlConfig","resource":"order.timeout-ms"`, 1)
	if _, err := reviewscope.Verify(targetedResourceSelection(), []byte(analysis)); err == nil || !strings.Contains(err.Error(), "*.yml") {
		t.Fatalf("Mapper XML path must not masquerade as YamlConfig, err=%v", err)
	}
}

func TestMapperXmlRoleRequiresMapperXmlPath(t *testing.T) {
	analysis := strings.Replace(resourceAnalysis,
		`"path":"src/main/resources/mapper/OrderMapper.xml","role":"MapperXml","resource":"OrderMapper.xml#updateStatus"`,
		`"path":"src/main/resources/order.xml","role":"MapperXml","resource":"OrderMapper.xml#updateStatus"`, 1)
	if _, err := reviewscope.Verify(targetedResourceSelection(), []byte(analysis)); err == nil || !strings.Contains(err.Error(), "*Mapper.xml") {
		t.Fatalf("non-Mapper XML path must not masquerade as MapperXml, err=%v", err)
	}
}

func TestResourceRelationMethodRequiresExactNavigationEvidence(t *testing.T) {
	analysis := strings.Replace(resourceAnalysis,
		`"fromSymbol":"OrderMapper.updateStatus","fromKind":"METHOD"`,
		`"fromSymbol":"MissingMapper.updateStatus","fromKind":"METHOD"`, 1)
	if _, err := reviewscope.Verify(targetedResourceSelection(), []byte(analysis)); err == nil || !strings.Contains(err.Error(), "exact Code Navigation path evidence") {
		t.Fatalf("missing METHOD navigation evidence must be rejected, err=%v", err)
	}
}

func TestResourceRelationClassRequiresExactNavigationEvidence(t *testing.T) {
	analysis := strings.Replace(resourceAnalysis,
		`{"symbol":"OrderController","path":"src/main/java/OrderController.java","role":"Controller","source":"FIND_SYMBOL"},`, "", 1)
	if _, err := reviewscope.Verify(targetedResourceSelection(), []byte(analysis)); err == nil || !strings.Contains(err.Error(), "exact Code Navigation path evidence") {
		t.Fatalf("missing CLASS navigation evidence must be rejected, err=%v", err)
	}
}

func TestResourceRelationClassRejectsMultiModuleDuplicateEvidence(t *testing.T) {
	analysis := strings.Replace(resourceAnalysis,
		`{"symbol":"OrderController","path":"src/main/java/OrderController.java","role":"Controller","source":"FIND_SYMBOL"},`,
		`{"symbol":"OrderController","path":"module-a/src/main/java/OrderController.java","role":"Controller","source":"FIND_SYMBOL"},
    {"symbol":"OrderController","path":"module-b/src/main/java/OrderController.java","role":"Controller","source":"FIND_SYMBOL"},`, 1)
	if _, err := reviewscope.Verify(targetedResourceSelection(), []byte(analysis)); err == nil || !strings.Contains(err.Error(), "ambiguous Code Navigation path") {
		t.Fatalf("multi-module duplicate CLASS evidence must be rejected, err=%v", err)
	}
}

func TestResourceRelationClassMustResolveToSelectedExactPath(t *testing.T) {
	analysis := strings.Replace(resourceAnalysis,
		`{"symbol":"OrderController","path":"src/main/java/OrderController.java","role":"Controller","source":"FIND_SYMBOL"},`,
		`{"symbol":"OrderController","path":"module-b/src/main/java/OrderController.java","role":"Controller","source":"FIND_SYMBOL"},`, 1)
	if _, err := reviewscope.Verify(targetedResourceSelection(), []byte(analysis)); err == nil || !strings.Contains(err.Error(), "selected exact path") {
		t.Fatalf("CLASS relation evidence from another module must not bind by simple class name, err=%v", err)
	}
}
