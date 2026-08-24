package reviewscope_test

import (
	"strings"
	"testing"

	"codea-harness-tools/internal/reviewscope"
)

const evidenceAnalysis = `{
  "changedFiles":[
    {"path":"module-a/src/main/java/OrderController.java"},
    {"path":"module-a/src/main/java/OrderService.java"},
    {"path":"module-b/src/main/java/OrderService.java"}
  ],
  "callChains":[
    {"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderService.approve"]},
    {"entryPoint":"OrderController.cancel","chain":["OrderController.cancel","OrderService.cancel"]},
    {"entryPoint":"AdminController.approve","chain":["AdminController.approve","OrderService.approve"]}
  ],
  "symbolLocations":[
    {"symbol":"OrderController","path":"module-a/src/main/java/OrderController.java","role":"Controller"},
    {"symbol":"OrderController.approve","path":"module-a/src/main/java/OrderController.java","role":"Controller"},
    {"symbol":"OrderController.cancel","path":"module-a/src/main/java/OrderController.java","role":"Controller"},
    {"symbol":"AdminController.approve","path":"module-a/src/main/java/AdminController.java","role":"Controller"},
    {"symbol":"OrderService.approve","path":"module-a/src/main/java/OrderService.java","role":"Service"},
    {"symbol":"OrderService.cancel","path":"module-a/src/main/java/OrderService.java","role":"Service"}
  ],
  "reviewCoverage":{
    "reviewedFiles":[
      {"path":"module-a/src/main/java/OrderController.java"},
      {"path":"module-a/src/main/java/AdminController.java"},
      {"path":"module-a/src/main/java/OrderService.java"},
      {"path":"module-b/src/main/java/OrderService.java"}
    ],
    "unresolvedSymbols":[]
  }
}`

func TestControllerClassMustSelectAllConfirmedTargetChains(t *testing.T) {
	selection := []byte(`{
      "mode":"TARGETED",
      "target":{"symbol":"OrderController","kind":"CLASS"},
      "selectedCallChains":[
        {"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderService.approve"]}
      ],
      "scopedFiles":["module-a/src/main/java/OrderController.java","module-a/src/main/java/OrderService.java"]
    }`)
	if _, err := reviewscope.Verify(selection, []byte(evidenceAnalysis)); err == nil || !strings.Contains(err.Error(), "all confirmed Controller chains") {
		t.Fatalf("Controller CLASS partial chain selection must be rejected, err=%v", err)
	}
}

func TestControllerMethodMustSelectAllConfirmedMethodChains(t *testing.T) {
	analysis := strings.Replace(evidenceAnalysis,
		`{"entryPoint":"OrderController.cancel","chain":["OrderController.cancel","OrderService.cancel"]},`,
		`{"entryPoint":"OrderController.approve","chain":["OrderController.approve","AuditService.record"]},`, 1)
	analysis = strings.Replace(analysis,
		`{"symbol":"OrderController.cancel","path":"module-a/src/main/java/OrderController.java","role":"Controller"},`,
		`{"symbol":"AuditService.record","path":"module-a/src/main/java/AuditService.java","role":"Service"},`, 1)
	selection := []byte(`{
      "mode":"TARGETED",
      "target":{"symbol":"OrderController.approve","kind":"METHOD"},
      "selectedCallChains":[
        {"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderService.approve"]}
      ],
      "scopedFiles":["module-a/src/main/java/OrderController.java","module-a/src/main/java/OrderService.java"]
    }`)
	if _, err := reviewscope.Verify(selection, []byte(analysis)); err == nil || !strings.Contains(err.Error(), "all confirmed Controller chains") {
		t.Fatalf("Controller METHOD partial chain selection must be rejected, err=%v", err)
	}
}

func TestServiceTargetMaySelectOneOfMultipleUpstreamChains(t *testing.T) {
	selection := []byte(`{
      "mode":"TARGETED",
      "target":{"symbol":"OrderService.approve","kind":"METHOD"},
      "selectedCallChains":[
        {"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderService.approve"]}
      ],
      "scopedFiles":["module-a/src/main/java/OrderController.java","module-a/src/main/java/OrderService.java"]
    }`)
	if _, err := reviewscope.Verify(selection, []byte(evidenceAnalysis)); err != nil {
		t.Fatalf("Service target must allow explicit upstream chain selection: %v", err)
	}
}

func TestDuplicateClassInOtherModuleCannotSatisfyExactSymbolPath(t *testing.T) {
	selection := []byte(`{
      "mode":"TARGETED",
      "target":{"symbol":"OrderController.approve","kind":"METHOD"},
      "selectedCallChains":[
        {"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderService.approve"]}
      ],
      "scopedFiles":["module-a/src/main/java/OrderController.java","module-b/src/main/java/OrderService.java"]
    }`)
	if _, err := reviewscope.Verify(selection, []byte(evidenceAnalysis)); err == nil || !strings.Contains(err.Error(), "exact Code Navigation path") {
		t.Fatalf("same basename from wrong module must be rejected, err=%v", err)
	}
}

func TestExactSymbolPathsAllowCorrectModule(t *testing.T) {
	selection := []byte(`{
      "mode":"TARGETED",
      "target":{"symbol":"OrderController.approve","kind":"METHOD"},
      "selectedCallChains":[
        {"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderService.approve"]}
      ],
      "scopedFiles":["module-a/src/main/java/OrderController.java","module-a/src/main/java/OrderService.java"]
    }`)
	if _, err := reviewscope.Verify(selection, []byte(evidenceAnalysis)); err != nil {
		t.Fatalf("exact navigation paths should pass: %v", err)
	}
}
