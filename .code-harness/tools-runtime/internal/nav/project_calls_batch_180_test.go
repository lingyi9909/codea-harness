package nav

import (
	"context"
	"testing"
)

type directCallBatch180Runner struct{}

func (directCallBatch180Runner) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	if len(args) == 0 || args[0] != "scan" {
		return nil, nil
	}
	return []byte(
		`{"file":"src/main/java/OrderController.java","text":"public class OrderController {\nprivate final OrderService orderService;\npublic void create() {\norderService.create();\n}\n}","ruleId":"codea-direct-calls-180","range":{"start":{"line":0,"column":0},"end":{"line":5,"column":1}}}` + "\n" +
			`{"file":"src/main/java/OrderController.java","text":"public void create() {\norderService.create();\n}","ruleId":"codea-direct-calls-180","range":{"start":{"line":2,"column":0},"end":{"line":4,"column":1}}}` + "\n" +
			`{"file":"src/main/java/OrderController.java","text":"orderService.create()","ruleId":"codea-direct-calls-180","range":{"start":{"line":3,"column":0},"end":{"line":3,"column":21}}}` + "\n",
	), nil
}

func Test180BatchDirectCallIsNotMisclassifiedAsMethod(t *testing.T) {
	n := Navigator{AstGrepPath: "ast-grep", Runner: directCallBatch180Runner{}}
	got, err := n.FindDirectMethodCallsBatch180(context.Background(), "src/main/java")
	if err != nil {
		t.Fatal(err)
	}
	calls := got["OrderController.create"]
	if len(calls) != 1 {
		t.Fatalf("calls=%+v", calls)
	}
	if calls[0].TargetSymbol != "OrderService.create" || calls[0].ReceiverType != "OrderService" || !calls[0].Resolved {
		t.Fatalf("call=%+v", calls[0])
	}
}
