package reviewselection

import (
	analysisruntime "codea-harness-tools/internal/analysis"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func exactOptionFixture(t *testing.T, id, branch, refPath string) ChainOption {
	t.Helper()
	var option ChainOption
	data := `{"chainId":"` + id + `","entryPoints":["OrderController.approve"],"source":"ACCEPTED","status":"VALID","callChain":{"entryPoint":"OrderController.approve","chain":["OrderController.approve","` + branch + `"],"entryPointRef":{"path":"src/OrderController.java","symbol":"OrderController.approve"},"chainRefs":[{"path":"src/OrderController.java","symbol":"OrderController.approve"},{"path":"` + refPath + `","symbol":"` + branch + `"}]}}`
	if err := json.Unmarshal([]byte(data), &option); err != nil {
		t.Fatal(err)
	}
	return option
}

func TestReviewOptionsCountSemanticBranchesAndDeduplicateBusinessContexts(t *testing.T) {
	first := exactOptionFixture(t, "a-context", "OrderService.approve", "src/OrderService.java")
	duplicate := exactOptionFixture(t, "b-context", "OrderService.approve", "src/OrderService.java")
	branch := exactOptionFixture(t, "c-context", "AuditService.record", "src/AuditService.java")
	in := Options{RunID: "r153", ChangeSetSHA256: strings.Repeat("b", 64), EntrypointCompleteness: "COMPLETE", Chains: []ChainOption{duplicate, branch, first}}
	got, err := finalizeOptions153(in, strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	if got.Decision != DecisionUser || len(got.Chains) != 2 {
		t.Fatalf("want 2 semantic branch options, got %+v", got)
	}
	in.Chains = []ChainOption{first, branch, duplicate}
	reordered, err := finalizeOptions153(in, strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, reordered) {
		t.Fatalf("input order changed options or hash")
	}
	in.Chains = []ChainOption{first, duplicate}
	single, err := finalizeOptions153(in, strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	if single.Decision != DecisionAutoSingle || len(single.Chains) != 1 {
		t.Fatalf("duplicate contexts inflated options: %+v", single)
	}
}

func TestReviewOptionsHashBindsExactBranchAndRefs(t *testing.T) {
	in := Options{RunID: "r153", ChangeSetSHA256: strings.Repeat("b", 64), EntrypointCompleteness: "COMPLETE"}
	hashes := map[string]bool{}
	for _, option := range []ChainOption{
		exactOptionFixture(t, "same-context", "OrderService.approve", "src/OrderService.java"),
		exactOptionFixture(t, "same-context", "AuditService.record", "src/AuditService.java"),
		exactOptionFixture(t, "same-context", "OrderService.approve", "other/OrderService.java"),
	} {
		in.Chains = []ChainOption{option}
		got, err := finalizeOptions153(in, strings.Repeat("a", 64))
		if err != nil {
			t.Fatal(err)
		}
		if hashes[got.OptionsHash] {
			t.Fatal("call-chain branch/ref mutation did not change options hash")
		}
		hashes[got.OptionsHash] = true
	}
}

func TestMatchingCallChainsPreservesExactRefs(t *testing.T) {
	var current analysisruntime.CallChain
	if err := json.Unmarshal([]byte(`{"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderService.approve"],"entryPointRef":{"path":"src/OrderController.java","symbol":"OrderController.approve"},"chainRefs":[{"path":"src/OrderController.java","symbol":"OrderController.approve"},{"path":"src/OrderService.java","symbol":"OrderService.approve"}]}`), &current); err != nil {
		t.Fatal(err)
	}
	got := matchingCallChains153("OrderService.approve", []analysisruntime.CallChain{current})
	if len(got) != 1 || !reflect.DeepEqual(got[0].EntryPointRef, current.EntryPointRef) || !reflect.DeepEqual(got[0].ChainRefs, current.ChainRefs) {
		t.Fatalf("target filtering lost exact refs: %+v", got)
	}
}
