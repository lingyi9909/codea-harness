package reviewscope_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/reviewscope"
)

func Test170AutoChainStaleUsesCurrentTemporaryWithoutWritingProjectState(t *testing.T) {
	root := t.TempDir()
	persisted := acceptedOrderChain()
	persisted.Notes = "manual note must survive review"
	path := writeAcceptedChain(t, root, persisted)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	selection := task4TargetedSelection(t, task4StaleAnalysis, "RiskService.check")
	const runID = "run-t6-auto-stale"
	opts := task4CertifiedDiscoveryOptions(t, root, runID, false)
	opts.Strategy = reviewscope.ChainResolutionStrategyAutoTemporary

	result, err := reviewscope.ResolveChainContexts(root, selection, []byte(task4StaleAnalysis), opts)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != reviewscope.ChainResolutionReady || len(result.Contexts) != 1 {
		t.Fatalf("stale review must automatically use current temporary chain: %+v", result)
	}
	if result.Contexts[0].Source != "DISCOVERED" || result.Contexts[0].Status != "TEMPORARY" {
		t.Fatalf("stale chain must never masquerade as accepted: %+v", result.Contexts[0])
	}
	if result.Status == reviewscope.ChainResolutionStaleDecision {
		t.Fatalf("normal 1.7 review must not ask for stale maintenance decision: %+v", result)
	}
	if len(result.Stale) != 1 || result.Stale[0].ID != "order-approve" {
		t.Fatalf("stale project-state provenance should remain visible: %+v", result.Stale)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("AUTO_TEMPORARY review must not overwrite persisted chain bytes, name, or notes")
	}
}

