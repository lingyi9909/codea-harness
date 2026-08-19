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
