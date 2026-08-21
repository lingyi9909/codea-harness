package schema

import (
	"os"
	"testing"
)

func validAPIDocJSON() []byte {
	return []byte(`{
		"controllers":[{
			"name":"OrderController",
			"apis":[{
				"title":"审核订单",
				"httpMethod":"POST",
				"path":"/api/order/approve",
				"description":"审核订单",
				"request":{"fields":[{"name":"orderId","type":"Long","required":true,"description":"订单ID","validation":[">= 1"],"enumValues":[]}],"example":{"orderId":10001}},
				"response":{"fields":[{"name":"code","type":"Integer","description":"状态码","enumValues":[]}],"example":{"code":0}},
				"errorCodes":[{"code":"ORDER_NOT_FOUND","scenario":"订单不存在"}],
				"businessNotes":["只有待审核订单可以操作"]
			}]
		}]
	}`)
}

func loadAPIDocSchema(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("../../../contracts/api-doc.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestA1ValidAPIDocPasses(t *testing.T) {
	if err := ValidateJSON(loadAPIDocSchema(t), validAPIDocJSON()); err != nil {
		t.Fatalf("valid API doc rejected: %v", err)
	}
}

func TestA2MissingHTTPMethodFails(t *testing.T) {
	input := []byte(`{"controllers":[{"name":"C","apis":[{"title":"t","path":"/x","description":"d","request":{"fields":[],"example":{}},"response":{"fields":[],"example":{}},"errorCodes":[],"businessNotes":[]}]}]}`)
	if err := ValidateJSON(loadAPIDocSchema(t), input); err == nil {
		t.Fatal("missing httpMethod must fail")
	}
}

func TestA3EmptyPathFails(t *testing.T) {
	input := []byte(`{"controllers":[{"name":"C","apis":[{"title":"t","httpMethod":"GET","path":"","description":"d","request":{"fields":[],"example":{}},"response":{"fields":[],"example":{}},"errorCodes":[],"businessNotes":[]}]}]}`)
	if err := ValidateJSON(loadAPIDocSchema(t), input); err == nil {
		t.Fatal("empty path must fail")
	}
}

func TestA4MalformedFieldFails(t *testing.T) {
	input := []byte(`{"controllers":[{"name":"C","apis":[{"title":"t","httpMethod":"POST","path":"/x","description":"d","request":{"fields":[{"name":"id","required":true,"description":"id","validation":[],"enumValues":[]}],"example":{}},"response":{"fields":[],"example":{}},"errorCodes":[],"businessNotes":[]}]}]}`)
	if err := ValidateJSON(loadAPIDocSchema(t), input); err == nil {
		t.Fatal("field missing type must fail")
	}
}

func TestA5MalformedErrorCodeFails(t *testing.T) {
	input := []byte(`{"controllers":[{"name":"C","apis":[{"title":"t","httpMethod":"POST","path":"/x","description":"d","request":{"fields":[],"example":{}},"response":{"fields":[],"example":{}},"errorCodes":[{"code":"","scenario":"bad"}],"businessNotes":[]}]}]}`)
	if err := ValidateJSON(loadAPIDocSchema(t), input); err == nil {
		t.Fatal("empty error code must fail")
	}
}

func TestA6AdditionalPropertiesFail(t *testing.T) {
	input := []byte(`{"controllers":[{"name":"C","apis":[{"title":"t","httpMethod":"GET","path":"/x","description":"d","request":{"fields":[],"example":{}},"response":{"fields":[],"example":{}},"errorCodes":[],"businessNotes":[],"unexpected":true}]}]}`)
	if err := ValidateJSON(loadAPIDocSchema(t), input); err == nil {
		t.Fatal("additional properties must fail")
	}
}
