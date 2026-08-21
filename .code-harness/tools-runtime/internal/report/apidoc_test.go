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
      "apiDoc":{
        "controllers":[{
          "name":"OrderController",
          "apis":[{
            "title":"Approve order",
            "httpMethod":"POST",
            "path":"/orders/{id}/approve",
            "description":"Approve an order",
            "request":{
              "fields":[
                {"name":"id","type":"Long","required":true,"description":"Order id","validation":["@PathVariable"],"enumValues":[]},
                {"name":"reason","type":"String","required":false,"description":"Optional reason","validation":["@RequestParam(required = false)"],"enumValues":[]}
              ],
              "example":{"id":1001,"reason":"ok"}
            },
            "response":{
              "fields":[
                {"name":"success","type":"Boolean","description":"Whether approval succeeded","enumValues":[]}
              ],
              "example":{"success":true}
            },
            "permissions":[{"statement":"Requires order:approve","status":"CONFIRMED","evidence":["OrderController.java:18"]}],
            "preconditions":[],
            "businessFlow":[{"statement":"Delegates to OrderService.approve","status":"CONFIRMED","evidence":["OrderController.java:24"]}],
            "stateTransitions":[],
            "dataEffects":[],
            "externalEffects":[],
            "transactions":[],
            "idempotency":[],
            "errorCodes":[{"code":"ORDER_NOT_FOUND","scenario":"Order does not exist","status":"CONFIRMED","evidence":["OrderService.java:41"]}],
            "testCoverage":[],
            "businessNotes":[]
          }]
        }]
      }
    }`
}

func TestDecodeApiDocRequestAndRenderDeterministically(t *testing.T) {
    req, err := DecodeApiDocRequest([]byte(validApiDocRequestJSON()))
    if err != nil {
        t.Fatal(err)
    }
    got, err := RenderApiDoc(req)
    if err != nil {
        t.Fatal(err)
    }
    required := []string{
        "# Frontend API Documentation",
        "Harness Version: 1.2.0",
        "## OrderController",
        "### POST /orders/{id}/approve",
        "Approve order",
        "#### Request",
        "@PathVariable",
        "#### Response",
        "#### Permissions",
        "Requires order:approve",
        "OrderController.java:18",
        "#### Business Flow",
        "OrderService.approve",
        "#### Error Codes",
        "ORDER_NOT_FOUND",
    }
    for _, s := range required {
        if !strings.Contains(got, s) {
            t.Fatalf("rendered api doc missing %q:\n%s", s, got)
        }
    }
    forbidden := []string{"#### Preconditions", "#### Transactions", "#### Test Coverage"}
    for _, s := range forbidden {
        if strings.Contains(got, s) {
            t.Fatalf("empty section %q must be omitted:\n%s", s, got)
        }
    }
}

func TestDecodeApiDocRequestRejectsContractViolation(t *testing.T) {
    bad := strings.Replace(validApiDocRequestJSON(), `"httpMethod":"POST"`, `"httpMethod":""`, 1)
    if _, err := DecodeApiDocRequest([]byte(bad)); err == nil {
        t.Fatal("expected api-doc schema violation")
    }
}

func TestWriteApiDocRequestFileCreatesArtifactAndRemovesTransport(t *testing.T) {
    root := t.TempDir()
    contracts := filepath.Join(root, ".code-harness", "contracts")
    requests := filepath.Join(root, ".code-harness", "runs", "api-doc-test-1", "requests")
    if err := os.MkdirAll(contracts, 0o755); err != nil { t.Fatal(err) }
    if err := os.MkdirAll(requests, 0o755); err != nil { t.Fatal(err) }

    schemaBytes, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "api-doc.schema.json"))
    if err != nil { t.Fatal(err) }
    if err := os.WriteFile(filepath.Join(contracts, "api-doc.schema.json"), schemaBytes, 0o600); err != nil { t.Fatal(err) }

    input := filepath.Join(requests, "transport.json")
    if err := os.WriteFile(input, []byte(validApiDocRequestJSON()), 0o600); err != nil { t.Fatal(err) }

    path, err := WriteApiDocRequestFile(root, input)
    if err != nil { t.Fatal(err) }
    if filepath.Base(path) != "api-doc.md" { t.Fatalf("artifact=%s", path) }
    if _, err := os.Stat(path); err != nil { t.Fatalf("api-doc.md missing: %v", err) }
    if _, err := os.Stat(input); !os.IsNotExist(err) { t.Fatalf("transport must be removed after success, err=%v", err) }
}

func TestWriteApiDocRequestFileRejectsRunMismatch(t *testing.T) {
    root := t.TempDir()
    contracts := filepath.Join(root, ".code-harness", "contracts")
    requests := filepath.Join(root, ".code-harness", "runs", "other-run", "requests")
    if err := os.MkdirAll(contracts, 0o755); err != nil { t.Fatal(err) }
    if err := os.MkdirAll(requests, 0o755); err != nil { t.Fatal(err) }

    schemaBytes, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "api-doc.schema.json"))
    if err != nil { t.Fatal(err) }
    if err := os.WriteFile(filepath.Join(contracts, "api-doc.schema.json"), schemaBytes, 0o600); err != nil { t.Fatal(err) }

    input := filepath.Join(requests, "transport.json")
    if err := os.WriteFile(input, []byte(validApiDocRequestJSON()), 0o600); err != nil { t.Fatal(err) }
    if _, err := WriteApiDocRequestFile(root, input); err == nil {
        t.Fatal("expected run/path mismatch rejection")
    }
}
