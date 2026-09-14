package main

import (
	"testing"

	"codea-harness-tools/internal/reviewrules"
)

func Test170KnowledgeBusinessRuleDoesNotInventContextRelation(t *testing.T) {
	d := reviewrules.Dispatch{
		ReviewUnitID: "u-business",
		RuleID: "BUSINESS:order-rules:ORDER-STATE-001",
		RequiredEvidence: []string{"BUSINESS_RULE", "CHANGED_RANGE"},
		DispatchReason: []string{"BUSINESS_RULE:order-rules:ORDER-STATE-001"},
	}
	if got := requiredKindsForDispatch170(d); len(got) != 0 {
		t.Fatalf("BUSINESS_RULE is T8 business evidence, not a synthetic T5 code relation: %v", got)
	}
}
