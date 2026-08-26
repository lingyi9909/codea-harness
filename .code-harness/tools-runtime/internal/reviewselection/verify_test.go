package reviewselection

import (
	"strings"
	"testing"
)

func task153SelectionOptions(t *testing.T) Options {
	t.Helper()
	got, err := finalizeOptions153(Options{
		RunID: "r153",
		ChangeSetSHA256: strings.Repeat("b", 64),
		EntrypointCompleteness: "COMPLETE",
		Chains: []ChainOption{
			{ChainID: "order", EntryPoints: []string{"OrderController.create"}, Source: "ACCEPTED", Status: "VALID"},
			{ChainID: "refund", EntryPoints: []string{"RefundController.refund"}, Source: "ACCEPTED", Status: "VALID"},
		},
	}, strings.Repeat("a", 64))
	if err != nil { t.Fatal(err) }
	return got
}

func Test153SelectionRejectsStaleOptionsHash(t *testing.T) {
	options := task153SelectionOptions(t)
	_, err := validateSelectionAgainstOptions153(options, SelectionRequest{RunID: "r153", Mode: "TARGETED", SelectionIDs: []string{"C1"}, OptionsHash: strings.Repeat("f", 64)})
	if err == nil || !strings.Contains(err.Error(), "REVIEW_OPTIONS_STALE") {
		t.Fatalf("stale hash must fail closed, got %v", err)
	}
}

func Test153SelectionRejectsUnknownChain(t *testing.T) {
	options := task153SelectionOptions(t)
	_, err := validateSelectionAgainstOptions153(options, SelectionRequest{RunID: "r153", Mode: "TARGETED", SelectionIDs: []string{"C9"}, OptionsHash: options.OptionsHash})
	if err == nil || !strings.Contains(err.Error(), "REVIEW_SELECTION_UNKNOWN_CHAIN") {
		t.Fatalf("unknown selection id must fail closed, got %v", err)
	}
}

func Test153SelectionAcceptsRuntimeBoundIDs(t *testing.T) {
	options := task153SelectionOptions(t)
	selected, err := validateSelectionAgainstOptions153(options, SelectionRequest{RunID: "r153", Mode: "TARGETED", SelectionIDs: []string{"C2", "C1"}, OptionsHash: options.OptionsHash})
	if err != nil { t.Fatal(err) }
	if len(selected) != 2 || selected[0].SelectionID != "C1" || selected[1].SelectionID != "C2" {
		t.Fatalf("selected options must be canonicalized by Runtime ids: %+v", selected)
	}
}

func Test153AutoSingleSelectionIsMachineExecutable(t *testing.T) {
	options, err := finalizeOptions153(Options{RunID: "r153", ChangeSetSHA256: strings.Repeat("b", 64), EntrypointCompleteness: "COMPLETE", Chains: []ChainOption{{ChainID: "order", EntryPoints: []string{"OrderController.create"}, Source: "ACCEPTED", Status: "VALID"}}}, strings.Repeat("a", 64))
	if err != nil { t.Fatal(err) }
	selected, err := validateSelectionAgainstOptions153(options, SelectionRequest{RunID: "r153", Mode: "TARGETED", SelectionIDs: options.AutoSelectionIDs, OptionsHash: options.OptionsHash})
	if err != nil { t.Fatal(err) }
	if len(selected) != 1 || selected[0].SelectionID != "C1" { t.Fatalf("AUTO_SINGLE must execute C1 directly: %+v", selected) }
}

func Test153RuntimeAutoDecisionCannotBeOverriddenBySelectionRequest(t *testing.T) {
	autoFull, err := finalizeOptions153(Options{RunID: "r153", ChangeSetSHA256: strings.Repeat("b", 64), EntrypointCompleteness: "COMPLETE"}, strings.Repeat("a", 64))
	if err != nil { t.Fatal(err) }
	if _, err := validateSelectionAgainstOptions153(autoFull, SelectionRequest{RunID: "r153", Mode: "TARGETED", SelectionIDs: []string{"C1"}, OptionsHash: autoFull.OptionsHash}); err == nil || !strings.Contains(err.Error(), "REVIEW_SELECTION_SCOPE_INVALID") {
		t.Fatalf("AUTO_FULL must not be overridden to TARGETED, got %v", err)
	}

	autoSingle, err := finalizeOptions153(Options{RunID: "r153", ChangeSetSHA256: strings.Repeat("b", 64), EntrypointCompleteness: "COMPLETE", Chains: []ChainOption{{ChainID: "order", EntryPoints: []string{"OrderController.create"}, Source: "ACCEPTED", Status: "VALID"}}}, strings.Repeat("a", 64))
	if err != nil { t.Fatal(err) }
	if _, err := validateSelectionAgainstOptions153(autoSingle, SelectionRequest{RunID: "r153", Mode: "FULL", OptionsHash: autoSingle.OptionsHash}); err == nil || !strings.Contains(err.Error(), "REVIEW_SELECTION_SCOPE_INVALID") {
		t.Fatalf("AUTO_SINGLE must not be overridden to FULL, got %v", err)
	}
}

func Test153UserSelectionAllowsExplicitFullReviewChoice(t *testing.T) {
	options := task153SelectionOptions(t)
	selected, err := validateSelectionAgainstOptions153(options, SelectionRequest{RunID: "r153", Mode: "FULL", OptionsHash: options.OptionsHash})
	if err != nil {
		t.Fatalf("USER_SELECTION must allow the user to choose full review: %v", err)
	}
	if len(selected) != 0 {
		t.Fatalf("FULL review must not carry Chain selections: %+v", selected)
	}
}

func Test153UserSelectionFullReviewRejectsChainIDs(t *testing.T) {
	options := task153SelectionOptions(t)
	_, err := validateSelectionAgainstOptions153(options, SelectionRequest{RunID: "r153", Mode: "FULL", SelectionIDs: []string{"C1"}, OptionsHash: options.OptionsHash})
	if err == nil || !strings.Contains(err.Error(), "REVIEW_SELECTION_SCOPE_INVALID") {
		t.Fatalf("FULL review with Chain IDs must fail closed, got %v", err)
	}
}
