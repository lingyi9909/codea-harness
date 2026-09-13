package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/changeset"
	"codea-harness-tools/internal/nav"
	"codea-harness-tools/internal/reviewcontext"
	"codea-harness-tools/internal/reviewprogress"
	"codea-harness-tools/internal/reviewrules"
	"codea-harness-tools/internal/reviewunit"
)

func Test170NewRunIdentityDoesNotDependOnDiscoveryArtifact(t *testing.T) {
	state := reviewprogress.State{ProtocolVersion: reviewprogress.Protocol170, Status: reviewprogress.StatusRunning, CurrentStage: reviewprogress.StageReviewPlanning}
	if !isReviewProtocol170(state) {
		t.Fatal("runtime-owned 1.7 identity was not recognized")
	}
	legacy := reviewprogress.State{Status: reviewprogress.StatusRunning, CurrentStage: reviewprogress.StageReviewPlanning}
	if isReviewProtocol170(legacy) {
		t.Fatal("legacy run was upgraded to 1.7 without runtime authority")
	}
}

func Test170DispatchRequiresDiscoveryForNewRun(t *testing.T) {
	root := t.TempDir()
	old, err := os.Getwd()
	if err != nil { t.Fatal(err) }
	if err := os.Chdir(root); err != nil { t.Fatal(err) }
	defer os.Chdir(old)

	runID := "review-new-170"
	if err := os.MkdirAll(filepath.Join(".code-harness", "runs", runID), 0o755); err != nil { t.Fatal(err) }
	if _, err := reviewprogress.Begin170(".", runID); err != nil { t.Fatal(err) }
	for _, stage := range []string{reviewprogress.StageSnapshot, reviewprogress.StageChangeAnalysis, reviewprogress.StageCertification} {
		if _, err := reviewprogress.Advance(".", runID, stage); err != nil { t.Fatal(err) }
	}
	err = runReview170([]string{"dispatch", "--run-id", runID})
	if err == nil || !strings.Contains(err.Error(), "REVIEW_CONTEXT_DISCOVERY_REQUIRED") {
		t.Fatalf("new 1.7 dispatch did not fail closed on missing DISCOVERY: %v", err)
	}
}

func Test170LegacyDispatchRoutingRemainsLegacy(t *testing.T) {
	state := reviewprogress.State{Status: reviewprogress.StatusRunning, CurrentStage: reviewprogress.StageReviewPlanning}
	if got := reviewDispatchProtocol170(state); got != "LEGACY" {
		t.Fatalf("legacy routing=%q", got)
	}
}

func Test170NeedsMapFormalEvidenceByRole(t *testing.T) {
	mapper := reviewrules.Dispatch{ReviewUnitID:"u-map", RuleID:"MYBATIS-SQL-001", RequiredEvidence:[]string{"CHANGED_RANGE", "RESOURCE_RELATION"}, DispatchReason:[]string{"CHANGED_ROLE:MapperXml"}}
	spring := reviewrules.Dispatch{ReviewUnitID:"u-tx", RuleID:"SPRING-TX-001", RequiredEvidence:[]string{"CHAIN", "SYMBOL"}, DispatchReason:[]string{"CHANGED_ROLE:Service"}}
	if got := requiredKindsForDispatch170(mapper); !reflect.DeepEqual(got, []string{"MYBATIS_STATEMENT"}) {
		t.Fatalf("mapper required kinds=%v", got)
	}
	if got := requiredKindsForDispatch170(spring); !reflect.DeepEqual(got, []string{"JAVA_CALL", "SPRING_BINDING"}) {
		t.Fatalf("spring required kinds=%v", got)
	}
}

