package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/reviewselection"
)

func task153ExactForgedOptionsHash(t *testing.T, raw map[string]any, analysisHash string) string {
	t.Helper()
	encoded, err := json.Marshal(raw)
	if err != nil { t.Fatal(err) }
	var forged reviewselection.Options
	if err := json.Unmarshal(encoded, &forged); err != nil { t.Fatal(err) }
	identity := struct {
		AnalysisHash           string                 `json:"analysisHash"`
		RunID                  string                 `json:"runId"`
		ChangeSetSHA256        string                 `json:"changeSetSha256"`
		EntrypointCompleteness string                 `json:"entrypointCompleteness"`
		Intent                 analysisruntime.Intent `json:"intent"`
		Decision               reviewselection.Decision `json:"decision"`
		AutoSelectionIDs       []string               `json:"autoSelectionIds,omitempty"`
		Chains                 []reviewselection.ChainOption `json:"chains"`
	}{
		AnalysisHash: analysisHash,
		RunID: forged.RunID,
		ChangeSetSHA256: forged.ChangeSetSHA256,
		EntrypointCompleteness: forged.EntrypointCompleteness,
		Intent: forged.Intent,
		Decision: forged.Decision,
		AutoSelectionIDs: forged.AutoSelectionIDs,
		Chains: forged.Chains,
	}
	canonical, err := json.Marshal(identity)
	if err != nil { t.Fatal(err) }
	return fmt.Sprintf("%x", sha256.Sum256(canonical))
}

func Test153DiagnosticJointOriginRewriteResult(t *testing.T) {
	analysisPath, options := task153PrepareReviewOptionsFixture(t, true, "")
	_, cert, err := loadCertifiedAnalysis153(".", filepath.ToSlash(analysisPath))
	if err != nil { t.Fatal(err) }

	originPath := filepath.Join(".code-harness", "runs", "run-task4-review", "analysis", "review-options-origin.json")
	originBytes, err := os.ReadFile(originPath)
	if err != nil { t.Fatal(err) }
	var origin map[string]any
	if err := json.Unmarshal(originBytes, &origin); err != nil { t.Fatal(err) }
	origin["intent"] = map[string]any{"mode": "TARGETED", "target": "OrderService.approve"}
	originBytes, err = json.MarshalIndent(origin, "", "  ")
	if err != nil { t.Fatal(err) }
	originBytes = append(originBytes, '\n')
	if err := os.WriteFile(originPath, originBytes, 0o644); err != nil { t.Fatal(err) }

	optionsPath, raw := task153ReadRawOptions(t)
	chains := raw["chains"].([]any)
	raw["intent"] = map[string]any{"mode": "TARGETED", "target": "OrderService.approve"}
	raw["chains"] = chains[:1]
	raw["decision"] = "AUTO_SINGLE"
	raw["autoSelectionIds"] = []any{"C1"}
	raw["optionsHash"] = task153ExactForgedOptionsHash(t, raw, cert.AnalysisSHA256)
	task153WriteRawOptions(t, optionsPath, raw)

	selection := writeQueryRequest(t, "run-task4-review", "review-select-origin-diagnostic.json", fmt.Sprintf(`{"runId":"run-task4-review","mode":"TARGETED","selectionIds":["C1"],"optionsHash":"%s"}`, raw["optionsHash"]))
	err = run([]string{"review", "select", "--input", selection})
	t.Fatalf("diagnostic: originalDecision=%s originalHash=%s mutatedHash=%s err=%v", options.Decision, options.OptionsHash, raw["optionsHash"], err)
}
