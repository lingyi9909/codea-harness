package nav

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"
)

func reviewMethod170(path, owner, name string, params ...string) ReviewRef170 {
	return ReviewRef170{
		Workspace:      "current",
		Path:           path,
		Side:           "CURRENT",
		Kind:           "METHOD",
		OwnerFQCN:      owner,
		Name:           name,
		ParameterTypes: append([]string{}, params...),
	}
}

func Test170SuperCallResolvesParentAndSameFileOwnership(t *testing.T) {
	n := newReviewFixture170(t, map[string]string{
		"src/main/java/demo/Child.java": `package demo;
class Parent { void pay() {} }
class Child extends Parent {
    @Override void pay() {}
    void submit() { super.pay(); }
}
`,
	})
	facts, err := n.InspectMethod170(context.Background(), reviewMethod170("src/main/java/demo/Child.java", "demo.Child", "submit"))
	if err != nil { t.Fatal(err) }
	if len(facts.Calls) != 1 { t.Fatalf("calls=%+v", facts.Calls) }
	call := facts.Calls[0]
	if call.Kind != "JAVA_CALL" || call.Resolution != "EXACT" || len(call.Targets) != 1 {
		t.Fatalf("call=%+v", call)
	}
	if got := call.Targets[0]; got.OwnerFQCN != "demo.Parent" || got.Name != "pay" {
		t.Fatalf("target=%+v", got)
	}
	if len(call.Evidence) != 1 || call.Evidence[0].StartColumn >= call.Evidence[0].EndColumn {
		t.Fatalf("evidence=%+v", call.Evidence)
	}
}

func Test170ShadowedParameterBeatsField(t *testing.T) {
	n := newReviewFixture170(t, map[string]string{
		"src/main/java/demo/Example.java": `package demo;
class First { void check() {} }
class Second { void check() {} }
class Example {
    First service;
    void run(Second service) { service.check(); }
}
`,
	})
	facts, err := n.InspectMethod170(context.Background(), reviewMethod170("src/main/java/demo/Example.java", "demo.Example", "run", "demo.Second"))
	if err != nil { t.Fatal(err) }
	if len(facts.Calls) != 1 || facts.Calls[0].Resolution != "EXACT" || len(facts.Calls[0].Targets) != 1 {
		t.Fatalf("calls=%+v", facts.Calls)
	}
	if got := facts.Calls[0].Targets[0].OwnerFQCN; got != "demo.Second" {
		t.Fatalf("owner=%q", got)
	}
}

func Test170OverloadNullRemainsAmbiguous(t *testing.T) {
	n := newReviewFixture170(t, map[string]string{
		"src/main/java/demo/Example.java": `package demo;
class Example {
    void send(String value) {}
    void send(Long value) {}
    void uncertain() { send(null); }
}
`,
	})
	facts, err := n.InspectMethod170(context.Background(), reviewMethod170("src/main/java/demo/Example.java", "demo.Example", "uncertain"))
	if err != nil { t.Fatal(err) }
	if len(facts.Calls) != 1 { t.Fatalf("calls=%+v", facts.Calls) }
	call := facts.Calls[0]
	if call.Resolution != "AMBIGUOUS" || !strings.Contains(call.Reason, "JAVA_OVERLOAD_UNRESOLVED") {
		t.Fatalf("call=%+v", call)
	}
	if len(call.Targets) != 2 { t.Fatalf("targets=%+v", call.Targets) }

	legacy, err := n.FindDirectMethodCalls(context.Background(), "Example.uncertain", "src/main/java")
	if err != nil { t.Fatal(err) }
	if len(legacy) != 1 || legacy[0].Resolved || legacy[0].TargetSymbol != "" {
		t.Fatalf("legacy=%+v", legacy)
	}
}

func Test170JavaExplicitImportDisambiguatesSameSimpleType(t *testing.T) {
	n := newReviewFixture170(t, map[string]string{
		"src/main/java/a/Service.java": `package a; public class Service { public void check() {} }`,
		"src/main/java/b/Service.java": `package b; public class Service { public void check() {} }`,
		"src/main/java/c/Example.java": `package c;
import b.Service;
class Example {
    Service service;
    void run() { service.check(); }
}
`,
	})
	facts, err := n.InspectMethod170(context.Background(), reviewMethod170("src/main/java/c/Example.java", "c.Example", "run"))
	if err != nil { t.Fatal(err) }
	if len(facts.Calls) != 1 || facts.Calls[0].Resolution != "EXACT" || len(facts.Calls[0].Targets) != 1 {
		t.Fatalf("calls=%+v", facts.Calls)
	}
	if got := facts.Calls[0].Targets[0].OwnerFQCN; got != "b.Service" {
		t.Fatalf("owner=%q", got)
	}
}

func Test170JavaCallsiteIdentityAndUnicodeColumns(t *testing.T) {
	line := `    void run() { String 标签 = ""; service.check(); service.check(); }`
	source := "package demo;\nclass Service { void check() {} }\nclass Example {\n    Service service;\n" + line + "\n}\n"
	n := newReviewFixture170(t, map[string]string{"src/main/java/demo/Example.java": source})
	facts, err := n.InspectMethod170(context.Background(), reviewMethod170("src/main/java/demo/Example.java", "demo.Example", "run"))
	if err != nil { t.Fatal(err) }
	if len(facts.Calls) != 2 { t.Fatalf("calls=%+v", facts.Calls) }
	if facts.Calls[0].ID == facts.Calls[1].ID { t.Fatalf("duplicate relation id=%q", facts.Calls[0].ID) }
	firstByte := strings.Index(line, "service.check()")
	secondByte := strings.Index(line[firstByte+1:], "service.check()") + firstByte + 1
	wantFirst := utf8.RuneCountInString(line[:firstByte]) + 1
	wantSecond := utf8.RuneCountInString(line[:secondByte]) + 1
	gotFirst := facts.Calls[0].Evidence[0].StartColumn
	gotSecond := facts.Calls[1].Evidence[0].StartColumn
	if gotFirst != wantFirst || gotSecond != wantSecond {
		t.Fatalf("unicode columns got=(%d,%d) want=(%d,%d)", gotFirst, gotSecond, wantFirst, wantSecond)
	}
}
