package nav

import (
	"context"
	"strings"
	"testing"
)

type receiverTypeRunner struct{}

func (receiverTypeRunner) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	p := ""
	for i, a := range args {
		if a == "--pattern" && i+1 < len(args) { p = args[i+1] }
	}
	types := `{"file":"src/main/java/Facade.java","range":{"start":{"line":0},"end":{"line":30}},"text":"public class Facade { }"}`+"\n"
	method := `{"file":"src/main/java/Facade.java","range":{"start":{"line":8},"end":{"line":20}},"text":"public void submit() { orderService.approve(); auditService.approve(); unknownService.approve(); }"}`+"\n"
	calls := `{"file":"src/main/java/Facade.java","range":{"start":{"line":10},"end":{"line":10}},"text":"orderService.approve()"}`+"\n"+
		`{"file":"src/main/java/Facade.java","range":{"start":{"line":11},"end":{"line":11}},"text":"auditService.approve()"}`+"\n"+
		`{"file":"src/main/java/Facade.java","range":{"start":{"line":12},"end":{"line":12}},"text":"unknownService.approve()"}`+"\n"
	switch {
	case strings.Contains(p, "$OBJ.approve"):
		return []byte(calls), nil
	case strings.Contains(p, "$M($$$ARGS)"):
		return []byte(method), nil
	case strings.Contains(p, "class $C"):
		return []byte(types), nil
	case strings.Contains(p, "orderService"):
		return []byte(`{"file":"src/main/java/Facade.java","range":{"start":{"line":2},"end":{"line":2}},"text":"private final OrderService orderService;"}`+"\n"), nil
	case strings.Contains(p, "auditService"):
		return []byte(`{"file":"src/main/java/Facade.java","range":{"start":{"line":3},"end":{"line":3}},"text":"private final AuditService auditService;"}`+"\n"), nil
	}
	return nil, nil
}

func TestFindCallersConfirmsReceiverTypeAndRejectsSameNamedOtherService(t *testing.T) {
	n := Navigator{AstGrepPath:"ast-grep", Runner:receiverTypeRunner{}}
	got, err := n.FindCallers(context.Background(), "OrderService.approve", "src/main/java")
	if err != nil { t.Fatal(err) }
	if len(got.Callers) != 1 || got.Callers[0].Line != 11 || got.Callers[0].ReceiverType != "OrderService" || got.Callers[0].Resolution != "CONFIRMED" {
		t.Fatalf("confirmed callers=%+v", got.Callers)
	}
	if len(got.Candidates) != 1 || got.Candidates[0].Receiver != "unknownService" || got.Candidates[0].Resolution != "CANDIDATE" {
		t.Fatalf("candidates=%+v", got.Candidates)
	}
}

type springSyntaxRunner struct{}

func (springSyntaxRunner) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	p := ""
	for i,a := range args { if a=="--pattern" && i+1<len(args) { p=args[i+1] } }
	if strings.Contains(p, "OrderController") {
		return []byte(`{"file":"OrderController.java","range":{"start":{"line":0},"end":{"line":30}},"text":"public class OrderController { }"}`+"\n"), nil
	}
	if strings.Contains(p, "get($$$ARGS)") {
		text := "@PostMapping(\n  value = \"/approve\"\n)\npublic Result<OrderVO> get(\n  @RequestParam(\"id\") Long id,\n  @RequestParam(required = false) String reason,\n  @PathVariable(\"orderId\") Long orderId) { return null; }"
		return []byte(`{"file":"OrderController.java","range":{"start":{"line":4},"end":{"line":12}},"text":`+strconvQuote(text)+`}`+"\n"), nil
	}
	return nil,nil
}

func strconvQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r { case '\\': b.WriteString("\\\\"); case '"': b.WriteString("\\\""); case '\n': b.WriteString("\\n"); case '\r': b.WriteString("\\r"); case '\t': b.WriteString("\\t"); default: b.WriteRune(r) }
	}
	b.WriteByte('"')
	return b.String()
}

func TestGetSymbolInfoBalancesSpringParameterAnnotationsAndMultilineMapping(t *testing.T) {
	n := Navigator{AstGrepPath:"ast-grep", Runner:springSyntaxRunner{}}
	got, err := n.GetSymbolInfo(context.Background(), "OrderController.get", "src/main/java")
	if err != nil { t.Fatal(err) }
	wantSig := `get(@RequestParam("id") Long id, @RequestParam(required = false) String reason, @PathVariable("orderId") Long orderId)`
	if got.Signature != wantSig { t.Fatalf("signature=%q want=%q", got.Signature, wantSig) }
	if len(got.Annotations)!=1 || got.Annotations[0] != "@PostMapping( value = \"/approve\" )" { t.Fatalf("annotations=%v", got.Annotations) }
}
