package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/reviewselection"
)

const task153SecondAcceptedChainYAML = `version: 1
id: refund-approve
name: 退款审批
status: ACCEPTED
entryPoints:
  - symbol: RefundController.refund
    path: src/main/java/com/example/order/RefundController.java
nodes:
  - symbol: RefundService.refund
    path: src/main/java/com/example/order/RefundService.java
    role: SERVICE
  - symbol: OrderServiceImpl.approve
    path: src/main/java/com/example/order/OrderServiceImpl.java
    role: SERVICE
  - symbol: OrderMapper.updateStatus
    path: src/main/java/com/example/order/OrderMapper.java
    role: MAPPER
resources:
  - path: src/main/resources/mapper/OrderMapper.xml
    symbol: OrderMapper.updateStatus
    role: MAPPER_XML
notes: 退款进入共享审批下游
`

func task153PrepareReviewOptionsFixture(t *testing.T, twoChains bool, target string) (string, reviewselection.Options) {
	t.Helper()
	analysisPath := setupTask4ReviewContextProject(t)
	if twoChains {
		writeFile(t, filepath.Join(".code-harness", "chains", "refund-approve.yaml"), task153SecondAcceptedChainYAML)
		data, err := os.ReadFile(analysisPath)
		if err != nil { t.Fatal(err) }
		var doc map[string]any
		if err := json.Unmarshal(data, &doc); err != nil { t.Fatal(err) }
		doc["affectedControllers"] = append(doc["affectedControllers"].([]any), map[string]any{
			"controller": "RefundController", "endpoints": []any{"RefundController.refund"},
			"impactType": "AFFECTED_BY_CALL_CHAIN", "sourceSymbols": []any{"OrderServiceImpl.approve"},
		})
		doc["callChains"] = append(doc["callChains"].([]any), map[string]any{
			"entryPoint": "RefundController.refund",
			"chain": []any{"RefundController.refund", "RefundService.refund", "OrderServiceImpl.approve", "OrderMapper.updateStatus"},
		})
		doc["symbolLocations"] = append(doc["symbolLocations"].([]any),
			map[string]any{"symbol": "RefundController.refund", "path": "src/main/java/com/example/order/RefundController.java", "role": "Controller", "source": "FIND_SYMBOL"},
			map[string]any{"symbol": "RefundService.refund", "path": "src/main/java/com/example/order/RefundService.java", "role": "Service", "source": "FIND_SYMBOL"},
		)
		updated, err := json.MarshalIndent(doc, "", "  ")
		if err != nil { t.Fatal(err) }
		updated = append(updated, '\n')
		if err := os.WriteFile(analysisPath, updated, 0o644); err != nil { t.Fatal(err) }
	}
	installTask153ReviewContextAuthoritySchemas(t)
	installTask153ReviewSelectionSchemas(t)
	prepareCommittedCertifiedAnalysisFixture153(t, "run-task4-review", analysisPath)
	body := map[string]any{
		"runId": "run-task4-review",
		"changeAnalysisPath": ".code-harness/runs/run-task4-review/analysis/change-analysis.json",
	}
	if strings.TrimSpace(target) != "" { body["target"] = target }
	requestBytes, err := json.Marshal(body)
	if err != nil { t.Fatal(err) }
	request := writeQueryRequest(t, "run-task4-review", "review-options-authority.json", string(requestBytes))
	if err := run([]string{"review", "options", "--input", request}); err != nil { t.Fatalf("review options: %v", err) }
	optionsPath := filepath.Join(".code-harness", "runs", "run-task4-review", "analysis", "review-options.json")
	data, err := os.ReadFile(optionsPath)
	if err != nil { t.Fatal(err) }
	var options reviewselection.Options
	if err := json.Unmarshal(data, &options); err != nil { t.Fatal(err) }
	return analysisPath, options
}

func task153AttackerOptionsHash(t *testing.T, raw map[string]any, analysisHash string) string {
	t.Helper()
	fields := []struct{name string; value any}{
		{"analysisHash", analysisHash},
		{"runId", raw["runId"]},
		{"changeSetSha256", raw["changeSetSha256"]},
		{"entrypointCompleteness", raw["entrypointCompleteness"]},
	}
	if intent, ok := raw["intent"]; ok { fields = append(fields, struct{name string; value any}{"intent", intent}) }
	fields = append(fields, struct{name string; value any}{"decision", raw["decision"]})
	if auto, ok := raw["autoSelectionIds"]; ok { fields = append(fields, struct{name string; value any}{"autoSelectionIds", auto}) }
	fields = append(fields, struct{name string; value any}{"chains", raw["chains"]})
	var b strings.Builder
	b.WriteByte('{')
	for i, field := range fields {
		if i > 0 { b.WriteByte(',') }
		name, _ := json.Marshal(field.name)
		value, err := json.Marshal(field.value)
		if err != nil { t.Fatal(err) }
		b.Write(name); b.WriteByte(':'); b.Write(value)
	}
	b.WriteByte('}')
	return fmt.Sprintf("%x", sha256.Sum256([]byte(b.String())))
}

