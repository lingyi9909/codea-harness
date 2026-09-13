package reviewcontext

import (
	"context"
	"errors"
	"strings"
	"testing"

	"codea-harness-tools/internal/nav"
)

type fakeResolver170 struct {
	methods map[string]nav.MethodFacts170
	callers map[string][]nav.Relation170
	dubbo   map[string][]nav.Relation170
	methodCalls []string
}

func (f *fakeResolver170) Method(_ context.Context, ref nav.ReviewRef170) (nav.MethodFacts170, error) {
	key, err := nav.ReviewRefKey170(ref)
	if err != nil { return nav.MethodFacts170{}, err }
	f.methodCalls = append(f.methodCalls, key)
	if v, ok := f.methods[key]; ok { return v, nil }
	return nav.MethodFacts170{Method: ref, Calls: []nav.Relation170{}, Issues: []nav.Issue170{}}, nil
}
func (f *fakeResolver170) Callers(_ context.Context, ref nav.ReviewRef170) ([]nav.Relation170, error) {
	key, err := nav.ReviewRefKey170(ref)
	if err != nil { return nil, err }
	return append([]nav.Relation170(nil), f.callers[key]...), nil
}
func (f *fakeResolver170) Mapper(context.Context, nav.SourceRange170) ([]nav.Relation170, []nav.Issue170, error) {
	return []nav.Relation170{}, []nav.Issue170{}, nil
}
func (f *fakeResolver170) Dubbo(_ context.Context, ref nav.ReviewRef170) ([]nav.Relation170, []nav.Issue170, error) {
	key, err := nav.ReviewRefKey170(ref)
	if err != nil { return nil, nil, err }
	return append([]nav.Relation170(nil), f.dubbo[key]...), []nav.Issue170{}, nil
}

func ref170(path, owner, name string) nav.ReviewRef170 {
	return nav.ReviewRef170{Workspace:"current", Path:path, Side:"CURRENT", Kind:"METHOD", OwnerFQCN:owner, Name:name, ParameterTypes:[]string{}}
}
func baseRef170(path, owner, name string) nav.ReviewRef170 {
	r := ref170(path, owner, name); r.Side = "BASE"; return r
}
func exact170(id, kind string, from, to nav.ReviewRef170) nav.Relation170 {
	return nav.Relation170{ID:id, Kind:kind, Resolution:"EXACT", From:from, Targets:[]nav.ReviewRef170{to}, Evidence:[]nav.SourceRange170{{Ref:from, StartLine:3, EndLine:3, StartColumn:5, EndColumn:15}}, Reason:"", Assumptions:[]string{}}
}
func fakeFor170(t *testing.T, seed nav.ReviewRef170, calls []nav.Relation170, dubbo []nav.Relation170) *fakeResolver170 {
	t.Helper(); key, err := nav.ReviewRefKey170(seed); if err != nil { t.Fatal(err) }
	return &fakeResolver170{methods:map[string]nav.MethodFacts170{key:{Method:seed, Calls:calls, Issues:[]nav.Issue170{}}}, callers:map[string][]nav.Relation170{}, dubbo:map[string][]nav.Relation170{key:dubbo}}
}

func Test170ParallelCallsStaySeparate(t *testing.T) {
	seed := ref170("src/main/java/demo/A.java", "demo.A", "submit")
	risk := ref170("src/main/java/demo/Risk.java", "demo.Risk", "check")
	repo := ref170("src/main/java/demo/Repo.java", "demo.Repo", "save")
	resolver := fakeFor170(t, seed, []nav.Relation170{exact170("r-risk","JAVA_CALL",seed,risk), exact170("r-repo","JAVA_CALL",seed,repo)}, nil)
	got, err := Build170(context.Background(), BuildInput170{RunID:"review-case", Phase:"DISCOVERY", Seeds:[]nav.ReviewRef170{seed}, Budget:DefaultBudget170()}, resolver)
	if err != nil { t.Fatal(err) }
	if len(got.Relations) != 2 { t.Fatalf("relations=%+v", got.Relations) }
	for _, rel := range got.Relations {
		if rel.From.OwnerFQCN != "demo.A" { t.Fatalf("parallel call was serialized through sibling target: %+v", rel) }
	}
}

func Test170BudgetReportsBlocked(t *testing.T) {
	seed := ref170("src/main/java/demo/A.java", "demo.A", "submit")
	local := ref170("src/main/java/demo/B.java", "demo.B", "save")
	remote := ref170("src/main/java/demo/C.java", "demo.C", "check")
	resolver := fakeFor170(t, seed, []nav.Relation170{exact170("r-local","JAVA_CALL",seed,local)}, []nav.Relation170{exact170("r-remote","DUBBO_CONTRACT",seed,remote)})
	budget := DefaultBudget170(); budget.MaxFiles = 1
	got, err := Build170(context.Background(), BuildInput170{RunID:"review-case", Phase:"RULES", Seeds:[]nav.ReviewRef170{seed}, Needs:[]Need170{
		{ReviewUnitID:"u1", RuleID:"LOCAL", Seeds:[]nav.ReviewRef170{seed}, RequiredKinds:[]string{"JAVA_CALL"}},
		{ReviewUnitID:"u1", RuleID:"REMOTE", Seeds:[]nav.ReviewRef170{seed}, RequiredKinds:[]string{"DUBBO_CONTRACT"}},
	}, Budget:budget}, resolver)
	if err != nil { t.Fatal(err) }
	status := map[string]string{}; reasons := map[string][]string{}
	for _, c := range got.Checks { status[c.RuleID]=c.Status; reasons[c.RuleID]=c.Reasons }
	if status["LOCAL"] != "READY" { t.Fatalf("local=%s checks=%+v", status["LOCAL"], got.Checks) }
	if status["REMOTE"] != "BLOCKED" { t.Fatalf("remote=%s checks=%+v", status["REMOTE"], got.Checks) }
	if !strings.Contains(strings.Join(reasons["REMOTE"], " "), "CONTEXT_BUDGET_EXCEEDED") { t.Fatalf("remote reasons=%v", reasons["REMOTE"]) }
}

