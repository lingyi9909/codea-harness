package report

import (
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func validApiDocRequestJSON() string {
    return `{
      "runId":"api-doc-test-1",
      "harnessVersion":"1.2.0",
      "apiDoc":{"controllers":[{"name":"OrderController","apis":[{
        "title":"Approve order","httpMethod":"POST","path":"/orders/{id}/approve","description":"Approve an order",
        "request":{"fields":[
          {"name":"id","type":"Long","location":"PATH","required":true,"description":"Order id","validation":[],"enumValues":[]},
          {"name":"reason","type":"String","location":"QUERY","required":false,"description":"Optional reason","validation":["@Size(max=200)"],"enumValues":[]},
          {"name":"tenantId","type":"String","location":"HEADER","required":true,"description":"Tenant id","validation":["@NotBlank"],"enumValues":[]},
          {"name":"request","type":"ApproveRequest","location":"BODY","required":true,"description":"Request body","validation":["@Valid"],"enumValues":[]}
        ],"example":{"id":1001,"reason":"ok"}},
        "response":{"fields":[{"name":"success","type":"Boolean","description":"Whether approval succeeded","enumValues":[]}],"example":{"success":true}},
        "permissions":[{"statement":"Requires order:approve","status":"CONFIRMED","evidence":["OrderController.java:18"]}],
        "preconditions":[],"businessFlow":[{"statement":"Delegates to OrderService.approve","status":"CONFIRMED","evidence":["OrderController.java:24"]}],
        "stateTransitions":[],"dataEffects":[],"externalEffects":[],"transactions":[],"idempotency":[],
        "errorCodes":[{"code":"ORDER_NOT_FOUND","scenario":"Order does not exist","status":"CONFIRMED","evidence":["OrderService.java:41"]}],
        "testCoverage":[],"businessNotes":[]
      }]}]}}
    `
}

func TestDecodeApiDocRequestAndRenderDeterministically(t *testing.T) {
    req,err:=DecodeApiDocRequest([]byte(validApiDocRequestJSON())); if err!=nil{t.Fatal(err)}
    got,err:=RenderApiDoc(req); if err!=nil{t.Fatal(err)}
    required:=[]string{"# Frontend API Documentation","Harness Version: 1.2.0","## OrderController","### POST /orders/{id}/approve","| Field | Location | Type | Required | Validation | Description |","| id | PATH | Long | true |  | Order id |","| reason | QUERY | String | false | @Size(max=200) | Optional reason |","| tenantId | HEADER | String | true | @NotBlank | Tenant id |","| request | BODY | ApproveRequest | true | @Valid | Request body |","#### Response","#### Permissions","Requires order:approve","#### Error Codes","ORDER_NOT_FOUND"}
    for _,s:=range required{if !strings.Contains(got,s){t.Fatalf("rendered api doc missing %q:\n%s",s,got)}}
    for _,s:=range []string{"@PathVariable","@RequestParam","@RequestHeader","@RequestBody"}{if strings.Contains(got,s){t.Fatalf("transport annotation must not be rendered as validation: %s",got)}}
    for _,s:=range []string{"#### Preconditions","#### Transactions","#### Test Coverage"}{if strings.Contains(got,s){t.Fatalf("empty section %q must be omitted",s)}}
}

func TestDecodeApiDocRequestRejectsMissingOrInvalidLocation(t *testing.T){
    missing:=strings.Replace(validApiDocRequestJSON(), `,"location":"PATH"`, "",1); if _,err:=DecodeApiDocRequest([]byte(missing));err==nil{t.Fatal("expected missing location rejection")}
    invalid:=strings.Replace(validApiDocRequestJSON(), `"location":"PATH"`, `"location":"COOKIE"`,1); if _,err:=DecodeApiDocRequest([]byte(invalid));err==nil{t.Fatal("expected invalid location rejection")}
}
func TestDecodeApiDocRequestRejectsContractViolation(t *testing.T){bad:=strings.Replace(validApiDocRequestJSON(),`"httpMethod":"POST"`,`"httpMethod":""`,1);if _,err:=DecodeApiDocRequest([]byte(bad));err==nil{t.Fatal("expected api-doc contract violation")}}
func TestWriteApiDocRequestFileCreatesArtifactAndRemovesTransport(t *testing.T){root:=t.TempDir();contracts:=filepath.Join(root,".code-harness","contracts");requests:=filepath.Join(root,".code-harness","runs","api-doc-test-1","requests");_ = os.MkdirAll(contracts,0o755);_ = os.MkdirAll(requests,0o755);schemaBytes,err:=os.ReadFile(filepath.Join("..","..","..","contracts","api-doc.schema.json"));if err!=nil{t.Fatal(err)};if err:=os.WriteFile(filepath.Join(contracts,"api-doc.schema.json"),schemaBytes,0o600);err!=nil{t.Fatal(err)};input:=filepath.Join(requests,"transport.json");if err:=os.WriteFile(input,[]byte(validApiDocRequestJSON()),0o600);err!=nil{t.Fatal(err)};path,err:=WriteApiDocRequestFile(root,input);if err!=nil{t.Fatal(err)};if filepath.Base(path)!="api-doc.md"{t.Fatalf("artifact=%s",path)};if _,err:=os.Stat(input);!os.IsNotExist(err){t.Fatalf("transport must be removed, err=%v",err)}}
func TestWriteApiDocRequestFileRejectsRunMismatch(t *testing.T){root:=t.TempDir();contracts:=filepath.Join(root,".code-harness","contracts");requests:=filepath.Join(root,".code-harness","runs","other-run","requests");_ = os.MkdirAll(contracts,0o755);_ = os.MkdirAll(requests,0o755);schemaBytes,err:=os.ReadFile(filepath.Join("..","..","..","contracts","api-doc.schema.json"));if err!=nil{t.Fatal(err)};_ = os.WriteFile(filepath.Join(contracts,"api-doc.schema.json"),schemaBytes,0o600);input:=filepath.Join(requests,"transport.json");_ = os.WriteFile(input,[]byte(validApiDocRequestJSON()),0o600);if _,err:=WriteApiDocRequestFile(root,input);err==nil{t.Fatal("expected run/path mismatch rejection")}}
