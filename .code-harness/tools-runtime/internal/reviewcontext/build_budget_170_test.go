package reviewcontext

import (
	"context"
	"strings"
	"testing"

	"codea-harness-tools/internal/nav"
)

type sizedResolver170 struct {
	*fakeResolver170
	sizes map[string]int
}

func (s *sizedResolver170) SourceBytes(_ context.Context, ref nav.ReviewRef170) (int, error) {
	key, err := nav.ReviewRefKey170(ref)
	if err != nil { return 0, err }
	return s.sizes[key], nil
}

func Test170SourceByteBudgetBlocksAdditionalContext(t *testing.T) {
	seed := ref170("src/main/java/demo/A.java", "demo.A", "submit")
	target := ref170("src/main/java/demo/Large.java", "demo.Large", "save")
	base := fakeFor170(t, seed, []nav.Relation170{exact170("r-large", "JAVA_CALL", seed, target)}, nil)
	targetKey, err := nav.ReviewRefKey170(target)
	if err != nil { t.Fatal(err) }
	resolver := &sizedResolver170{fakeResolver170: base, sizes: map[string]int{targetKey: 1024}}
	budget := DefaultBudget170(); budget.MaxSourceBytes = 32
	got, err := Build170(context.Background(), BuildInput170{RunID:"review-case", Phase:"RULES", Seeds:[]nav.ReviewRef170{seed}, Needs:[]Need170{{ReviewUnitID:"u1", RuleID:"CALL", Seeds:[]nav.ReviewRef170{seed}, RequiredKinds:[]string{"JAVA_CALL"}}}, Budget:budget}, resolver)
	if err != nil { t.Fatal(err) }
	if len(got.Relations) != 0 { t.Fatalf("oversize relation escaped budget: %+v", got.Relations) }
	if got.Usage.SourceBytes != 0 { t.Fatalf("rejected source counted as consumed: %+v", got.Usage) }
	if len(got.Checks) != 1 || got.Checks[0].Status != "BLOCKED" || !strings.Contains(strings.Join(got.Checks[0].Reasons, " "), "CONTEXT_BUDGET_EXCEEDED") {
		t.Fatalf("checks=%+v", got.Checks)
	}
}

func Test170BudgetRejectedEdgeDoesNotExpandQueue(t *testing.T) {
	seed := ref170("src/main/java/demo/A.java", "demo.A", "submit")
	target := ref170("src/main/java/demo/Large.java", "demo.Large", "save")
	seedKey, _ := nav.ReviewRefKey170(seed)
	targetKey, _ := nav.ReviewRefKey170(target)
	base := &fakeResolver170{
		methods: map[string]nav.MethodFacts170{
			seedKey: {Method:seed, Calls:[]nav.Relation170{exact170("r-large", "JAVA_CALL", seed, target)}, Issues:[]nav.Issue170{}},
			targetKey: {Method:target, Calls:[]nav.Relation170{}, Issues:[]nav.Issue170{}},
		},
		callers: map[string][]nav.Relation170{}, dubbo: map[string][]nav.Relation170{},
	}
	resolver := &sizedResolver170{fakeResolver170: base, sizes: map[string]int{targetKey: 1024}}
	budget := DefaultBudget170(); budget.MaxSourceBytes = 32
	if _, err := Build170(context.Background(), BuildInput170{RunID:"review-case", Phase:"DISCOVERY", Seeds:[]nav.ReviewRef170{seed}, Budget:budget}, resolver); err != nil { t.Fatal(err) }
	for _, call := range resolver.methodCalls {
		if call == targetKey { t.Fatalf("budget-rejected target was still expanded: calls=%v", resolver.methodCalls) }
	}
}

func Test170RecursionBoundaryIsExplicit(t *testing.T) {
	seed := ref170("src/main/java/demo/A.java", "demo.A", "a")
	b := ref170("src/main/java/demo/B.java", "demo.B", "b")
	c := ref170("src/main/java/demo/C.java", "demo.C", "c")
	seedKey, _ := nav.ReviewRefKey170(seed)
	bKey, _ := nav.ReviewRefKey170(b)
	resolver := &fakeResolver170{
		methods: map[string]nav.MethodFacts170{
			seedKey: {Method:seed, Calls:[]nav.Relation170{exact170("ab", "JAVA_CALL", seed, b)}, Issues:[]nav.Issue170{}},
			bKey: {Method:b, Calls:[]nav.Relation170{exact170("bc", "JAVA_CALL", b, c)}, Issues:[]nav.Issue170{}},
		},
		callers: map[string][]nav.Relation170{}, dubbo: map[string][]nav.Relation170{},
	}
	budget := DefaultBudget170(); budget.MaxDownstreamDepth = 1
	got, err := Build170(context.Background(), BuildInput170{RunID:"review-case", Phase:"DISCOVERY", Seeds:[]nav.ReviewRef170{seed}, Budget:budget}, resolver)
	if err != nil { t.Fatal(err) }
	found := false
	for _, issue := range got.Issues { if issue.Code == "RECURSION_BOUNDARY" { found = true } }
	if !found { t.Fatalf("missing RECURSION_BOUNDARY: issues=%+v relations=%+v", got.Issues, got.Relations) }
}
