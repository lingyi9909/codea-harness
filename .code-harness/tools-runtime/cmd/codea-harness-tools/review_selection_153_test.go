package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"codea-harness-tools/internal/reviewselection"
)

func installTask153ReviewSelectionSchemas(t *testing.T) {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(0)
	if !ok { t.Fatal("locate test source") }
	contracts := filepath.Join(filepath.Dir(testFile), "..", "..", "..", "contracts")
	for _, name := range []string{"review-options.schema.json", "review-selection-request.schema.json"} {
		data, err := os.ReadFile(filepath.Join(contracts, name))
		if err != nil { t.Fatal(err) }
		writeFile(t, filepath.Join(".code-harness", "contracts", name), string(data))
	}
}

func task153BuildReviewOptions(t *testing.T) reviewselection.Options {
	t.Helper()
	analysisPath := setupTask4ReviewContextProject(t)
	installTask153ReviewContextAuthoritySchemas(t)
	installTask153ReviewSelectionSchemas(t)
	prepareCommittedCertifiedAnalysisFixture153(t, "run-task4-review", analysisPath)
	request := writeQueryRequest(t, "run-task4-review", "review-options-request.json", `{"runId":"run-task4-review","changeAnalysisPath":".code-harness/runs/run-task4-review/analysis/change-analysis.json"}`)
	if err := run([]string{"review", "options", "--input", request}); err != nil { t.Fatalf("review options: %v", err) }
	data, err := os.ReadFile(filepath.Join(".code-harness", "runs", "run-task4-review", "analysis", "review-options.json"))
	if err != nil { t.Fatal(err) }
	var options reviewselection.Options
	if err := json.Unmarshal(data, &options); err != nil { t.Fatal(err) }
	return options
}

func Test153ReviewOptionsAutoSingleExecutesWithoutUserChoice(t *testing.T) {
	options := task153BuildReviewOptions(t)
	if options.Decision != reviewselection.DecisionAutoSingle || len(options.AutoSelectionIDs) != 1 || options.AutoSelectionIDs[0] != "C1" {
		t.Fatalf("one valid Chain must AUTO_SINGLE: %+v", options)
	}
	selection := writeQueryRequest(t, "run-task4-review", "review-auto-select.json", `{"runId":"run-task4-review","mode":"TARGETED","selectionIds":["C1"],"optionsHash":"`+options.OptionsHash+`"}`)
	if err := run([]string{"review", "select", "--input", selection}); err != nil { t.Fatalf("review select AUTO_SINGLE: %v", err) }
	scopeBytes, err := os.ReadFile(filepath.Join(".code-harness", "runs", "run-task4-review", "analysis", "review-scope.json"))
	if err != nil { t.Fatal(err) }
	if !strings.Contains(string(scopeBytes), `"mode": "TARGETED"`) || !strings.Contains(string(scopeBytes), `"OrderController.approve"`) {
		t.Fatalf("AUTO_SINGLE must produce TARGETED Runtime scope: %s", scopeBytes)
	}
}

func Test153ReviewSelectRejectsStaleAndUnknownRuntimeOptions(t *testing.T) {
	options := task153BuildReviewOptions(t)
	stale := writeQueryRequest(t, "run-task4-review", "review-select-stale.json", `{"runId":"run-task4-review","mode":"TARGETED","selectionIds":["C1"],"optionsHash":"`+strings.Repeat("f", 64)+`"}`)
	if err := run([]string{"review", "select", "--input", stale}); err == nil || !strings.Contains(err.Error(), "REVIEW_OPTIONS_STALE") {
		t.Fatalf("stale optionsHash must fail closed, got %v", err)
	}
	unknown := writeQueryRequest(t, "run-task4-review", "review-select-unknown.json", `{"runId":"run-task4-review","mode":"TARGETED","selectionIds":["C9"],"optionsHash":"`+options.OptionsHash+`"}`)
	if err := run([]string{"review", "select", "--input", unknown}); err == nil || !strings.Contains(err.Error(), "REVIEW_SELECTION_UNKNOWN_CHAIN") {
		t.Fatalf("unknown Runtime option must fail closed, got %v", err)
	}
}
