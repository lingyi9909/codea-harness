package schema

import (
	"os"
	"strings"
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
				"request":{"fields":[{"name":"orderId","type":"Long","location":"PATH","required":true,"description":"订单ID","validation":["@Min(1)"],"enumValues":[]}],"example":{"orderId":10001}},
				"response":{"fields":[{"name":"code","type":"Integer","description":"状态码","enumValues":[]}],"example":{"code":0}},
				"permissions":[],
				"preconditions":[{"statement":"订单状态必须为 WAIT_APPROVE","status":"CONFIRMED","evidence":["OrderServiceImpl.java:128"]}],
				"businessFlow":[],"stateTransitions":[],"dataEffects":[],"externalEffects":[],"transactions":[],"idempotency":[],
				"errorCodes":[{"code":"ORDER_NOT_FOUND","scenario":"订单不存在","status":"CONFIRMED","evidence":["OrderServiceImpl.java:120"]}],
				"testCoverage":[],
				"businessNotes":[{"statement":"只有待审核订单可以操作","status":"CONFIRMED","evidence":["OrderServiceImpl.java:128"]}]
			}]
		}]
	}`)
}

func loadAPIDocSchema(t *testing.T) []byte { t.Helper(); b,err:=os.ReadFile("../../../contracts/api-doc.schema.json"); if err!=nil{t.Fatal(err)}; return b }
func TestA1ValidAPIDocPasses(t *testing.T){ if err:=ValidateJSON(loadAPIDocSchema(t),validAPIDocJSON());err!=nil{t.Fatalf("valid API doc rejected: %v",err)} }
func TestA2MissingHTTPMethodFails(t *testing.T){ in:=strings.Replace(string(validAPIDocJSON()),`"httpMethod":"POST",`,"",1); if ValidateJSON(loadAPIDocSchema(t),[]byte(in))==nil{t.Fatal("missing httpMethod must fail")} }
func TestA3EmptyPathFails(t *testing.T){ in:=strings.Replace(string(validAPIDocJSON()),`"path":"/api/order/approve"`,`"path":""`,1); if ValidateJSON(loadAPIDocSchema(t),[]byte(in))==nil{t.Fatal("empty path must fail")} }
func TestA4MalformedFieldFails(t *testing.T){ in:=strings.Replace(string(validAPIDocJSON()),`"type":"Long",`,"",1); if ValidateJSON(loadAPIDocSchema(t),[]byte(in))==nil{t.Fatal("field missing type must fail")} }
func TestA5MalformedErrorCodeFails(t *testing.T){ in:=strings.Replace(string(validAPIDocJSON()),`"code":"ORDER_NOT_FOUND"`,`"code":null`,1); if ValidateJSON(loadAPIDocSchema(t),[]byte(in))==nil{t.Fatal("malformed error code must fail")} }
func TestA6AdditionalPropertiesFail(t *testing.T){ in:=strings.Replace(string(validAPIDocJSON()),`"title":"审核订单",`,`"title":"审核订单","unexpected":true,`,1); if ValidateJSON(loadAPIDocSchema(t),[]byte(in))==nil{t.Fatal("additional properties must fail")} }
func TestRequestLocationRequiredAndEnumerated(t *testing.T){ missing:=strings.Replace(string(validAPIDocJSON()),`"location":"PATH",`,"",1);if ValidateJSON(loadAPIDocSchema(t),[]byte(missing))==nil{t.Fatal("missing request location must fail")};invalid:=strings.Replace(string(validAPIDocJSON()),`"location":"PATH"`,`"location":"COOKIE"`,1);if ValidateJSON(loadAPIDocSchema(t),[]byte(invalid))==nil{t.Fatal("invalid request location must fail")} }
func TestEvidenceBackedSemanticRequiresEvidenceWhenConfirmedOrInferred(t *testing.T){ in:=strings.Replace(string(validAPIDocJSON()),`"evidence":["OrderServiceImpl.java:128"]`,`"evidence":[]`,1); if ValidateJSON(loadAPIDocSchema(t),[]byte(in))==nil{t.Fatal("confirmed semantic without evidence must fail")} }
func TestUnknownSemanticMayHaveNoEvidence(t *testing.T){ in:=strings.Replace(string(validAPIDocJSON()),`"status":"CONFIRMED","evidence":["OrderServiceImpl.java:128"]`,`"status":"UNKNOWN","evidence":[]`,1); if err:=ValidateJSON(loadAPIDocSchema(t),[]byte(in));err!=nil{t.Fatalf("UNKNOWN semantic with empty evidence should pass: %v",err)} }
