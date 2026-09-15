package finding

import (
	"strings"
	"testing"

	"codea-harness-tools/internal/knowledge"
	"codea-harness-tools/internal/nav"
	"codea-harness-tools/internal/reviewcontext"
	"codea-harness-tools/internal/reviewrules"
)

func Test170ReviewChecksRequireEveryReadyDispatchedRule(t *testing.T) {
	ctx := verifyContext160(t)
	ctx.reviewContext170 = &reviewcontext.Context170{RunID: "run-task3", Phase: "RULES", Relations: []nav.Relation170{}, Checks: []reviewcontext.Check170{
		{ReviewUnitID: "RU-TASK3", RuleID: "SPRING-TX-001", Status: "READY", RelationIDs: []string{}, Reasons: []string{}},
		{ReviewUnitID: "RU-TASK3", RuleID: "SPRING-AUTH-001", Status: "BLOCKED", RelationIDs: []string{}, Reasons: []string{"RELATION_REQUIRED:JAVA_CALL"}},
	}}
	checks := []CheckResult170{{ReviewUnitID: "RU-TASK3", RuleID: "SPRING-TX-001", Status: "COMPLETED", Reason: "", SourceIDs: []string{}}}
	summary, err := ValidateCheckResults170(ctx, checks)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Status != "PARTIAL" || len(summary.BlockedChecks) != 1 || summary.BlockedChecks[0].RuleID != "SPRING-AUTH-001" {
		t.Fatalf("runtime BLOCKED check must survive reviewer completion: %#v", summary)
	}

	if _, err := ValidateCheckResults170(ctx, nil); err == nil || !strings.Contains(err.Error(), "REVIEW_CHECK_MISSING") {
		t.Fatalf("missing READY reviewer declaration must fail certification, got %v", err)
	}
	dupe := append(checks, checks[0])
	if _, err := ValidateCheckResults170(ctx, dupe); err == nil || !strings.Contains(err.Error(), "REVIEW_CHECK_DUPLICATE") {
		t.Fatalf("duplicate reviewer declaration must fail, got %v", err)
	}
	wrong := []CheckResult170{{ReviewUnitID: "RU-OTHER", RuleID: "SPRING-TX-001", Status: "COMPLETED", SourceIDs: []string{}}}
	if _, err := ValidateCheckResults170(ctx, wrong); err == nil || !strings.Contains(err.Error(), "REVIEW_CHECK_NOT_DISPATCHED") {
		t.Fatalf("cross-unit reviewer check must fail, got %v", err)
	}
}

func Test170BusinessReviewCheckRequiresActuallyReadSource(t *testing.T) {
	ctx := verifyContext160(t)
	manifest := knowledge.Manifest170{RunID: "run-task3", ProjectID: "order-service", Status: "READY", BindingSHA256: strings.Repeat("1", 64), Sources: []knowledge.SourceRecord170{{SourceID: "rules", Root: "PROJECT", Path: "docs/rules.md", Kind: "RULES", Required: true, SHA256: strings.Repeat("2", 64), Status: "READY", Rule: &knowledge.Rule170{RuleID: "R-1", ProjectID: "order-service", Status: "ACTIVE", Version: "1", Owner: "o", Source: "docs/rules.md", ApprovalRef: "a", AppliesTo: knowledge.AppliesTo170{Paths: []string{"./"}}}}}, Checks: []knowledge.BusinessCheck170{{ReviewUnitID: "RU-TASK3", RuleKey: "BUSINESS:rules:R-1", SourceID: "rules", RuleID: "R-1", Status: "READY", Reasons: []string{}}}, Issues: []string{}}
	digest, err := knowledge.Digest170(manifest)
	if err != nil {
		t.Fatal(err)
	}
	ctx.knowledge170 = &manifest
	ctx.dispatch.KnowledgeSHA256 = digest
	ctx.dispatch.Dispatches = append(ctx.dispatch.Dispatches, reviewrules.Dispatch{ReviewUnitID: "RU-TASK3", RuleID: "BUSINESS:rules:R-1", RuleVersion: 1, Kind: reviewrules.KindAgent, RequiredEvidence: []string{"BUSINESS_RULE", "CHANGED_RANGE"}, DispatchReason: []string{"BUSINESS_RULE:rules:R-1"}})
	ctx.reviewContext170 = &reviewcontext.Context170{RunID: "run-task3", Phase: "RULES", Relations: []nav.Relation170{}, Checks: []reviewcontext.Check170{}}

	checks := make([]CheckResult170, 0, len(ctx.dispatch.Dispatches))
	for _, d := range ctx.dispatch.Dispatches {
		if d.ReviewUnitID != "RU-TASK3" {
			continue
		}
		checks = append(checks, CheckResult170{ReviewUnitID: d.ReviewUnitID, RuleID: d.RuleID, Status: "COMPLETED", SourceIDs: []string{}})
	}
	if _, err := ValidateCheckResults170(ctx, checks); err == nil || !strings.Contains(err.Error(), "REVIEW_CHECK_SOURCE_NOT_READ") {
		t.Fatalf("business completion without source read must fail, got %v", err)
	}
	for i := range checks {
		if checks[i].RuleID == "BUSINESS:rules:R-1" {
			checks[i].SourceIDs = []string{"rules"}
		}
	}
	summary, err := ValidateCheckResults170(ctx, checks)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Status != "COMPLETE" {
		t.Fatalf("all dispatched READY checks completed should be complete, got %#v", summary)
	}

	for i := range checks {
		if checks[i].RuleID == "BUSINESS:rules:R-1" {
			checks[i].Status = "INCOMPLETE"
			checks[i].Reason = "rule conflict requires manual review"
		}
	}
	summary, err = ValidateCheckResults170(ctx, checks)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Status != "PARTIAL" {
		t.Fatalf("explicit reviewer INCOMPLETE must produce PARTIAL, got %#v", summary)
	}
}
