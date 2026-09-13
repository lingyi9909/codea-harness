package nav

import (
	"context"
	"testing"
)

func Test170ExactCallersHonorsSemanticCandidateBudget(t *testing.T) {
	n := newReviewFixture170(t, map[string]string{
		"src/main/java/demo/Target.java": `package demo; class Target { void hit() {} }`,
		"src/main/java/demo/A.java": `package demo; class A { Target t; void a(){ t.hit(); } void a2(){} }`,
		"src/main/java/demo/B.java": `package demo; class B { Target t; void b(){ t.hit(); } void b2(){} }`,
		"src/main/java/demo/C.java": `package demo; class C { Target t; void c(){ t.hit(); } void c2(){} }`,
	})
	target := ReviewRef170{Workspace:"current",Path:"src/main/java/demo/Target.java",Side:"CURRENT",Kind:"METHOD",OwnerFQCN:"demo.Target",Name:"hit",ParameterTypes:[]string{}}
	_, candidates, exhausted, err := n.ExactCallersBounded170(context.Background(), target, 2)
	if err != nil { t.Fatal(err) }
	if candidates != 2 { t.Fatalf("semantic candidates=%d want=2", candidates) }
	if !exhausted { t.Fatal("candidate budget did not stop reverse discovery") }
}

func Test170DiscoverMethodsForSideKeepsBaseIdentity(t *testing.T) {
	n := newReviewFixture170(t, map[string]string{
		"src/main/java/demo/A.java": `package demo; class A { void removed(){} }`,
	})
	refs, ranges, err := n.DiscoverMethodsForSide170(context.Background(), []string{"src/main/java/demo/A.java"}, "current", "BASE")
	if err != nil { t.Fatal(err) }
	if len(refs) != 1 || len(ranges) != 1 { t.Fatalf("refs=%+v ranges=%+v", refs, ranges) }
	if refs[0].Side != "BASE" || ranges[0].Ref.Side != "BASE" { t.Fatalf("BASE identity lost: %+v %+v", refs[0], ranges[0]) }
}
