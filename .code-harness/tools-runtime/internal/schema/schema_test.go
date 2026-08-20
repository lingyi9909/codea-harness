package schema

import (
	"os"
	"testing"
)

func TestValidateJSONContract(t *testing.T) {
	s := []byte(`{"type":"object","additionalProperties":false,"required":["name","items"],"properties":{"name":{"type":"string","minLength":1},"items":{"type":"array","minItems":1,"items":{"type":"object","required":["id"],"properties":{"id":{"type":"integer"}}}}}}`)
	if err := ValidateJSON(s, []byte(`{"name":"ok","items":[{"id":1}]}`)); err != nil {
		t.Fatal(err)
	}
	if err := ValidateJSON(s, []byte(`{"name":"ok","items":[{}]}`)); err == nil {
		t.Fatal("missing nested required field should fail")
	}
	if err := ValidateJSON(s, []byte(`{"name":"ok","items":[{"id":1}],"extra":true}`)); err == nil {
		t.Fatal("additional property should fail")
	}
}

func TestValidateJSONLocalRef(t *testing.T) {
	s := []byte(`{"type":"object","required":["role"],"properties":{"role":{"$ref":"#/$defs/role"}},"$defs":{"role":{"enum":["Controller","Service"]}}}`)
	if err := ValidateJSON(s, []byte(`{"role":"Service"}`)); err != nil {
		t.Fatal(err)
	}
	if err := ValidateJSON(s, []byte(`{"role":"Other"}`)); err == nil {
		t.Fatal("enum ref should reject")
	}
}

func TestValidateJSONDraft202012OneOfAndPattern(t *testing.T) {
	s := []byte(`{
		"$schema":"https://json-schema.org/draft/2020-12/schema",
		"oneOf":[
			{"type":"string","pattern":"^ORD-[0-9]+$"},
			{"type":"integer","minimum":1}
		]
	}`)
	if err := ValidateJSON(s, []byte(`"ORD-42"`)); err != nil {
		t.Fatal(err)
	}
	if err := ValidateJSON(s, []byte(`"bad"`)); err == nil {
		t.Fatal("pattern/oneOf violation should fail")
	}
}

func TestValidateJSONDraft202012UnevaluatedProperties(t *testing.T) {
	s := []byte(`{
		"$schema":"https://json-schema.org/draft/2020-12/schema",
		"allOf":[{"properties":{"name":{"type":"string"}},"required":["name"]}],
		"unevaluatedProperties":false
	}`)
	if err := ValidateJSON(s, []byte(`{"name":"ok"}`)); err != nil {
		t.Fatal(err)
	}
	if err := ValidateJSON(s, []byte(`{"name":"ok","unexpected":true}`)); err == nil {
		t.Fatal("unevaluatedProperties should reject unexpected field")
	}
}

func TestValidateYAMLUsesRealParserAndValidatesNestedArrayItems(t *testing.T) {
	s := []byte(`{
		"$schema":"https://json-schema.org/draft/2020-12/schema",
		"type":"object",
		"required":["services"],
		"properties":{
			"services":{
				"type":"array",
				"minItems":1,
				"items":{
					"type":"object",
					"required":["name","enabled"],
					"properties":{
						"name":{"type":"string","pattern":"^[a-z-]+$"},
						"enabled":{"type":"boolean"}
					}
				}
			}
		}
	}`)
	valid := []byte("services:\n  - name: order-service\n    enabled: true\n")
	if err := ValidateYAML(s, valid); err != nil {
		t.Fatal(err)
	}
	invalid := []byte("services:\n  - name: ORDER_SERVICE\n    enabled: true\n")
	if err := ValidateYAML(s, invalid); err == nil {
		t.Fatal("nested YAML item must be validated against JSON Schema")
	}
}

func TestInvalidDraft202012SchemaIsRejectedAtCompileTime(t *testing.T) {
	invalidSchema := []byte(`{
		"$schema":"https://json-schema.org/draft/2020-12/schema",
		"type":123
	}`)
	if err := ValidateJSON(invalidSchema, []byte(`{}`)); err == nil {
		t.Fatal("invalid JSON Schema must fail compilation")
	}
}

