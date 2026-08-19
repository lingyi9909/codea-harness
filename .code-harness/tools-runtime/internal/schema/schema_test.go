package schema

import "testing"

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