func Test170NeedsDoNotPairChainAndFilesByIndex(t *testing.T) {
	units := reviewunit.Manifest{Units: []reviewunit.Unit{{
		ID: "u1",
		Chain: []string{"demo.Service.pay"},
		Files: []reviewunit.FileRef{
			{Path:"src/main/resources/demo/Mapper.xml", Role:"MapperXml", Changed:true, Workspace:"current"},
			{Path:"src/main/java/demo/Service.java", Role:"Service", Changed:true, Workspace:"current"},
		},
	}}}
	dispatch := reviewrules.Manifest{Dispatches: []reviewrules.Dispatch{{ReviewUnitID:"u1", RuleID:"SPRING-TX-001", RequiredEvidence:[]string{"CHAIN", "SYMBOL"}, DispatchReason:[]string{"CHANGED_ROLE:Service"}}}}
	a := analysisruntime.ChangeAnalysis{SymbolLocations: []analysisruntime.SymbolLocation{{Workspace:"current", Symbol:"demo.Service.pay", Path:"src/main/java/demo/Service.java", Role:"Service"}}}
	needs := needsFromDispatch170(units, dispatch, a)
	if len(needs) != 1 || len(needs[0].Seeds) != 1 {
		t.Fatalf("needs=%+v", needs)
	}
	if got := needs[0].Seeds[0].Path; got != "src/main/java/demo/Service.java" {
		t.Fatalf("chain symbol incorrectly paired with Files index: %s", got)
	}
}

func Test170BaseAdapterReturnsDeletedMethodAndMapperEvidence(t *testing.T) {
	if os.Getenv("CODEA_AST_GREP") == "" { t.Fatal("CODEA_AST_GREP must be provisioned") }
	root := t.TempDir()
	git := func(args ...string) {
		cmd := exec.Command("git", args...); cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil { t.Fatalf("git %v: %v: %s", args, err, out) }
	}
	write := func(p, body string) {
		dst := filepath.Join(root, filepath.FromSlash(p)); if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil { t.Fatal(err) }
		if err := os.WriteFile(dst, []byte(body), 0o644); err != nil { t.Fatal(err) }
	}
	git("init"); git("config", "user.email", "t@example.invalid"); git("config", "user.name", "t")
	javaPath := "src/main/java/demo/OrderMapper.java"
	xmlPath := "src/main/resources/demo/OrderMapper.xml"
	write(javaPath, `package demo; public interface OrderMapper { void updateStatus(Long id, Long tenantId); default void removed(){ updateStatus(1L, 2L); } }`)
	write(xmlPath, `<mapper namespace="demo.OrderMapper"><update id="updateStatus">UPDATE orders SET status=1 WHERE id=#{id} AND tenant_id=#{tenantId}</update></mapper>`)
	git("add", "."); git("commit", "-m", "base")
	baseOut, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output(); if err != nil { t.Fatal(err) }
	base := strings.TrimSpace(string(baseOut))
	write(javaPath, `package demo; public interface OrderMapper { void updateStatus(Long id, Long tenantId); }`)
	write(xmlPath, `<mapper namespace="demo.OrderMapper"><update id="updateStatus">UPDATE orders SET status=1 WHERE id=#{id}</update></mapper>`)
	git("add", "."); git("commit", "-m", "current")
	snap, err := changeset.Compute(root, base, true); if err != nil { t.Fatal(err) }
	baseCtx, cleanup, err := prepareBaseSource170(context.Background(), root, snap, []string{javaPath, xmlPath}, os.Getenv("CODEA_AST_GREP"))
	if err != nil { t.Fatal(err) }
	defer cleanup()
	refs, ranges, err := baseCtx.Navigator.DiscoverMethodsForSide170(context.Background(), []string{javaPath}, "current", "BASE")
	if err != nil { t.Fatal(err) }
	var removedRange nav.SourceRange170
	foundRemoved := false
	for i, ref := range refs {
		if ref.Name == "removed" { foundRemoved = true; removedRange = ranges[i] }
		if ref.Side != "BASE" { t.Fatalf("base ref escaped as %s", ref.Side) }
	}
	if !foundRemoved { t.Fatal("deleted method missing from BASE adapter") }
	_ = removedRange
	var mapperRange nav.SourceRange170
	for i, ref := range refs { if ref.Name == "updateStatus" { mapperRange = ranges[i] } }
	if mapperRange.Ref.Name == "" { t.Fatal("BASE mapper method missing") }
	rels, _, err := reviewcontext.ResolveMapper170(context.Background(), baseCtx.Root, mapperRange)
	if err != nil { t.Fatal(err) }
	foundTenantBase := false
	for _, rel := range rels {
		if rel.Kind != "MYBATIS_STATEMENT" || rel.Resolution != "EXACT" { continue }
		for _, ev := range rel.Evidence {
			if ev.Ref.Side == "BASE" && strings.HasSuffix(ev.Ref.Path, "ordermapper.xml") { foundTenantBase = true }
		}
	}
	if !foundTenantBase { t.Fatalf("BASE statement evidence missing: %+v", rels) }
}

