package main

import (
	"codea-harness-tools/internal/reviewscope"
	"codea-harness-tools/internal/reviewselection"
	"codea-harness-tools/internal/reviewunit"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func groupedReviewOptions(t *testing.T, target string, padded ...bool) reviewselection.Options {
	t.Helper()
	analysisPath := setupTask4ReviewContextProject(t)
	data, err := os.ReadFile(analysisPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	doc["affectedControllers"] = append(doc["affectedControllers"].([]any), map[string]any{"controller": "RefundController", "endpoints": []any{"RefundController.refund"}, "impactType": "AFFECTED_BY_CALL_CHAIN", "sourceSymbols": []any{"OrderServiceImpl.approve"}})
	doc["callChains"] = append(doc["callChains"].([]any), map[string]any{"entryPoint": "RefundController.refund", "chain": []any{"RefundController.refund", "OrderService.approve", "OrderServiceImpl.approve", "OrderMapper.updateStatus"}})
	doc["symbolLocations"] = append(doc["symbolLocations"].([]any), map[string]any{"symbol": "RefundController.refund", "path": "src/main/java/com/example/order/RefundController.java", "role": "Controller", "source": "FIND_SYMBOL"})
	// Bind exact symbol identities through options, selected scope, and unit planning.
	paths := map[string]string{}
	for _, raw := range doc["symbolLocations"].([]any) {
		location := raw.(map[string]any)
		paths[location["symbol"].(string)] = location["path"].(string)
	}
	for _, raw := range doc["callChains"].([]any) {
		current := raw.(map[string]any)
		entry := current["entryPoint"].(string)
		current["entryPointRef"] = map[string]any{"symbol": entry, "path": paths[entry]}
		refs := []any{}
		for _, symbol := range current["chain"].([]any) {
			refs = append(refs, map[string]any{"symbol": symbol, "path": paths[symbol.(string)]})
		}
		current["chainRefs"] = refs
	}

	if len(padded) > 0 && padded[0] {
		for _, raw := range doc["callChains"].([]any) {
			current := raw.(map[string]any)
			current["entryPoint"] = " " + current["entryPoint"].(string) + " "
			for i, symbol := range current["chain"].([]any) {
				current["chain"].([]any)[i] = " " + symbol.(string) + " "
			}
		}
	}

	data, err = json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, analysisPath, string(data))
	merged := strings.Replace(task3AcceptedYAML, "nodes:\n", "  - symbol: RefundController.refund\n    path: src/main/java/com/example/order/RefundController.java\nnodes:\n", 1)
	writeFile(t, filepath.Join(".code-harness", "chains", "order-approve.yaml"), merged)
	installTask153ReviewContextAuthoritySchemas(t)
	installTask153ReviewSelectionSchemas(t)
	prepareCommittedCertifiedAnalysisFixture153(t, "run-task4-review", analysisPath)
	requestBody := map[string]string{"runId": "run-task4-review", "changeAnalysisPath": ".code-harness/runs/run-task4-review/analysis/change-analysis.json"}
	if target != "" {
		requestBody["target"] = target
	}
	encoded, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatal(err)
	}
	request := writeQueryRequest(t, "run-task4-review", "review-options-grouped.json", string(encoded))
	if err := run([]string{"review", "options", "--input", request}); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(".code-harness", "runs", "run-task4-review", "analysis", "review-options.json"))
	if err != nil {
		t.Fatal(err)
	}
	var options reviewselection.Options
	if err := json.Unmarshal(data, &options); err != nil {
		t.Fatal(err)
	}
	return options
}