func Test170UntrackedSeedIncluded(t *testing.T) {
	seed := ref170("src/main/java/demo/NewService.java", "demo.NewService", "submit")
	target := ref170("src/main/java/demo/Repo.java", "demo.Repo", "save")
	resolver := fakeFor170(t, seed, []nav.Relation170{exact170("r-new","JAVA_CALL",seed,target)}, nil)
	got, err := Build170(context.Background(), BuildInput170{RunID:"review-case", Phase:"DISCOVERY", Seeds:[]nav.ReviewRef170{seed}, Budget:DefaultBudget170()}, resolver)
	if err != nil { t.Fatal(err) }
	if len(got.Relations) == 0 || len(resolver.methodCalls) == 0 { t.Fatalf("untracked seed was filtered: %+v", got) }
}

func Test170DeletedMethodBeforeContext(t *testing.T) {
	seed := baseRef170("src/main/java/demo/Removed.java", "demo.Removed", "submit")
	resolver := &fakeResolver170{methods:map[string]nav.MethodFacts170{}, callers:map[string][]nav.Relation170{}, dubbo:map[string][]nav.Relation170{}}
	got, err := Build170(context.Background(), BuildInput170{RunID:"review-case", Phase:"DISCOVERY", Seeds:[]nav.ReviewRef170{seed}, Budget:DefaultBudget170()}, resolver)
	if err != nil { t.Fatal(err) }
	if len(got.Relations) != 0 || len(resolver.methodCalls) != 0 { t.Fatalf("BASE-only deleted method became CURRENT context: %+v calls=%v", got.Relations, resolver.methodCalls) }
}

func Test170ContextCannotExpandScope(t *testing.T) {
	seed := ref170("src/main/java/demo/A.java", "demo.A", "submit")
	dep := ref170("src/main/java/shared/Shared.java", "shared.Shared", "check"); dep.Workspace = "shared-verified"
	resolver := fakeFor170(t, seed, []nav.Relation170{exact170("r-dep","JAVA_CALL",seed,dep)}, nil)
	input := BuildInput170{RunID:"review-case", Phase:"DISCOVERY", Seeds:[]nav.ReviewRef170{seed}, Budget:DefaultBudget170()}
	got, err := Build170(context.Background(), input, resolver)
	if err != nil { t.Fatal(err) }
	if len(got.Relations) != 1 || got.Relations[0].Targets[0].Workspace != "shared-verified" { t.Fatalf("dependency context missing: %+v", got.Relations) }
	if len(input.Seeds) != 1 || input.Seeds[0].Workspace != "current" { t.Fatalf("Build170 mutated authoritative selection input: %+v", input.Seeds) }
}

func Test170ForgedRelationRejected(t *testing.T) {
	seed := ref170("src/main/java/demo/A.java", "demo.A", "submit")
	target := ref170("src/main/java/demo/B.java", "demo.B", "save")
	resolver := fakeFor170(t, seed, []nav.Relation170{exact170("r1","JAVA_CALL",seed,target)}, nil)
	input := BuildInput170{RunID:"review-case", Phase:"DISCOVERY", Seeds:[]nav.ReviewRef170{seed}, Budget:DefaultBudget170()}
	ctx, err := Build170(context.Background(), input, resolver); if err != nil { t.Fatal(err) }
	ctx.Relations[0].Targets[0].OwnerFQCN = "evil.Forged"
	if err := VerifyRelations170(context.Background(), ctx, input, resolver); err == nil || !strings.Contains(err.Error(), "CONTEXT_RELATION_NOT_VERIFIED") {
		t.Fatalf("forged relation accepted: %v", err)
	}
}

func Test170BuildRejectsResolverFailure(t *testing.T) {
	seed := ref170("src/main/java/demo/A.java", "demo.A", "submit")
	resolver := errorResolver170{}
	if _, err := Build170(context.Background(), BuildInput170{RunID:"review-case", Phase:"DISCOVERY", Seeds:[]nav.ReviewRef170{seed}, Budget:DefaultBudget170()}, resolver); err == nil { t.Fatal("resolver failure accepted") }
}

type errorResolver170 struct{}
func (errorResolver170) Method(context.Context, nav.ReviewRef170) (nav.MethodFacts170,error){ return nav.MethodFacts170{}, errors.New("boom") }
func (errorResolver170) Callers(context.Context, nav.ReviewRef170)([]nav.Relation170,error){ return nil, errors.New("boom") }
func (errorResolver170) Mapper(context.Context, nav.SourceRange170)([]nav.Relation170,[]nav.Issue170,error){ return nil,nil,errors.New("boom") }
func (errorResolver170) Dubbo(context.Context, nav.ReviewRef170)([]nav.Relation170,[]nav.Issue170,error){ return nil,nil,errors.New("boom") }