func Test170UseTimeVerificationRejectsTamperedRuleContext(t *testing.T) {
	seed := nav.ReviewRef170{Workspace:"current",Path:"src/main/java/demo/A.java",Side:"CURRENT",Kind:"METHOD",OwnerFQCN:"demo.A",Name:"run",ParameterTypes:[]string{}}
	target := seed; target.Path="src/main/java/demo/B.java"; target.OwnerFQCN="demo.B"; target.Name="save"
	rel := nav.Relation170{ID:"r1",Kind:"JAVA_CALL",Resolution:"EXACT",From:seed,Targets:[]nav.ReviewRef170{target},Evidence:[]nav.SourceRange170{{Ref:seed,StartLine:1,EndLine:1,StartColumn:1,EndColumn:2}},Assumptions:[]string{}}
	resolver := &useTimeResolver170{facts: nav.MethodFacts170{Method:seed, Calls:[]nav.Relation170{rel}, Issues:[]nav.Issue170{}}}
	input := reviewcontext.BuildInput170{RunID:"r",Phase:"RULES",Seeds:[]nav.ReviewRef170{seed},Needs:[]reviewcontext.Need170{{ReviewUnitID:"u",RuleID:"SPRING-TX-001",Seeds:[]nav.ReviewRef170{seed},RequiredKinds:[]string{"JAVA_CALL"}}},Budget:reviewcontext.DefaultBudget170()}
	claimed, err := reviewcontext.Build170(context.Background(), input, resolver); if err != nil { t.Fatal(err) }
	claimed.Relations[0].Targets[0].OwnerFQCN = "evil.Tampered"
	if err := verifyReviewContextUse170(context.Background(), claimed, input, resolver); err == nil || !strings.Contains(err.Error(), "CONTEXT_RELATION_NOT_VERIFIED") {
		t.Fatalf("tampered relation passed use-time verification: %v", err)
	}
}

type useTimeResolver170 struct{ facts nav.MethodFacts170 }
func (r *useTimeResolver170) Method(context.Context, nav.ReviewRef170)(nav.MethodFacts170,error){ return r.facts,nil }
func (r *useTimeResolver170) Callers(context.Context, nav.ReviewRef170)([]nav.Relation170,error){ return []nav.Relation170{},nil }
func (r *useTimeResolver170) Mapper(context.Context, nav.SourceRange170)([]nav.Relation170,[]nav.Issue170,error){ return []nav.Relation170{},[]nav.Issue170{},nil }
func (r *useTimeResolver170) Dubbo(context.Context, nav.ReviewRef170)([]nav.Relation170,[]nav.Issue170,error){ return []nav.Relation170{},[]nav.Issue170{},nil }
func (r *useTimeResolver170) Spring(context.Context, nav.ReviewRef170)([]nav.Relation170,[]nav.Issue170,error){ return []nav.Relation170{},[]nav.Issue170{},nil }

func Test170SortHelperSanity(t *testing.T) {
	// Keeps imports deterministic while the acceptance regressions above exercise
	// the real rule and source adapters.
	v := []string{"b", "a"}; sort.Strings(v); if strings.Join(v, "") != "ab" { t.Fatal(v) }
}
