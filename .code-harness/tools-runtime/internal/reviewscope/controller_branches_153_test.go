package reviewscope_test

import (
	"strings"
	"testing"

	"codea-harness-tools/internal/reviewscope"
)

func Test153ExplicitControllerMethodRequiresAllConfirmedBranches(t *testing.T) {
	analysis := []byte(`{
	  "changedFiles":[
	    {"path":"src/main/java/OrderController.java"},
	    {"path":"src/main/java/OrderService.java"},
	    {"path":"src/main/java/AuditService.java"}
	  ],
	  "callChains":[
	    {"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderService.approve"]},
	    {"entryPoint":"OrderController.approve","chain":["OrderController.approve","AuditService.record"]}
	  ],
	  "symbolLocations":[
	    {"symbol":"OrderController.approve","path":"src/main/java/OrderController.java","role":"Controller","source":"FIND_SYMBOL"},
	    {"symbol":"OrderService.approve","path":"src/main/java/OrderService.java","role":"Service","source":"FIND_SYMBOL"},
	    {"symbol":"AuditService.record","path":"src/main/java/AuditService.java","role":"Service","source":"FIND_SYMBOL"}
	  ],
	  "reviewCoverage":{
	    "reviewedFiles":[
	      {"path":"src/main/java/OrderController.java"},
	      {"path":"src/main/java/OrderService.java"},
	      {"path":"src/main/java/AuditService.java"}
	    ],
	    "unresolvedSymbols":[]
	  }
	}`)

	partial := []byte(`{
	  "mode":"TARGETED",
	  "target":{"symbol":"OrderController.approve","kind":"METHOD"},
	  "selectedCallChains":[
	    {"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderService.approve"]}
	  ],
	  "scopedFiles":["src/main/java/OrderController.java","src/main/java/OrderService.java"]
	}`)
	if _, err := reviewscope.Verify(partial, analysis); err == nil || !strings.Contains(err.Error(), "must include all confirmed Controller chains") {
		t.Fatalf("explicit Controller method must reject omitted branch, err=%v", err)
	}

	complete := []byte(`{
	  "mode":"TARGETED",
	  "target":{"symbol":"OrderController.approve","kind":"METHOD"},
	  "selectedCallChains":[
	    {"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderService.approve"]},
	    {"entryPoint":"OrderController.approve","chain":["OrderController.approve","AuditService.record"]}
	  ],
	  "scopedFiles":["src/main/java/AuditService.java","src/main/java/OrderController.java","src/main/java/OrderService.java"]
	}`)
	if _, err := reviewscope.Verify(complete, analysis); err != nil {
		t.Fatalf("explicit Controller method with every confirmed branch must pass: %v", err)
	}
}
