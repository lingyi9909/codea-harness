package reviewcontext

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"codea-harness-tools/internal/nav"
)

type boundedResolver170 struct {
	seed       nav.ReviewRef170
	facts      nav.MethodFacts170
	candidates int
	exhausted  bool
}

func (r *boundedResolver170) Method(context.Context, nav.ReviewRef170) (nav.MethodFacts170, error) { return r.facts, nil }
func (r *boundedResolver170) Callers(context.Context, nav.ReviewRef170) ([]nav.Relation170, error) { return []nav.Relation170{}, nil }
func (r *boundedResolver170) CallersBounded170(context.Context, nav.ReviewRef170, int) ([]nav.Relation170, int, bool, error) {
	return []nav.Relation170{}, r.candidates, r.exhausted, nil
}
func (r *boundedResolver170) Mapper(context.Context, nav.SourceRange170) ([]nav.Relation170, []nav.Issue170, error) { return []nav.Relation170{}, []nav.Issue170{}, nil }
func (r *boundedResolver170) Dubbo(context.Context, nav.ReviewRef170) ([]nav.Relation170, []nav.Issue170, error) { return []nav.Relation170{}, []nav.Issue170{}, nil }
func (r *boundedResolver170) Spring(context.Context, nav.ReviewRef170) ([]nav.Relation170, []nav.Issue170, error) { return []nav.Relation170{}, []nav.Issue170{}, nil }

func Test170SemanticCandidateBudgetBlocksInsteadOfOverScanning(t *testing.T) {
	seed := ref170("src/main/java/demo/A.java", "demo.A", "run")
	resolver := &boundedResolver170{seed:seed, facts:nav.MethodFacts170{Method:seed, Calls:[]nav.Relation170{}, Issues:[]nav.Issue170{}}, candidates:3, exhausted:true}
	budget := DefaultBudget170(); budget.MaxCandidates = 2
	got, err := Build170(context.Background(), BuildInput170{
		RunID:"r", Phase:"RULES", Seeds:[]nav.ReviewRef170{seed},
		Needs:[]Need170{{ReviewUnitID:"u",RuleID:"SPRING-TX-001",Seeds:[]nav.ReviewRef170{seed},RequiredKinds:[]string{"JAVA_CALL"}}}, Budget:budget,
	}, resolver)
	if err != nil { t.Fatal(err) }
	if got.Usage.Candidates > budget.MaxCandidates { t.Fatalf("candidate budget exceeded in usage: %+v", got.Usage) }
	if len(got.Checks) != 1 || got.Checks[0].Status != "BLOCKED" || !strings.Contains(strings.Join(got.Checks[0].Reasons," "), "CONTEXT_BUDGET_EXCEEDED") {
		t.Fatalf("budget exhaustion did not fail closed: %+v issues=%+v", got.Checks, got.Issues)
	}
}

type timeoutResolver170 struct{ seed nav.ReviewRef170 }
func (r timeoutResolver170) Method(ctx context.Context, ref nav.ReviewRef170) (nav.MethodFacts170, error) {
	<-ctx.Done(); return nav.MethodFacts170{}, ctx.Err()
}
func (r timeoutResolver170) Callers(context.Context, nav.ReviewRef170)([]nav.Relation170,error){ return nil, errors.New("must not reach callers") }
func (r timeoutResolver170) Mapper(context.Context, nav.SourceRange170)([]nav.Relation170,[]nav.Issue170,error){ return []nav.Relation170{},[]nav.Issue170{},nil }
func (r timeoutResolver170) Dubbo(context.Context, nav.ReviewRef170)([]nav.Relation170,[]nav.Issue170,error){ return []nav.Relation170{},[]nav.Issue170{},nil }
func (r timeoutResolver170) Spring(context.Context, nav.ReviewRef170)([]nav.Relation170,[]nav.Issue170,error){ return []nav.Relation170{},[]nav.Issue170{},nil }

