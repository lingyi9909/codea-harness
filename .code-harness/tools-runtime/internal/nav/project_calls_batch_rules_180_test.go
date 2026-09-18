package nav

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type directCallRuleAwareRunner180 struct{ calls int }

func (r *directCallRuleAwareRunner180) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	r.calls++
	if len(args) < 5 || args[0] != "scan" || args[1] != "--inline-rules" || args[3] != "--json=stream" {
		return nil, errors.New("expected one inline batch scan")
	}
	rules := args[2]
	for _, want := range []string{
		"id: codea-direct-calls-180-types",
		"id: codea-direct-calls-180-methods",
		"id: codea-direct-calls-180-calls",
		"kind: method_invocation",
	} {
		if !strings.Contains(rules, want) {
			return nil, errors.New("missing independent direct-call rule: " + want)
		}
	}
	return []byte(
		`{"file":"src/main/java/com/example/OrderServiceImpl.java","text":"public class OrderServiceImpl implements OrderService {\nprivate final OrderMapper orderMapper;\n@Override\npublic void create() {\norderMapper.insertOrder();\n}\n}","ruleId":"codea-direct-calls-180-types","range":{"start":{"line":0,"column":0},"end":{"line":6,"column":1}}}` + "\n" +
			`{"file":"src/main/java/com/example/OrderServiceImpl.java","text":"@Override\npublic void create() {\norderMapper.insertOrder();\n}","ruleId":"codea-direct-calls-180-methods","range":{"start":{"line":2,"column":0},"end":{"line":5,"column":1}}}` + "\n" +
			`{"file":"src/main/java/com/example/OrderServiceImpl.java","text":"orderMapper.insertOrder()","ruleId":"codea-direct-calls-180-calls","range":{"start":{"line":4,"column":0},"end":{"line":4,"column":25}}}` + "\n",
	), nil
}

func Test180BatchDirectCallsKeepOverrideServiceToMapper(t *testing.T) {
	r := &directCallRuleAwareRunner180{}
	n := Navigator{AstGrepPath: "ast-grep", Runner: r}
	got, err := n.FindDirectMethodCallsBatch180(context.Background(), "src/main/java")
	if err != nil {
		t.Fatal(err)
	}
	if r.calls != 1 {
		t.Fatalf("astGrepProcessCount=%d want=1", r.calls)
	}
	calls := got["OrderServiceImpl.create"]
	if len(calls) != 1 {
		t.Fatalf("calls=%+v", calls)
	}
	if calls[0].TargetSymbol != "OrderMapper.insertOrder" || calls[0].ReceiverType != "OrderMapper" || !calls[0].Resolved {
		t.Fatalf("call=%+v", calls[0])
	}
}
