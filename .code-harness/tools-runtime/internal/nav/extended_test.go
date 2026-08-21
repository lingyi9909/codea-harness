package nav

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type routingRunner struct{}

func (routingRunner) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	p := ""
	for i, a := range args {
		if a == "--pattern" && i+1 < len(args) {
			p = args[i+1]
		}
	}
	typeLine := `{"file":"src/main/java/OrderController.java","range":{"start":{"line":0,"column":0},"end":{"line":20,"column":1}},"text":"@RestController\npublic class OrderController { }"}` + "\n"
	enumLine := `{"file":"src/main/java/OrderType.java","range":{"start":{"line":0,"column":0},"end":{"line":4,"column":1}},"text":"public enum OrderType { NORMAL, APPEAL }"}` + "\n"
	methodLine := `{"file":"src/main/java/OrderController.java","range":{"start":{"line":4,"column":4},"end":{"line":7,"column":5}},"text":"@PostMapping(\"/approve\")\npublic Result<Void> approve(ApproveRequest request) { return service.approve(request); }"}` + "\n"
	callerType := `{"file":"src/main/java/OrderFacade.java","range":{"start":{"line":0,"column":0},"end":{"line":15,"column":1}},"text":"public class OrderFacade { }"}` + "\n"
	callerMethod := `{"file":"src/main/java/OrderFacade.java","range":{"start":{"line":4,"column":4},"end":{"line":10,"column":5}},"text":"public void submit() { service.approve(request); }"}` + "\n"
	callLine := `{"file":"src/main/java/OrderFacade.java","range":{"start":{"line":6,"column":8},"end":{"line":6,"column":32}},"text":"service.approve(request)"}` + "\n"
	switch {
	case strings.Contains(p, "$OBJ.approve") || p == "approve($$$ARGS)":
		return []byte(callLine), nil
	case strings.Contains(p, "approve($$$ARGS)"):
		return []byte(methodLine), nil
	case strings.Contains(p, "OrderController"):
		return []byte(typeLine), nil
	case strings.Contains(p, "OrderType"):
		return []byte(enumLine), nil
	case strings.Contains(p, "@PostMapping"):
		return []byte(methodLine), nil
	case strings.Contains(p, "class $C"):
		return []byte(typeLine + callerType), nil
	case strings.Contains(p, "$M($$$ARGS)"):
		return []byte(methodLine + callerMethod), nil
	}
	return nil, nil
}

func TestN1MethodSymbolInfo(t *testing.T) {
	n := Navigator{AstGrepPath: "ast-grep", Runner: routingRunner{}}
	got, err := n.GetSymbolInfo(context.Background(), "OrderController.approve", "src/main/java")
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != "METHOD" || got.DeclaringType != "OrderController" || got.Signature != "approve(ApproveRequest request)" || got.ReturnType != "Result<Void>" {
		t.Fatalf("got=%+v", got)
	}
	if len(got.Annotations) != 1 || got.Annotations[0] != "@PostMapping(\"/approve\")" {
		t.Fatalf("annotations=%v", got.Annotations)
	}
}

func TestN2ClassSymbolInfo(t *testing.T) {
	n := Navigator{AstGrepPath: "ast-grep", Runner: routingRunner{}}
	got, err := n.GetSymbolInfo(context.Background(), "OrderController", "src/main/java")
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != "CLASS" || got.DeclaringType != "OrderController" {
		t.Fatalf("got=%+v", got)
	}
}

func TestN3EnumSymbolInfo(t *testing.T) {
	n := Navigator{AstGrepPath: "ast-grep", Runner: routingRunner{}}
	got, err := n.GetSymbolInfo(context.Background(), "OrderType", "src/main/java")
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != "ENUM" {
		t.Fatalf("got=%+v", got)
	}
}

func TestN4AnnotationSearch(t *testing.T) {
	n := Navigator{AstGrepPath: "ast-grep", Runner: routingRunner{}}
	got, err := n.FindByAnnotation(context.Background(), "PostMapping", "src/main/java")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Matches) != 1 || got.Matches[0].Symbol != "OrderController.approve" || got.Matches[0].Kind != "METHOD" {
		t.Fatalf("got=%+v", got)
	}
}

func TestN5Callers(t *testing.T) {
	n := Navigator{AstGrepPath: "ast-grep", Runner: routingRunner{}}
	got, err := n.FindCallers(context.Background(), "OrderService.approve", "src/main/java")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Callers) != 1 || got.Callers[0].CallerSymbol != "OrderFacade.submit" || got.Callers[0].Line != 7 {
		t.Fatalf("got=%+v", got)
	}
}

func TestN6AmbiguousMethodFailsClosed(t *testing.T) {
	n := Navigator{AstGrepPath: "ast-grep", Runner: ambiguousRunner{}}
	_, err := n.GetSymbolInfo(context.Background(), "OrderController.approve", "src/main/java")
	if !errors.Is(err, ErrAmbiguousSymbol) {
		t.Fatalf("err=%v", err)
	}
}

type ambiguousRunner struct{}

func (ambiguousRunner) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	p := ""
	for i, a := range args {
		if a == "--pattern" && i+1 < len(args) {
			p = args[i+1]
		}
	}
	if strings.Contains(p, "OrderController") {
		return []byte(`{"file":"A.java","range":{"start":{"line":0},"end":{"line":20}},"text":"class OrderController {}"}` + "\n"), nil
	}
	if strings.Contains(p, "approve($$$ARGS)") {
		return []byte(`{"file":"A.java","range":{"start":{"line":3},"end":{"line":5}},"text":"Result<Void> approve(A a) {}"}` + "\n" + `{"file":"A.java","range":{"start":{"line":7},"end":{"line":9}},"text":"Result<Void> approve(B b) {}"}` + "\n"), nil
	}
	return nil, nil
}

func TestN7ScopeEscapeRejectedForNewNavigation(t *testing.T) {
	n := Navigator{AstGrepPath: "ast-grep", Runner: routingRunner{}}
	if _, err := n.FindByAnnotation(context.Background(), "RestController", "../outside"); !errors.Is(err, ErrInvalidScope) {
		t.Fatalf("err=%v", err)
	}
}

func TestSymbolInfoSupportsField(t *testing.T) {
	n := Navigator{AstGrepPath: "ast-grep", Runner: fieldRunner{}}
	got, err := n.GetSymbolInfo(context.Background(), "OrderController.DEFAULT_LIMIT", "src/main/java")
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != "FIELD" || got.ReturnType != "int" || got.Signature != "DEFAULT_LIMIT" {
		t.Fatalf("got=%+v", got)
	}
}

type fieldRunner struct{}

func (fieldRunner) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	p := ""
	for i, a := range args {
		if a == "--pattern" && i+1 < len(args) {
			p = args[i+1]
		}
	}
	if strings.Contains(p, "OrderController") {
		return []byte(`{"file":"A.java","range":{"start":{"line":0},"end":{"line":20}},"text":"class OrderController {}"}` + "\n"), nil
	}
	if strings.Contains(p, "DEFAULT_LIMIT") {
		return []byte(`{"file":"A.java","range":{"start":{"line":2},"end":{"line":2}},"text":"private static final int DEFAULT_LIMIT = 100;"}` + "\n"), nil
	}
	return nil, nil
}