func TestTargetSelectionSchema(t *testing.T) {
	schemaBytes, err := os.ReadFile("../../../contracts/test-target-selection.schema.json")
	if err != nil {
		t.Fatal(err)
	}

	valid := []byte(`{"selectionId":"sel-001","status":"SELECTED","mode":"USER_MULTI","selectedControllerIds":["controller:OrderController"],"availableControllerIds":["controller:OrderController","controller:PaymentController"]}`)
	if err := ValidateJSON(schemaBytes, valid); err != nil {
		t.Fatalf("valid selection rejected: %v", err)
	}

	cases := []string{
		`{"selectionId":"sel-002","status":"SELECTED","mode":"USER_MULTI","selectedControllerIds":[],"availableControllerIds":["controller:OrderController"]}`,
		`{"selectionId":"sel-003","status":"SELECTED","mode":"UNKNOWN","selectedControllerIds":["controller:OrderController"],"availableControllerIds":["controller:OrderController"]}`,
		`{"selectionId":"sel-004","status":"SELECTED","mode":"USER_MULTI","selectedControllerIds":["controller:OrderController","controller:OrderController"],"availableControllerIds":["controller:OrderController"]}`,
		`{"selectionId":"sel-005","status":"SELECTED","mode":"USER_MULTI","selectedControllerIds":["controller:OrderController"],"availableControllerIds":["controller:OrderController","controller:OrderController"]}`,
	}
	for _, input := range cases {
		if err := ValidateJSON(schemaBytes, []byte(input)); err == nil {
			t.Fatalf("invalid selection accepted by schema: %s", input)
		}
	}

	cancelled := []byte(`{"selectionId":"sel-006","status":"CANCELLED","mode":"USER_MULTI","selectedControllerIds":[],"availableControllerIds":["controller:OrderController","controller:PaymentController"]}`)
	if err := ValidateJSON(schemaBytes, cancelled); err != nil {
		t.Fatalf("valid cancelled selection rejected: %v", err)
	}
}

func TestDiagnosisSchemaEvidenceExtensions(t *testing.T) {
	schemaBytes, err := os.ReadFile("../../../contracts/diagnosis.schema.json")
	if err != nil {
		t.Fatal(err)
	}

	valid := []byte(`{
		"classification":"PRODUCTION_CODE_ERROR",
		"rootCause":"Order state is not persisted before returning",
		"evidence":["expected APPROVED, actual PENDING"],
		"failedTests":[{"testClass":"OrderControllerIT","testMethod":"shouldApprove"}],
		"suspectSymbols":["OrderServiceImpl.approve"],
		"codeEvidence":[{"path":"src/main/java/com/acme/OrderServiceImpl.java","symbol":"OrderServiceImpl.approve","lineStart":178,"lineEnd":190,"reason":"stack trace target"}],
		"databaseEvidence":["dbq-002"],
		"externalDependencies":["PaymentRpcClient"],
		"nextAction":"GENERATE_FIX_PLAN"
	}`)
	if err := ValidateJSON(schemaBytes, valid); err != nil {
		t.Fatalf("valid evidence-backed diagnosis rejected: %v", err)
	}

	invalid := []string{
		`{"classification":"UNKNOWN","rootCause":"x","evidence":["x"],"codeEvidence":[{"path":"a.java","symbol":"A.m","lineStart":0,"lineEnd":1,"reason":"x"}],"nextAction":"STOP_UNKNOWN"}`,
		`{"classification":"UNKNOWN","rootCause":"x","evidence":["x"],"codeEvidence":[{"path":"a.java","symbol":"A.m","lineStart":10,"lineEnd":9,"reason":"x"}],"nextAction":"STOP_UNKNOWN"}`,
		`{"classification":"UNKNOWN","rootCause":"x","evidence":["x"],"suspectSymbols":[""],"nextAction":"STOP_UNKNOWN"}`,
		`{"classification":"UNKNOWN","rootCause":"x","evidence":["x"],"databaseEvidence":[""],"nextAction":"STOP_UNKNOWN"}`,
	}
	for _, input := range invalid {
		if err := ValidateJSON(schemaBytes, []byte(input)); err == nil {
			t.Fatalf("invalid diagnosis accepted: %s", input)
		}
	}
}
