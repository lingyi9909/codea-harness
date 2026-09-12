package nav

import (
	"context"
	"strings"
	"testing"
)

// These regressions lock the four false-EXACT failure modes found during the
// T2 code-level acceptance review, including static nested member-type identity.
// They exercise the real AST fixture so compatibility changes cannot bypass
// conservative proof.
func Test170JavaUnsupportedLambdaArgumentNeverExact(t *testing.T) {
	n := newReviewFixture170(t, map[string]string{
		"src/main/java/demo/Example.java": `package demo;
interface Handler { String apply(String value); }
class Service { void send(Handler handler) {} }
class Example {
    Service service;
    void run() { service.send(value -> value); }
}
`,
	})
	facts, err := n.InspectMethod170(context.Background(), reviewMethod170("src/main/java/demo/Example.java", "demo.Example", "run"))
	if err != nil {
		t.Fatal(err)
	}
	if len(facts.Calls) != 1 {
		t.Fatalf("calls=%+v", facts.Calls)
	}
	if facts.Calls[0].Resolution == "EXACT" {
		t.Fatalf("unsupported lambda argument must not be EXACT: %+v", facts.Calls[0])
	}
	if facts.Calls[0].Resolution != "UNRESOLVED" || !strings.Contains(facts.Calls[0].Reason, "JAVA_OVERLOAD_UNRESOLVED") {
		t.Fatalf("unsupported argument inference must fail closed with overload reason: %+v", facts.Calls[0])
	}
}

func Test170JavaNestedTypeIdentityIncludesEnclosingType(t *testing.T) {
	n := newReviewFixture170(t, map[string]string{
		"src/main/java/demo/Outer.java": `package demo;
class OuterA {
    static class Service { void check() {} }
    Service service;
    void run() { service.check(); }
}
class OuterB {
    static class Service { void check() {} }
    Service service;
    void run() { service.check(); }
}
`,
	})
	for _, tc := range []struct {
		owner string
		want  string
	}{
		{owner: "demo.OuterA", want: "demo.OuterA.Service"},
		{owner: "demo.OuterB", want: "demo.OuterB.Service"},
	} {
		facts, err := n.InspectMethod170(context.Background(), reviewMethod170("src/main/java/demo/Outer.java", tc.owner, "run"))
		if err != nil {
			t.Fatal(err)
		}
		if len(facts.Calls) != 1 || facts.Calls[0].Resolution != "EXACT" || len(facts.Calls[0].Targets) != 1 {
			t.Fatalf("owner=%s calls=%+v", tc.owner, facts.Calls)
		}
		if got := facts.Calls[0].Targets[0].OwnerFQCN; got != tc.want {
			t.Fatalf("owner=%s target=%q want=%q", tc.owner, got, tc.want)
		}
	}
}

func Test170JavaLocalReceiverUsesLexicalScopeAtCallsite(t *testing.T) {
	n := newReviewFixture170(t, map[string]string{
		"src/main/java/demo/Example.java": `package demo;
class First { void check() {} }
class Second { void check() {} }
class Example {
    First service;
    void run() {
        service.check();
        {
            Second service;
            service.check();
        }
        service.check();
    }
}
`,
	})
	facts, err := n.InspectMethod170(context.Background(), reviewMethod170("src/main/java/demo/Example.java", "demo.Example", "run"))
	if err != nil {
		t.Fatal(err)
	}
	if len(facts.Calls) != 3 {
		t.Fatalf("calls=%+v", facts.Calls)
	}
	want := []string{"demo.First", "demo.Second", "demo.First"}
	for i, call := range facts.Calls {
		if call.Resolution != "EXACT" || len(call.Targets) != 1 {
			t.Fatalf("call[%d]=%+v", i, call)
		}
		if got := call.Targets[0].OwnerFQCN; got != want[i] {
			t.Fatalf("call[%d] owner=%q want=%q", i, got, want[i])
		}
	}
}

func Test170SpringBeanMethodRequiresRegisteredOwner(t *testing.T) {
	n := newReviewFixture170(t, map[string]string{
		"src/main/java/demo/PayService.java": `package demo; public interface PayService { void pay(); }`,
		"src/main/java/demo/A.java": `package demo; class A implements PayService { public void pay() {} }`,
		"src/main/java/demo/PlainConfig.java": `package demo; import org.springframework.context.annotation.Bean; class PlainConfig { @Bean PayService payService() { return new A(); } }`,
		"src/main/java/demo/Checkout.java": `package demo; class Checkout { PayService service; }`,
	})
	rel, err := n.ResolveInjection170(context.Background(), springInjection170("src/main/java/demo/Checkout.java", "demo.Checkout", "demo.PayService", "service", "FIELD"))
	if err != nil {
		t.Fatal(err)
	}
	if rel.Resolution != "UNRESOLVED" {
		t.Fatalf("@Bean on an unregistered owner must remain UNRESOLVED: %+v", rel)
	}
	for _, target := range rel.Targets {
		if target.OwnerFQCN == "demo.PlainConfig" {
			t.Fatalf("unregistered @Bean owner leaked into candidates: %+v", rel)
		}
	}
}
