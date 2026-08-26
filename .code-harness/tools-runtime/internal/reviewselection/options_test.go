package reviewselection

import (
	"strings"
	"testing"
)

func Test153ReviewOptionsDecisionZeroOneTwo(t *testing.T) {
	analysisHash := strings.Repeat("a", 64)
	cases := []struct {
		name     string
		chains   []ChainOption
		decision Decision
		auto     int
	}{
		{name: "zero", chains: nil, decision: DecisionAutoFull, auto: 0},
		{name: "one", chains: []ChainOption{{ChainID: "order", EntryPoints: []string{"OrderController.create"}, Source: "ACCEPTED", Status: "VALID"}}, decision: DecisionAutoSingle, auto: 1},
		{name: "two", chains: []ChainOption{{ChainID: "refund", EntryPoints: []string{"RefundController.refund"}, Source: "ACCEPTED", Status: "VALID"}, {ChainID: "order", EntryPoints: []string{"OrderController.create"}, Source: "TEMPORARY", Status: "TEMPORARY"}}, decision: DecisionUser, auto: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := finalizeOptions153(Options{RunID: "r153", ChangeSetSHA256: strings.Repeat("b", 64), EntrypointCompleteness: "COMPLETE", Chains: tc.chains}, analysisHash)
			if err != nil { t.Fatal(err) }
			if got.Decision != tc.decision { t.Fatalf("decision=%s want=%s", got.Decision, tc.decision) }
			if len(got.AutoSelectionIDs) != tc.auto { t.Fatalf("autoSelectionIds=%v", got.AutoSelectionIDs) }
			if len(got.Chains) > 0 && got.Chains[0].SelectionID != "C1" { t.Fatalf("first selection id=%q", got.Chains[0].SelectionID) }
			if len(got.OptionsHash) != 64 { t.Fatalf("optionsHash=%q", got.OptionsHash) }
		})
	}
}

func Test153ReviewOptionsStableSortBeforeSelectionIDs(t *testing.T) {
	got, err := finalizeOptions153(Options{
		RunID: "r153",
		ChangeSetSHA256: strings.Repeat("b", 64),
		EntrypointCompleteness: "COMPLETE",
		Chains: []ChainOption{
			{ChainID: "z-chain", EntryPoints: []string{"ZController.run"}, Source: "ACCEPTED", Status: "VALID"},
			{ChainID: "a-chain", EntryPoints: []string{"AController.run"}, Source: "ACCEPTED", Status: "VALID"},
		},
	}, strings.Repeat("a", 64))
	if err != nil { t.Fatal(err) }
	if got.Chains[0].ChainID != "a-chain" || got.Chains[0].SelectionID != "C1" || got.Chains[1].SelectionID != "C2" {
		t.Fatalf("unstable options: %+v", got.Chains)
	}
}

func Test153ReviewOptionsRejectIncompleteEntrypointInventory(t *testing.T) {
	_, err := finalizeOptions153(Options{RunID: "r153", ChangeSetSHA256: strings.Repeat("b", 64), EntrypointCompleteness: "INCOMPLETE"}, strings.Repeat("a", 64))
	if err == nil || !strings.Contains(err.Error(), "ENTRYPOINT_COMPLETENESS_INCOMPLETE") {
		t.Fatalf("incomplete inventory must block options, got %v", err)
	}
}
