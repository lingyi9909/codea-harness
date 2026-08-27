package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test160ReviewUnitsCommandWritesRuntimeOwnedManifest(t *testing.T) {
	options := task153BuildReviewOptions(t)
	if len(options.AutoSelectionIDs) != 1 {
		t.Fatalf("fixture must produce one AUTO_SINGLE selection, got %+v", options)
	}
	selection := writeQueryRequest(t, "run-task4-review", "review-unit-auto-select.json", `{"runId":"run-task4-review","mode":"TARGETED","selectionIds":["`+options.AutoSelectionIDs[0]+`"],"optionsHash":"`+options.OptionsHash+`"}`)
	if err := run([]string{"review", "select", "--input", selection}); err != nil {
		t.Fatalf("review select fixture: %v", err)
	}
	copyTask153CommandContract(t, ".", "review-unit.schema.json")

	if err := run([]string{"review", "units", "--run-id", "run-task4-review"}); err != nil {
		t.Fatalf("review units: %v", err)
	}
	artifactPath := filepath.Join(".code-harness", "runs", "run-task4-review", "analysis", "review-units.json")
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read review units artifact: %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode review units artifact: %v", err)
	}
	if manifest["runId"] != "run-task4-review" || manifest["mode"] != "TARGETED" {
		t.Fatalf("unexpected review unit manifest identity: %s", data)
	}
	units, _ := manifest["units"].([]any)
	if len(units) == 0 {
		t.Fatalf("review units command must publish at least one Runtime unit: %s", data)
	}
	if _, err := os.Stat(filepath.Join(".code-harness", "runs", "run-task4-review", "requests", "review-units.json")); !os.IsNotExist(err) {
		t.Fatalf("review units must not publish authority under requests/**, stat=%v", err)
	}
}

func Test160ReviewUnitsCommandRequiresRunID(t *testing.T) {
	if err := run([]string{"review", "units"}); err == nil || !strings.Contains(err.Error(), "--run-id") {
		t.Fatalf("review units without run id must reject, got %v", err)
	}
}

func Test160ReviewUnitsCommandRejectsNonCanonicalRunID(t *testing.T) {
	options := task153BuildReviewOptions(t)
	if len(options.AutoSelectionIDs) != 1 {
		t.Fatalf("fixture must produce one AUTO_SINGLE selection, got %+v", options)
	}
	selection := writeQueryRequest(t, "run-task4-review", "review-unit-auto-select.json", `{"runId":"run-task4-review","mode":"TARGETED","selectionIds":["`+options.AutoSelectionIDs[0]+`"],"optionsHash":"`+options.OptionsHash+`"}`)
	if err := run([]string{"review", "select", "--input", selection}); err != nil {
		t.Fatalf("review select fixture: %v", err)
	}
	copyTask153CommandContract(t, ".", "review-unit.schema.json")

	err := run([]string{"review", "units", "--run-id", " run-task4-review "})
	if err == nil || !strings.Contains(err.Error(), "REVIEW_UNIT_RUN_ID_INVALID") {
		t.Fatalf("review units must reject a non-canonical run id before any authority read/write, got %v", err)
	}
	wrongPath := filepath.Join(".code-harness", "runs", " run-task4-review ", "analysis", "review-units.json")
	if _, statErr := os.Stat(wrongPath); !os.IsNotExist(statErr) {
		t.Fatalf("non-canonical run id must not create an alternate Runtime-owned path, stat=%v", statErr)
	}
}