func Test170AutoChainCorruptProjectStateRecoversFromCurrentAnalysis(t *testing.T) {
	root := t.TempDir()
	chainDir := filepath.Join(root, ".code-harness", "chains")
	if err := os.MkdirAll(chainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(chainDir, "order.yaml")
	const corrupt = "version: [broken yaml\nname: do-not-trust"
	if err := os.WriteFile(path, []byte(corrupt), 0o644); err != nil {
		t.Fatal(err)
	}
	selection := task4TargetedSelection(t, task4Analysis, "OrderService.approve")
	const runID = "run-t6-auto-corrupt"
	opts := task4CertifiedDiscoveryOptions(t, root, runID, false)
	opts.Strategy = reviewscope.ChainResolutionStrategyAutoTemporary

	result, err := reviewscope.ResolveChainContexts(root, selection, []byte(task4Analysis), opts)
	if err != nil {
		t.Fatalf("corrupt saved YAML must not block current certified chain recovery: %v", err)
	}
	if result.Status != reviewscope.ChainResolutionReady || len(result.Contexts) != 1 || result.Contexts[0].Status != "TEMPORARY" {
		t.Fatalf("expected recovered temporary chain: %+v", result)
	}
	if len(result.Notices) != 1 || !strings.Contains(result.Notices[0], "PERSISTED_CHAIN_INVALID") || !strings.Contains(result.Notices[0], "order.yaml") {
		t.Fatalf("corrupt saved YAML must produce one stable notice: %+v", result.Notices)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != corrupt {
		t.Fatal("review recovery must not rewrite corrupt Project State")
	}
}

func Test170AutoChainSelectedBranchDoesNotExpandToAll(t *testing.T) {
	root := t.TempDir()
	analysis := `{
	  "changedFiles":[
	    {"path":"src/main/java/OrderController.java","role":"Controller"},
	    {"path":"src/main/java/RiskService.java","role":"Service"},
	    {"path":"src/main/java/AuditService.java","role":"Service"}
	  ],
	  "affectedControllers":[{"controller":"OrderController","endpoints":["OrderController.approve"],"impactType":"DIRECT_CHANGE","sourceSymbols":["RiskService.check","AuditService.record"]}],
	  "callChains":[
	    {"entryPoint":"OrderController.approve","chain":["OrderController.approve","RiskService.check"]},
	    {"entryPoint":"OrderController.approve","chain":["OrderController.approve","AuditService.record"]}
	  ],
	  "symbolLocations":[
	    {"symbol":"OrderController.approve","path":"src/main/java/OrderController.java","role":"Controller","source":"FIND_SYMBOL"},
	    {"symbol":"RiskService.check","path":"src/main/java/RiskService.java","role":"Service","source":"FIND_SYMBOL"},
	    {"symbol":"AuditService.record","path":"src/main/java/AuditService.java","role":"Service","source":"FIND_SYMBOL"}
	  ],
	  "resourceRelations":[],"externalDependencies":[],
	  "reviewCoverage":{"reviewedFiles":[
	    {"path":"src/main/java/OrderController.java","role":"Controller"},
	    {"path":"src/main/java/RiskService.java","role":"Service"},
	    {"path":"src/main/java/AuditService.java","role":"Service"}
	  ],"unresolvedSymbols":[]}
	}`
	selectionJSON := []byte(`{
	  "mode":"TARGETED",
	  "target":{"symbol":"RiskService.check","kind":"METHOD"},
	  "selectedCallChains":[{"entryPoint":"OrderController.approve","chain":["OrderController.approve","RiskService.check"]}],
	  "scopedFiles":["src/main/java/OrderController.java","src/main/java/RiskService.java"]
	}`)
	selection, err := reviewscope.Verify(selectionJSON, []byte(analysis))
	if err != nil {
		t.Fatal(err)
	}
	const runID = "run-t6-auto-selected"
	opts := task4CertifiedDiscoveryOptions(t, root, runID, false)
	opts.Strategy = reviewscope.ChainResolutionStrategyAutoTemporary
	result, err := reviewscope.ResolveChainContexts(root, selection, []byte(analysis), opts)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != reviewscope.ChainResolutionReady || len(result.Contexts) != 1 {
		t.Fatalf("selected branch must remain exactly one temporary context: %+v", result)
	}
	entries, err := os.ReadDir(filepath.Join(root, ".code-harness", "runs", runID, "analysis", "discovered-chains"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("selected branch must publish only its YAML + cert, got %d entries: %v", len(entries), entries)
	}
}

func Test170AutoChainAmbiguousCurrentDoesNotReuseSavedTarget(t *testing.T) {
	root := t.TempDir()
	writeAcceptedChain(t, root, acceptedOrderChain())
	analysis := `{
	  "changedFiles":[{"path":"src/main/java/OrderController.java","role":"Controller"},{"path":"src/main/java/RiskService.java","role":"Service"}],
	  "affectedControllers":[{"controller":"OrderController","endpoints":["OrderController.approve"],"impactType":"DIRECT_CHANGE","sourceSymbols":["RiskService.check"]}],
	  "callChains":[{"entryPoint":"OrderController.approve","chain":["OrderController.approve","RiskService.check"]}],
	  "symbolLocations":[
	    {"symbol":"OrderController.approve","path":"src/main/java/OrderController.java","role":"Controller","source":"FIND_SYMBOL"},
	    {"symbol":"RiskService.check","path":"src/main/java/RiskService.java","role":"Service","source":"FIND_SYMBOL"},
	    {"symbol":"RiskService.check","path":"src/main/java/other/RiskService.java","role":"Service","source":"FIND_SYMBOL"}
	  ],
	  "resourceRelations":[],"externalDependencies":[],"reviewCoverage":{"unresolvedSymbols":[]}
	}`
	selection := reviewscope.Selection{Mode: "TARGETED", SelectedCallChains: []reviewscope.CallChain{{EntryPoint: "OrderController.approve", Chain: []string{"OrderController.approve", "RiskService.check"}}}}
	const runID = "run-t6-auto-ambiguous"
	opts := task4CertifiedDiscoveryOptions(t, root, runID, false)
	opts.Strategy = reviewscope.ChainResolutionStrategyAutoTemporary

	result, err := reviewscope.ResolveChainContexts(root, selection, []byte(analysis), opts)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != reviewscope.ChainResolutionPartial || len(result.Contexts) != 0 {
		t.Fatalf("ambiguous current target must stay unresolved and must not reuse stale Project State: %+v", result)
	}
	if len(result.Unresolved) == 0 || !strings.Contains(strings.Join(result.Unresolved, "\n"), "AMBIGUOUS_INTERNAL_SYMBOL") {
		t.Fatalf("expected explicit ambiguity reason: %+v", result.Unresolved)
	}
}

func Test170AutoChainSamePathOverloadAmbiguityCannotConfirmTemporaryOrReuseSaved(t *testing.T) {
	root := t.TempDir()
	persisted := acceptedOrderChain()
	persisted.Nodes[0].Symbol = "RiskService.check"
	persisted.Nodes[0].Path = "src/main/java/RiskService.java"
	path := writeAcceptedChain(t, root, persisted)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	analysis := `{
	  "changedFiles":[{"path":"src/main/java/OrderController.java","role":"Controller"},{"path":"src/main/java/RiskService.java","role":"Service"}],
	  "affectedControllers":[{"controller":"OrderController","endpoints":["OrderController.approve"],"impactType":"DIRECT_CHANGE","sourceSymbols":["RiskService.check"]}],
	  "callChains":[{"entryPoint":"OrderController.approve","chain":["OrderController.approve","RiskService.check"]}],
	  "symbolLocations":[
	    {"symbol":"OrderController.approve","path":"src/main/java/OrderController.java","role":"Controller","source":"FIND_SYMBOL"},
	    {"symbol":"RiskService.check","path":"src/main/java/RiskService.java","role":"Service","source":"FIND_SYMBOL"},
	    {"symbol":"RiskService.check","path":"src/main/java/RiskService.java","role":"Service","source":"FIND_SYMBOL"}
	  ],
	  "resourceRelations":[],"externalDependencies":[],
	  "reviewCoverage":{"unresolvedSymbols":[{"symbol":"RiskService.check","from":"OrderController.approve","reason":"AMBIGUOUS_METHOD_OVERLOAD: RiskService.check(String), RiskService.check(Long)"}]}
	}`
	selection := reviewscope.Selection{Mode: "TARGETED", SelectedCallChains: []reviewscope.CallChain{{EntryPoint: "OrderController.approve", Chain: []string{"OrderController.approve", "RiskService.check"}}}}
	const runID = "run-t6-auto-same-path-overload"
	opts := task4CertifiedDiscoveryOptions(t, root, runID, false)
	opts.Strategy = reviewscope.ChainResolutionStrategyAutoTemporary

	result, err := reviewscope.ResolveChainContexts(root, selection, []byte(analysis), opts)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != reviewscope.ChainResolutionPartial || len(result.Contexts) != 0 {
		t.Fatalf("same-path overload ambiguity must be PARTIAL with zero confirmed contexts: %+v", result)
	}
	unresolved := strings.Join(result.Unresolved, "\n")
	if !strings.Contains(unresolved, "AMBIGUOUS_METHOD_OVERLOAD") || !strings.Contains(unresolved, "RiskService.check(String)") || !strings.Contains(unresolved, "RiskService.check(Long)") {
		t.Fatalf("overload ambiguity must remain explicit and traceable: %+v", result.Unresolved)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("AUTO_TEMPORARY overload handling must not rewrite or retarget saved Project State")
	}
	entries, err := os.ReadDir(filepath.Join(root, ".code-harness", "runs", runID, "analysis", "discovered-chains"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".cert.json") {
			t.Fatalf("ambiguous overload must not produce a confirmed temporary chain: %s", entry.Name())
		}
	}
}

func Test170AutoChainDeletedEntrypointIsNoticeOnly(t *testing.T) {
	root := t.TempDir()
	persisted := acceptedOrderChain()
	path := writeAcceptedChain(t, root, persisted)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	analysis := `{
	  "changedFiles":[{"path":"src/main/java/OrderController.java","role":"Controller"}],
	  "affectedControllers":[],"callChains":[],"symbolLocations":[],"resourceRelations":[],"externalDependencies":[],
	  "reviewCoverage":{"reviewedFiles":[{"path":"src/main/java/OrderController.java","role":"Controller"}],"unresolvedSymbols":[]}
	}`
	selection, err := reviewscope.Verify([]byte(`{"mode":"FULL","selectedCallChains":[],"scopedFiles":[]}`), []byte(analysis))
	if err != nil {
		t.Fatal(err)
	}
	result, err := reviewscope.ResolveChainContexts(root, selection, []byte(analysis), reviewscope.ChainResolveOptions{RunID: "run-t6-auto-deleted", Strategy: reviewscope.ChainResolutionStrategyAutoTemporary})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Contexts) != 0 || len(result.Stale) != 0 {
		t.Fatalf("deleted entrypoint must not survive as current review chain: %+v", result)
	}
	if len(result.Notices) != 1 || result.Notices[0] != "CHAIN_ENTRYPOINT_REMOVED: OrderController.approve" {
		t.Fatalf("deleted entrypoint must be represented only by deletion notice: %+v", result.Notices)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("deleted-entry review must not mutate Project State")
	}
}