func TestReviewGroupedBusinessChainSelectsExactCallChain(t *testing.T) {
	options := groupedReviewOptions(t, "")
	if options.Decision != reviewselection.DecisionUser || len(options.Chains) != 2 || len(options.Chains[0].EntryPoints) != 1 || options.Chains[0].SelectionID != "C1" || options.Chains[1].SelectionID != "C2" {
		t.Fatalf("unexpected options: %+v", options)
	}
	request := writeQueryRequest(t, options.RunID, "audit-auto-select.json", fmt.Sprintf(`{"runId":%q,"mode":"TARGETED","selectionIds":["C1"],"optionsHash":%q}`, options.RunID, options.OptionsHash))
	if err := run([]string{"review", "select", "--input", request}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(".code-harness", "runs", options.RunID, "analysis", "review-scope.json"))
	if err != nil {
		t.Fatal(err)
	}
	var scope map[string]any
	if err := json.Unmarshal(data, &scope); err != nil {
		t.Fatal(err)
	}
	if len(scope["selectedCallChains"].([]any)) != 1 {
		t.Fatalf("unexpected scope: %s", data)
	}
	if scope["selectedCallChains"].([]any)[0].(map[string]any)["entryPoint"] != "OrderController.approve" {
		t.Fatalf("wrong selected chain: %s", data)
	}
	var exactScope reviewscope.Selection
	if err := json.Unmarshal(data, &exactScope); err != nil {
		t.Fatal(err)
	}
	if len(exactScope.SelectedCallChains) != 1 || exactScope.SelectedCallChains[0].EntryPointRef == nil || len(exactScope.SelectedCallChains[0].ChainRefs) != 4 || !reflect.DeepEqual(exactScope.SelectedCallChains[0], options.Chains[0].CallChain) {
		t.Fatalf("scope lost exact selected payload/refs: %s", data)
	}

	copyTask153CommandContract(t, ".", "review-unit.schema.json")
	if err := run([]string{"review", "units", "--run-id", options.RunID}); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(".code-harness", "runs", options.RunID, "analysis", "review-units.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest reviewunit.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Mode != reviewunit.ModeTargeted || len(manifest.Units) != 1 || manifest.Units[0].EntryPoint != "OrderController.approve" {
		t.Fatalf("selected units expanded: %s", data)
	}
	for _, file := range manifest.Units[0].Files {
		if strings.Contains(file.Path, "RefundController") {
			t.Fatalf("unselected file in units: %+v", file)
		}
	}
}

func TestReviewTargetFilteredGroupedChainKeepsOnlyMatchingEntry(t *testing.T) {
	for _, target := range []string{"OrderController", "OrderController.approve"} {
		t.Run(target, func(t *testing.T) {
			options := groupedReviewOptions(t, target)
			if options.Decision != reviewselection.DecisionAutoSingle || len(options.Chains) != 1 || len(options.Chains[0].EntryPoints) != 1 || options.Chains[0].CallChain.EntryPoint != "OrderController.approve" {
				t.Fatalf("target filtered options leaked candidate entries: %+v", options)
			}
			request := writeQueryRequest(t, options.RunID, "select-target-subset.json", fmt.Sprintf(`{"runId":%q,"mode":"TARGETED","selectionIds":["C1"],"optionsHash":%q}`, options.RunID, options.OptionsHash))
			if err := run([]string{"review", "select", "--input", request}); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(".code-harness", "runs", options.RunID, "analysis", "review-scope.json"))
			if err != nil {
				t.Fatal(err)
			}
			var scope reviewscope.Selection
			if err := json.Unmarshal(data, &scope); err != nil {
				t.Fatal(err)
			}
			if scope.Target == nil || scope.Target.Symbol != target || scope.Mode != "TARGETED" || len(scope.SelectedCallChains) != 1 || scope.SelectedCallChains[0].EntryPoint != "OrderController.approve" {
				t.Fatalf("explicit target changed or selection expanded: %s", data)
			}
		})
	}
}

func TestReviewOptionsRebuildRejectsCallChainPayloadTamper(t *testing.T) {
	options := groupedReviewOptions(t, "")
	options.Chains[0].CallChain = options.Chains[1].CallChain
	data, err := json.Marshal(options)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(".code-harness", "runs", options.RunID, "analysis", "review-options.json"), string(data))
	request := writeQueryRequest(t, options.RunID, "select-mutated-payload.json", fmt.Sprintf(`{"runId":%q,"mode":"TARGETED","selectionIds":["C1"],"optionsHash":%q}`, options.RunID, options.OptionsHash))
	if err := run([]string{"review", "select", "--input", request}); err == nil || !strings.Contains(err.Error(), "REVIEW_OPTIONS_STALE") {
		t.Fatalf("mutated callChain payload accepted: %v", err)
	}
}

func TestReviewPaddedCertifiedChainsCannotDisappearFromOptions(t *testing.T) {
	options := groupedReviewOptions(t, "", true)
	if options.Decision != reviewselection.DecisionUser || len(options.Chains) != 2 {
		t.Fatalf("certified chains silently disappeared: %+v", options)
	}
	for _, option := range options.Chains {
		if !strings.HasPrefix(option.CallChain.EntryPoint, " ") {
			t.Fatalf("exact certified payload was rewritten: %+v", option)
		}
	}
}
