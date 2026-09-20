package schema

import (
	"os"
	"testing"
)

func TestReviewScopeSchemaFullAndTargeted(t *testing.T) {
	schemaBytes, err := os.ReadFile("../../../contracts/review-scope.schema.json")
	if err != nil {
		t.Fatal(err)
	}

	valid := [][]byte{
		[]byte(`{"mode":"FULL","selectedCallChains":[],"scopedFiles":[]}`),
		[]byte(`{
		  "mode":"TARGETED",
		  "target":{"symbol":"OrderController.approve","kind":"METHOD"},
		  "selectedCallChains":[{"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderService.approve"]}],
		  "scopedFiles":["src/main/java/OrderController.java","src/main/java/OrderService.java"]
		}`),
	}
	for i, doc := range valid {
		if err := ValidateJSON(schemaBytes, doc); err != nil {
			t.Fatalf("valid review scope %d rejected: %v", i, err)
		}
	}

	invalid := [][]byte{
		[]byte(`{"mode":"TARGETED","selectedCallChains":[{"entryPoint":"OrderController.approve","chain":["OrderController.approve"]}],"scopedFiles":["src/main/java/OrderController.java"]}`),
		[]byte(`{"mode":"TARGETED","target":{"symbol":"OrderController","kind":"PACKAGE"},"selectedCallChains":[{"entryPoint":"OrderController.approve","chain":["OrderController.approve"]}],"scopedFiles":["src/main/java/OrderController.java"]}`),
		[]byte(`{"mode":"TARGETED","target":{"symbol":"OrderController.approve","kind":"METHOD"},"selectedCallChains":[],"scopedFiles":["src/main/java/OrderController.java"]}`),
		[]byte(`{"mode":"TARGETED","target":{"symbol":"OrderController.approve","kind":"METHOD"},"selectedCallChains":[{"entryPoint":"OrderController.approve","chain":["OrderController.approve"]}],"scopedFiles":[]}`),
		[]byte(`{"mode":"TARGETED","target":{"symbol":"OrderController.approve","kind":"METHOD"},"selectedCallChains":[{"entryPoint":"OrderController.approve","chain":["OrderController.approve"]}],"scopedFiles":["src/main/java/OrderController.java"],"unexpected":true}`),
	}
	for i, doc := range invalid {
		if err := ValidateJSON(schemaBytes, doc); err == nil {
			t.Fatalf("invalid review scope %d accepted: %s", i, doc)
		}
	}
}