func Test170ResolverDeadlineBecomesContextLimitation(t *testing.T) {
	seed := ref170("src/main/java/demo/A.java", "demo.A", "run")
	budget := DefaultBudget170(); budget.MaxMillis = 5
	started := time.Now()
	got, err := Build170(context.Background(), BuildInput170{
		RunID:"r", Phase:"RULES", Seeds:[]nav.ReviewRef170{seed},
		Needs:[]Need170{{ReviewUnitID:"u",RuleID:"SPRING-TX-001",Seeds:[]nav.ReviewRef170{seed},RequiredKinds:[]string{"JAVA_CALL"}}}, Budget:budget,
	}, timeoutResolver170{seed:seed})
	if err != nil { t.Fatalf("deadline became product error: %v", err) }
	if time.Since(started) > time.Second { t.Fatal("deadline was not bounded") }
	if len(got.Checks) != 1 || got.Checks[0].Status != "BLOCKED" { t.Fatalf("checks=%+v", got.Checks) }
	if !strings.Contains(strings.Join(got.Checks[0].Reasons," "), "CONTEXT_BUDGET_EXCEEDED") { t.Fatalf("reasons=%v", got.Checks[0].Reasons) }
	found := false
	for _, issue := range got.Issues { if issue.Code == "CONTEXT_BUDGET_EXCEEDED" { found=true } }
	if !found { t.Fatalf("timeout limitation issue missing: %+v", got.Issues) }
}

type springResolver170 struct {
	seed nav.ReviewRef170
	call nav.Relation170
	binding nav.Relation170
}
func (r springResolver170) Method(context.Context, nav.ReviewRef170)(nav.MethodFacts170,error){ return nav.MethodFacts170{Method:r.seed,Calls:[]nav.Relation170{r.call},Issues:[]nav.Issue170{}},nil }
func (r springResolver170) Callers(context.Context, nav.ReviewRef170)([]nav.Relation170,error){ return []nav.Relation170{},nil }
func (r springResolver170) Mapper(context.Context, nav.SourceRange170)([]nav.Relation170,[]nav.Issue170,error){ return []nav.Relation170{},[]nav.Issue170{},nil }
func (r springResolver170) Dubbo(context.Context, nav.ReviewRef170)([]nav.Relation170,[]nav.Issue170,error){ return []nav.Relation170{},[]nav.Issue170{},nil }
func (r springResolver170) Spring(context.Context, nav.ReviewRef170)([]nav.Relation170,[]nav.Issue170,error){ return []nav.Relation170{r.binding},[]nav.Issue170{},nil }

func Test170SpringRuleRequiresCallAndBinding(t *testing.T) {
	seed := ref170("src/main/java/demo/Service.java", "demo.Service", "tx")
	repo := ref170("src/main/java/demo/Repo.java", "demo.Repo", "save")
	call := exact170("call","JAVA_CALL",seed,repo)
	owner := nav.ReviewRef170{Workspace:"current",Path:"src/main/java/demo/Controller.java",Side:"CURRENT",Kind:"TYPE",OwnerFQCN:"demo.Controller",Name:"Controller",ParameterTypes:[]string{}}
	targetType := nav.ReviewRef170{Workspace:"current",Path:seed.Path,Side:"CURRENT",Kind:"TYPE",OwnerFQCN:seed.OwnerFQCN,Name:"Service",ParameterTypes:[]string{}}
	binding := nav.Relation170{ID:"binding",Kind:"SPRING_BINDING",Resolution:"EXACT",From:owner,Targets:[]nav.ReviewRef170{targetType},Evidence:[]nav.SourceRange170{{Ref:owner,StartLine:1,EndLine:1,StartColumn:1,EndColumn:2}},Assumptions:[]string{}}
	need := Need170{ReviewUnitID:"u",RuleID:"SPRING-TX-001",Seeds:[]nav.ReviewRef170{seed},RequiredKinds:[]string{"JAVA_CALL","SPRING_BINDING"}}
	got, err := Build170(context.Background(), BuildInput170{RunID:"r",Phase:"RULES",Seeds:[]nav.ReviewRef170{seed},Needs:[]Need170{need},Budget:DefaultBudget170()}, springResolver170{seed:seed,call:call,binding:binding})
	if err != nil { t.Fatal(err) }
	if len(got.Checks)!=1 || got.Checks[0].Status!="READY" { t.Fatalf("spring check not ready with verified call+binding: %+v", got.Checks) }

	missing, err := Build170(context.Background(), BuildInput170{RunID:"r2",Phase:"RULES",Seeds:[]nav.ReviewRef170{seed},Needs:[]Need170{need},Budget:DefaultBudget170()}, springResolver170{seed:seed,call:call})
	if err != nil { t.Fatal(err) }
	if len(missing.Checks)!=1 || missing.Checks[0].Status!="BLOCKED" || !strings.Contains(strings.Join(missing.Checks[0].Reasons," "), "SPRING_BINDING") {
		t.Fatalf("missing proxy/binding context was not blocked: %+v", missing.Checks)
	}
}
