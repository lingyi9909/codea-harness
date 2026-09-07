package nav

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type countingEntrypointRunner164 struct {
	calls      int
	noController bool
	outOfScope bool
}

func (r *countingEntrypointRunner164) Run(_ context.Context, _ string, _ ...string) ([]byte, error) {
	r.calls++
	if r.noController {
		return nil, nil
	}
	path := "src/main/java/acme/AController.java"
	if r.outOfScope {
		path = "src/main/java/acme/UnrelatedController.java"
	}
	if r.calls == 1 {
		return []byte(fmt.Sprintf("{\"ruleId\":\"entrypoint-type-000\",\"file\":%q,\"text\":\"@RestController\\nclass AController { }\",\"range\":{\"start\":{\"line\":0,\"column\":0},\"end\":{\"line\":20,\"column\":1}}}\n", path)), nil
	}
	return []byte(fmt.Sprintf("{\"ruleId\":\"entrypoint-method-000\",\"file\":%q,\"text\":\"@GetMapping\\npublic String get() { return \\\"ok\\\"; }\",\"range\":{\"start\":{\"line\":5,\"column\":2},\"end\":{\"line\":8,\"column\":3}}}\n", path)), nil
}

func Test164Task1BBatchEntrypointScannerUsesAtMostTwoAstProcessesPerSide(t *testing.T) {
	runner := &countingEntrypointRunner164{}
	n := Navigator{AstGrepPath: "ast-grep.exe", Runner: runner}

	got, err := n.FindControllerEndpointsBatch(context.Background(), []string{
		"src/main/java/acme/AController.java",
		"src/main/java/acme/AService.java",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Symbol != "AController.get" {
		t.Fatalf("unexpected endpoint result: %+v", got)
	}
	if runner.calls != 2 {
		t.Fatalf("single-side batch entrypoint scan started %d ast-grep processes; want 2", runner.calls)
	}
}

func Test164Task1BNoControllerSkipsMethodBatch(t *testing.T) {
	runner := &countingEntrypointRunner164{noController: true}
	n := Navigator{AstGrepPath: "ast-grep.exe", Runner: runner}
	got, err := n.FindControllerEndpointsBatch(context.Background(), []string{"src/main/java/acme/AService.java"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 || runner.calls != 1 {
		t.Fatalf("no-controller side must use exactly one Type process, got endpoints=%+v calls=%d", got, runner.calls)
	}
}

func Test164Task1BRuleCountsAndStableIDs(t *testing.T) {
	if got := len(allTypePatterns()); got != 30 {
		t.Fatalf("TYPE_PATTERN_COUNT=%d want=30", got)
	}
	if got := len(allMethodPatterns()); got != 78 {
		t.Fatalf("METHOD_PATTERN_COUNT=%d want=78", got)
	}
	typePack := entrypointRulePack164(entrypointTypeRulePrefix164, allTypePatterns())
	methodPack := entrypointRulePack164(entrypointMethodRulePrefix164, allMethodPatterns())
	if !strings.Contains(typePack, "id: entrypoint-type-000") || !strings.Contains(typePack, "id: entrypoint-type-029") {
		t.Fatalf("type rule IDs are not deterministic")
	}
	if !strings.Contains(methodPack, "id: entrypoint-method-000") || !strings.Contains(methodPack, "id: entrypoint-method-077") {
		t.Fatalf("method rule IDs are not deterministic")
	}
}

func Test164Task1BRejectsOutOfScopeAstResult(t *testing.T) {
	runner := &countingEntrypointRunner164{outOfScope: true}
	n := Navigator{AstGrepPath: "ast-grep.exe", Runner: runner}
	_, err := n.FindControllerEndpointsBatch(context.Background(), []string{"src/main/java/acme/AController.java"})
	if err == nil || !strings.Contains(err.Error(), "ENTRYPOINT_SCAN_RESULT_OUT_OF_SCOPE") {
		t.Fatalf("out-of-scope AST result must fail closed, got %v", err)
	}
}
