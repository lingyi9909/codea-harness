package nav

import (
	"context"
	"testing"
)

func Test170DiscoverChangedMethodsUsesExactRequestedPaths(t *testing.T) {
	n := newReviewFixture170(t, map[string]string{
		"src/main/java/demo/NewService.java": `package demo;
class NewService { void submit(String id) {} void helper() {} }
`,
		"src/main/java/demo/Other.java": `package demo; class Other { void hidden() {} }`,
	})
	methods, ranges, err := n.DiscoverMethods170(context.Background(), []string{"src/main/java/demo/NewService.java"})
	if err != nil { t.Fatal(err) }
	if len(methods) != 2 || len(ranges) != 2 { t.Fatalf("methods=%+v ranges=%+v", methods, ranges) }
	for _, method := range methods {
		if method.Path != "src/main/java/demo/NewService.java" || method.OwnerFQCN != "demo.NewService" { t.Fatalf("unexpected method=%+v", method) }
		if method.Name == "hidden" { t.Fatalf("unrequested path leaked into discovery: %+v", methods) }
	}
}

func Test170ExactCallersAreReprovedFromForwardEdges(t *testing.T) {
	n := newReviewFixture170(t, map[string]string{
		"src/main/java/demo/A.java": `package demo; class A { B b; void submit(){ b.save(); } }`,
		"src/main/java/demo/B.java": `package demo; class B { void save(){} }`,
	})
	target := reviewMethod170("src/main/java/demo/B.java", "demo.B", "save")
	callers, err := n.ExactCallers170(context.Background(), target)
	if err != nil { t.Fatal(err) }
	if len(callers) != 1 { t.Fatalf("callers=%+v", callers) }
	if callers[0].Resolution != "EXACT" || callers[0].From.OwnerFQCN != "demo.A" || callers[0].From.Name != "submit" { t.Fatalf("caller=%+v", callers[0]) }
	if len(callers[0].Targets) != 1 || callers[0].Targets[0].OwnerFQCN != "demo.B" { t.Fatalf("caller target=%+v", callers[0]) }
}
