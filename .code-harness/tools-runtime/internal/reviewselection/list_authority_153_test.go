package reviewselection

import (
	"strings"
	"testing"
)

func Test153ListCannotOverrideAutoDecisions(t *testing.T) {
	analysisHash := strings.Repeat("a", 64)
	autoFull, err := finalizeOptions153(Options{
		RunID: "r153", ChangeSetSHA256: strings.Repeat("b", 64), EntrypointCompleteness: "COMPLETE",
	}, analysisHash)
	if err != nil { t.Fatal(err) }
	if _, err := validateSelectionAgainstOptions153(autoFull, SelectionRequest{RunID: "r153", Mode: "LIST", OptionsHash: autoFull.OptionsHash}); err == nil || !strings.Contains(err.Error(), "REVIEW_SELECTION_SCOPE_INVALID") {
		t.Fatalf("AUTO_FULL + LIST must reject, got %v", err)
	}

	autoSingle, err := finalizeOptions153(Options{
		RunID: "r153", ChangeSetSHA256: strings.Repeat("b", 64), EntrypointCompleteness: "COMPLETE",
		Chains: []ChainOption{{ChainID: "order", EntryPoints: []string{"OrderController.create"}, Source: "ACCEPTED", Status: "VALID"}},
	}, analysisHash)
	if err != nil { t.Fatal(err) }
	if _, err := validateSelectionAgainstOptions153(autoSingle, SelectionRequest{RunID: "r153", Mode: "LIST", OptionsHash: autoSingle.OptionsHash}); err == nil || !strings.Contains(err.Error(), "REVIEW_SELECTION_SCOPE_INVALID") {
		t.Fatalf("AUTO_SINGLE + LIST must reject, got %v", err)
	}
}

func Test153UserSelectionAllowsListOnly(t *testing.T) {
	options := task153SelectionOptions(t)
	selected, err := validateSelectionAgainstOptions153(options, SelectionRequest{RunID: "r153", Mode: "LIST", OptionsHash: options.OptionsHash})
	if err != nil { t.Fatalf("USER_SELECTION + LIST must remain supported: %v", err) }
	if len(selected) != 0 { t.Fatalf("LIST must not authorize Chain selection: %+v", selected) }
}