func task153ReadRawOptions(t *testing.T) (string, map[string]any) {
	t.Helper()
	path := filepath.Join(".code-harness", "runs", "run-task4-review", "analysis", "review-options.json")
	data, err := os.ReadFile(path)
	if err != nil { t.Fatal(err) }
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil { t.Fatal(err) }
	return path, raw
}

func task153WriteRawOptions(t *testing.T, path string, raw map[string]any) {
	t.Helper()
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil { t.Fatal(err) }
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil { t.Fatal(err) }
}

func Test153ReviewSelectRejectsRehashedOptionSetDeletion(t *testing.T) {
	analysisPath, options := task153PrepareReviewOptionsFixture(t, true, "")
	if options.Decision != reviewselection.DecisionUser || len(options.Chains) != 2 { t.Fatalf("fixture must produce C1+C2 USER_SELECTION: %+v", options) }
	_, cert, err := loadCertifiedAnalysis153(".", filepath.ToSlash(analysisPath))
	if err != nil { t.Fatal(err) }
	optionsPath, raw := task153ReadRawOptions(t)
	chains := raw["chains"].([]any)
	raw["chains"] = chains[:1]
	raw["decision"] = "AUTO_SINGLE"
	raw["autoSelectionIds"] = []any{"C1"}
	raw["optionsHash"] = task153AttackerOptionsHash(t, raw, cert.AnalysisSHA256)
	task153WriteRawOptions(t, optionsPath, raw)
	selection := writeQueryRequest(t, "run-task4-review", "review-select-rehashed-delete.json", fmt.Sprintf(`{"runId":"run-task4-review","mode":"TARGETED","selectionIds":["C1"],"optionsHash":"%s"}`, raw["optionsHash"]))
	err = run([]string{"review", "select", "--input", selection})
	if err == nil || !strings.Contains(err.Error(), "REVIEW_OPTIONS_STALE") { t.Fatalf("rehashed incomplete option set must fail closed, got %v", err) }
	if _, statErr := os.Stat(filepath.Join(".code-harness", "runs", "run-task4-review", "analysis", "review-scope.json")); !os.IsNotExist(statErr) {
		t.Fatalf("rejected option tamper must make 0 review-scope writes, stat=%v", statErr)
	}
}

