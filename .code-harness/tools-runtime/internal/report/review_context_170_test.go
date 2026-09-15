package report

import (
	"testing"

	"codea-harness-tools/internal/finding"
)

func Test170PartialReviewContextPreservesCertifiedFindingAndRequiresManualAction(t *testing.T) {
	req := ReviewRequest{RunID: "run-170", HarnessVersion: "1.7.0-dev", BaseRef: "base", Head: "head", Result: ResultPassed, Mode: "FULL", Scope: ReviewScope{ChangedFiles: []string{"src/main/java/A.java"}}, Coverage: ReviewCoverage{ReviewedFiles: []string{"src/main/java/A.java"}, CallChains: []CallChain{}, ExternalDependencies: []string{}, Unresolved: []string{}, MissingReviewedFiles: []string{}, RuntimeErrors: []string{}, Status: "COMPLETE"}, Findings: []Finding{}}
	set := finding.CertifiedSet{Findings: []finding.CertifiedFinding{{ID: "CF-1", RuleID: "SPRING-TX-001", ReviewUnitID: "RU-1", Category: "PRODUCTION_CODE", Severity: "high", Anchor: finding.Anchor{Kind: finding.AnchorFile, Path: "src/main/java/A.java"}, EvidenceRefs: []finding.EvidenceRef{{Kind: "CHANGED_RANGE", Path: "src/main/java/A.java", StartLine: 1, EndLine: 1}}, Problem: "p", Impact: "i", Recommendation: "r", Confidence: 0.9}}, ReviewContext: &finding.ReviewContextSummary170{Status: "PARTIAL", BlockedChecks: []finding.BlockedCheck170{{ReviewUnitID: "RU-1", RuleID: "SPRING-AUTH-001", Reasons: []string{"RELATION_REQUIRED:JAVA_CALL"}}}}}
	got, err := finalizeCertifiedRequest170(req, set)
	if err != nil {
		t.Fatal(err)
	}
	if got.Result != ResultManualActionRequired {
		t.Fatalf("PARTIAL must require manual action, got %s", got.Result)
	}
	if got.Coverage.Status != "PARTIAL" {
		t.Fatalf("PARTIAL summary must force partial coverage, got %s", got.Coverage.Status)
	}
	if len(got.Findings) != 1 || got.Findings[0].ID != "CF-1" {
		t.Fatalf("certified finding must be preserved during partial review: %#v", got.Findings)
	}
	if len(got.Coverage.Unresolved) == 0 {
		t.Fatal("blocked check must be visible as an incomplete-review gap")
	}
}

func Test170PartialZeroFindingCannotBecomePassed(t *testing.T) {
	req := ReviewRequest{RunID: "run-170", HarnessVersion: "1.7.0-dev", BaseRef: "base", Head: "head", Result: ResultPassed, Mode: "FULL", Scope: ReviewScope{ChangedFiles: []string{}}, Coverage: ReviewCoverage{ReviewedFiles: []string{}, CallChains: []CallChain{}, ExternalDependencies: []string{}, Unresolved: []string{}, MissingReviewedFiles: []string{}, RuntimeErrors: []string{}, Status: "COMPLETE"}, Findings: []Finding{}}
	set := finding.CertifiedSet{Findings: []finding.CertifiedFinding{}, ReviewContext: &finding.ReviewContextSummary170{Status: "PARTIAL", BlockedChecks: []finding.BlockedCheck170{{ReviewUnitID: "RU-1", RuleID: "SPRING-TX-001", Reasons: []string{"REVIEWER_INCOMPLETE"}}}}}
	got, err := finalizeCertifiedRequest170(req, set)
	if err != nil {
		t.Fatal(err)
	}
	if got.Result == ResultPassed {
		t.Fatal("zero findings with blocked checks must not pass")
	}
}

func Test170CompleteZeroFindingCanPass(t *testing.T) {
	req := ReviewRequest{RunID: "run-170", HarnessVersion: "1.7.0-dev", BaseRef: "base", Head: "head", Result: ResultManualActionRequired, Mode: "FULL", Scope: ReviewScope{ChangedFiles: []string{}}, Coverage: ReviewCoverage{ReviewedFiles: []string{}, CallChains: []CallChain{}, ExternalDependencies: []string{}, Unresolved: []string{}, MissingReviewedFiles: []string{}, RuntimeErrors: []string{}, Status: "COMPLETE"}, Findings: []Finding{}}
	set := finding.CertifiedSet{Findings: []finding.CertifiedFinding{}, ReviewContext: &finding.ReviewContextSummary170{Status: "COMPLETE", BlockedChecks: []finding.BlockedCheck170{}}}
	got, err := finalizeCertifiedRequest170(req, set)
	if err != nil {
		t.Fatal(err)
	}
	if got.Result != ResultPassed || got.Coverage.Status != "COMPLETE" {
		t.Fatalf("complete zero-finding review should pass, got result=%s coverage=%s", got.Result, got.Coverage.Status)
	}
}