func Test153ReviewSelectRejectsJointOriginAndOptionsIntentRewrite(t *testing.T) {
	analysisPath, options := task153PrepareReviewOptionsFixture(t, true, "")
	if options.Decision != reviewselection.DecisionUser || len(options.Chains) != 2 || options.Intent.Mode != "FULL" {
		t.Fatalf("plain review fixture must be FULL C1+C2 USER_SELECTION: %+v", options)
	}
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
	raw["optionsHash"] = task153AttackerOptionsHash(t, raw, cert.AnalysisSHA256)
	task153WriteRawOptions(t, optionsPath, raw)

	selection := writeQueryRequest(t, "run-task4-review", "review-select-origin-options-joint-tamper.json", fmt.Sprintf(`{"runId":"run-task4-review","mode":"TARGETED","selectionIds":["C1"],"optionsHash":"%s"}`, raw["optionsHash"]))
	err = run([]string{"review", "select", "--input", selection})
	if err == nil || (!strings.Contains(err.Error(), "REVIEW_OPTIONS_STALE") && !strings.Contains(err.Error(), "REVIEW_OPTIONS_TAMPERED")) {
		t.Fatalf("joint origin/options intent rewrite must fail closed, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(".code-harness", "runs", "run-task4-review", "analysis", "review-scope.json")); !os.IsNotExist(statErr) {
		t.Fatalf("joint origin/options tamper must make 0 review-scope writes, stat=%v", statErr)
	}
}

func Test153ExplicitTargetIsImmutableAndHashBound(t *testing.T) {
	analysisPath, options := task153PrepareReviewOptionsFixture(t, false, "OrderServiceImpl.approve")
	if options.Decision != reviewselection.DecisionAutoSingle { t.Fatalf("one upstream explicit target must AUTO_SINGLE: %+v", options) }
	optionsPath, raw := task153ReadRawOptions(t)
	intent, ok := raw["intent"].(map[string]any)
	if !ok { t.Fatal("ReviewOptions must persist immutable intent/target") }
	if intent["mode"] != "TARGETED" || intent["target"] != "OrderServiceImpl.approve" { t.Fatalf("unexpected intent: %#v", intent) }
	_, cert, err := loadCertifiedAnalysis153(".", filepath.ToSlash(analysisPath))
	if err != nil { t.Fatal(err) }
	intent["target"] = "OrderService.approve"
	raw["intent"] = intent
	raw["optionsHash"] = task153AttackerOptionsHash(t, raw, cert.AnalysisSHA256)
	task153WriteRawOptions(t, optionsPath, raw)
	selection := writeQueryRequest(t, "run-task4-review", "review-select-target-tamper.json", fmt.Sprintf(`{"runId":"run-task4-review","mode":"TARGETED","selectionIds":["C1"],"optionsHash":"%s"}`, raw["optionsHash"]))
	err = run([]string{"review", "select", "--input", selection})
	if err == nil || !strings.Contains(err.Error(), "REVIEW_OPTIONS_STALE") { t.Fatalf("intent/target tamper must fail closed, got %v", err) }
}

func Test153ExplicitTargetAutoSinglePreservesReviewScopeTarget(t *testing.T) {
	_, options := task153PrepareReviewOptionsFixture(t, false, "OrderServiceImpl.approve")
	selection := writeQueryRequest(t, "run-task4-review", "review-select-explicit-single.json", fmt.Sprintf(`{"runId":"run-task4-review","mode":"TARGETED","selectionIds":["C1"],"optionsHash":"%s"}`, options.OptionsHash))
	if err := run([]string{"review", "select", "--input", selection}); err != nil { t.Fatalf("explicit target select: %v", err) }
	scopeBytes, err := os.ReadFile(filepath.Join(".code-harness", "runs", "run-task4-review", "analysis", "review-scope.json"))
	if err != nil { t.Fatal(err) }
	var scope map[string]any
	if err := json.Unmarshal(scopeBytes, &scope); err != nil { t.Fatal(err) }
	target := scope["target"].(map[string]any)
	if target["symbol"] != "OrderServiceImpl.approve" { t.Fatalf("explicit target drifted: %#v", target) }
}

func Test153ExplicitTargetUserSelectionPreservesTargetForEveryUpstreamChoice(t *testing.T) {
	_, options := task153PrepareReviewOptionsFixture(t, true, "OrderServiceImpl.approve")
	if options.Decision != reviewselection.DecisionUser || len(options.Chains) != 2 { t.Fatalf("two upstream explicit target must USER_SELECTION: %+v", options) }
	for _, id := range []string{"C1", "C2"} {
		requestName := "review-select-explicit-" + strings.ToLower(id) + ".json"
		selection := writeQueryRequest(t, "run-task4-review", requestName, fmt.Sprintf(`{"runId":"run-task4-review","mode":"TARGETED","selectionIds":["%s"],"optionsHash":"%s"}`, id, options.OptionsHash))
		if err := run([]string{"review", "select", "--input", selection}); err != nil { t.Fatalf("explicit target %s select: %v", id, err) }
		scopeBytes, err := os.ReadFile(filepath.Join(".code-harness", "runs", "run-task4-review", "analysis", "review-scope.json"))
		if err != nil { t.Fatal(err) }
		var scope map[string]any
		if err := json.Unmarshal(scopeBytes, &scope); err != nil { t.Fatal(err) }
		target := scope["target"].(map[string]any)
		if target["symbol"] != "OrderServiceImpl.approve" { t.Fatalf("explicit target drifted for %s: %#v", id, target) }
	}
}

func Test153ExplicitTargetWithoutMatchingChainStopsInsteadOfAutoFull(t *testing.T) {
	analysisPath := setupTask4ReviewContextProject(t)
	installTask153ReviewContextAuthoritySchemas(t)
	installTask153ReviewSelectionSchemas(t)
	prepareCommittedCertifiedAnalysisFixture153(t, "run-task4-review", analysisPath)
	request := writeQueryRequest(t, "run-task4-review", "review-options-missing-target.json", `{"runId":"run-task4-review","changeAnalysisPath":".code-harness/runs/run-task4-review/analysis/change-analysis.json","target":"PaymentService.pay"}`)
	err := run([]string{"review", "options", "--input", request})
	if err == nil || !strings.Contains(err.Error(), "REVIEW_TARGET_NOT_FOUND") { t.Fatalf("explicit target with 0 matching Chains must stop, got %v", err) }
	if _, statErr := os.Stat(filepath.Join(".code-harness", "runs", "run-task4-review", "analysis", "review-options.json")); !os.IsNotExist(statErr) {
		t.Fatalf("missing explicit target must not write review-options, stat=%v", statErr)
	}
}
