package reviewscope_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/chain"
	"codea-harness-tools/internal/reviewscope"
)

const task4Analysis = `{
  "changedFiles":[
    {"path":"src/main/java/OrderController.java","role":"Controller"},
    {"path":"src/main/java/OrderService.java","role":"Service"},
    {"path":"src/main/java/UnrelatedService.java","role":"Service"}
  ],
  "affectedControllers":[
    {"controller":"OrderController","endpoints":["OrderController.approve"],"impactType":"DIRECT_CHANGE","sourceSymbols":["OrderService.approve"]}
  ],
  "callChains":[
    {"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderService.approve"]}
  ],
  "symbolLocations":[
    {"symbol":"OrderController.approve","path":"src/main/java/OrderController.java","role":"Controller","source":"FIND_SYMBOL"},
    {"symbol":"OrderService.approve","path":"src/main/java/OrderService.java","role":"Service","source":"FIND_SYMBOL"}
  ],
  "resourceRelations":[],
  "externalDependencies":[],
  "reviewCoverage":{
    "reviewedFiles":[
      {"path":"src/main/java/OrderController.java","role":"Controller"},
      {"path":"src/main/java/OrderService.java","role":"Service"}
    ],
    "unresolvedSymbols":[]
  }
}`

const task4StaleAnalysis = `{
  "changedFiles":[
    {"path":"src/main/java/OrderController.java","role":"Controller"},
    {"path":"src/main/java/RiskService.java","role":"Service"}
  ],
  "affectedControllers":[
    {"controller":"OrderController","endpoints":["OrderController.approve"],"impactType":"AFFECTED_BY_CALL_CHAIN","sourceSymbols":["RiskService.check"]}
  ],
  "callChains":[
    {"entryPoint":"OrderController.approve","chain":["OrderController.approve","RiskService.check"]}
  ],
  "symbolLocations":[
    {"symbol":"OrderController.approve","path":"src/main/java/OrderController.java","role":"Controller","source":"FIND_SYMBOL"},
    {"symbol":"RiskService.check","path":"src/main/java/RiskService.java","role":"Service","source":"FIND_SYMBOL"}
  ],
  "resourceRelations":[],
  "externalDependencies":[],
  "reviewCoverage":{
    "reviewedFiles":[
      {"path":"src/main/java/OrderController.java","role":"Controller"},
      {"path":"src/main/java/RiskService.java","role":"Service"}
    ],
    "unresolvedSymbols":[]
  }
}`

func task4TargetedSelection(t *testing.T, analysis string, service string) reviewscope.Selection {
	t.Helper()
	selection := []byte(`{
      "mode":"TARGETED",
      "target":{"symbol":"OrderController.approve","kind":"METHOD"},
      "selectedCallChains":[{"entryPoint":"OrderController.approve","chain":["OrderController.approve","` + service + `"]}],
      "scopedFiles":["src/main/java/OrderController.java","src/main/java/` + className(service) + `.java"]
    }`)
	verified, err := reviewscope.Verify(selection, []byte(analysis))
	if err != nil {
		t.Fatal(err)
	}
	return verified
}

func className(symbol string) string {
	for i := len(symbol) - 1; i >= 0; i-- {
		if symbol[i] == '.' {
			return symbol[:i]
		}
	}
	return symbol
}

func writeAcceptedChain(t *testing.T, root string, c chain.Chain) string {
	t.Helper()
	path, err := chain.ChainPath(root, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := chain.MarshalYAML(c)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func acceptedOrderChain() chain.Chain {
	return chain.Chain{
		Version: 1,
		ID:      "order-approve",
		Name:    "订单审批",
		Status:  chain.StatusAccepted,
		EntryPoints: []chain.EntryPoint{{
			Symbol: "OrderController.approve",
			Path:   "src/main/java/OrderController.java",
		}},
		Nodes: []chain.Node{{
			Symbol: "OrderService.approve",
			Path:   "src/main/java/OrderService.java",
			Role:   "SERVICE",
		}},
	}
}

func task4CertifiedDiscoveryOptions(t *testing.T, root, runID string, allowTemporaryForStale bool) reviewscope.ChainResolveOptions {
	t.Helper()
	schemaBytes, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "chain-candidate-cert.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	schemaPath := filepath.Join(root, ".code-harness", "contracts", "chain-candidate-cert.schema.json")
	if err := os.MkdirAll(filepath.Dir(schemaPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(schemaPath, schemaBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	cert := analysisruntime.Certificate{RunID: runID, AnalysisSHA256: strings.Repeat("a", 64)}
	return reviewscope.ChainResolveOptions{
		RunID:                  runID,
		AllowTemporaryForStale: allowTemporaryForStale,
		CertifyDiscovered: func(candidate chain.Chain) error {
			candidatePath := filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "analysis", "discovered-chains", candidate.ID+".yaml"))
			_, err := chain.CertifyCandidate(root, candidate, candidatePath, "DISCOVERED", cert)
			return err
		},
	}
}

func assertTask4DiscoveredYAMLAndCert(t *testing.T, root, runID string) {
	t.Helper()
	dir := filepath.Join(root, ".code-harness", "runs", runID, "analysis", "discovered-chains")
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatalf("temporary discovery must publish YAML + cert: entries=%v err=%v", entries, err)
	}
	var yamlFound, certFound bool
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".cert.json") {
			certFound = true
		} else if strings.EqualFold(filepath.Ext(entry.Name()), ".yaml") {
			yamlFound = true
		}
	}
	if !yamlFound || !certFound {
		t.Fatalf("temporary discovery must publish paired YAML + cert: entries=%v", entries)
	}
}

func TestResolveChainContextsReusesValidAcceptedChain(t *testing.T) {
	root := t.TempDir()
	writeAcceptedChain(t, root, acceptedOrderChain())
	selection := task4TargetedSelection(t, task4Analysis, "OrderService.approve")

	result, err := reviewscope.ResolveChainContexts(root, selection, []byte(task4Analysis), reviewscope.ChainResolveOptions{RunID: "run-task4-accepted"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != reviewscope.ChainResolutionReady || len(result.Contexts) != 1 {
		t.Fatalf("accepted chain should be ready: %+v", result)
	}
	ctx := result.Contexts[0]
	if ctx.ID != "order-approve" || ctx.Name != "订单审批" || ctx.Source != "ACCEPTED" || ctx.Status != "VALID" {
		t.Fatalf("unexpected accepted context: %+v", ctx)
	}
	if _, err := os.Stat(filepath.Join(root, ".code-harness", "runs", "run-task4-accepted", "analysis", "discovered-chains")); !os.IsNotExist(err) {
		t.Fatalf("accepted reuse must not run discovery, stat err=%v", err)
	}
}

func TestResolveChainContextsLazilyDiscoversWhenAcceptedMissing(t *testing.T) {
	root := t.TempDir()
	selection := task4TargetedSelection(t, task4Analysis, "OrderService.approve")
	const runID = "run-task4-missing"

	result, err := reviewscope.ResolveChainContexts(root, selection, []byte(task4Analysis), task4CertifiedDiscoveryOptions(t, root, runID, false))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != reviewscope.ChainResolutionReady || len(result.Contexts) != 1 {
		t.Fatalf("missing accepted chain should use temporary discovery: %+v", result)
	}
	ctx := result.Contexts[0]
	if ctx.Source != "DISCOVERED" || ctx.Status != "TEMPORARY" || ctx.ID == "" || ctx.Name == "" {
		t.Fatalf("unexpected temporary context: %+v", ctx)
	}
	assertTask4DiscoveredYAMLAndCert(t, root, runID)
	if _, err := os.Stat(filepath.Join(root, ".code-harness", "chains")); !os.IsNotExist(err) {
		t.Fatalf("review discovery must not persist Project State, stat err=%v", err)
	}
}

func TestResolveChainContextsStaleRequiresExplicitDecision(t *testing.T) {
	root := t.TempDir()
	path := writeAcceptedChain(t, root, acceptedOrderChain())
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	selection := task4TargetedSelection(t, task4StaleAnalysis, "RiskService.check")

	result, err := reviewscope.ResolveChainContexts(root, selection, []byte(task4StaleAnalysis), reviewscope.ChainResolveOptions{RunID: "run-task4-stale"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != reviewscope.ChainResolutionStaleDecision || len(result.Stale) != 1 || len(result.Contexts) != 0 {
		t.Fatalf("stale chain must block silent reuse: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(root, ".code-harness", "runs", "run-task4-stale", "analysis", "discovered-chains")); !os.IsNotExist(err) {
		t.Fatalf("stale chain must not rediscover before user decision, stat err=%v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("stale review lookup must not mutate Project State")
	}
}

func TestResolveChainContextsStaleCanUseTemporaryAfterExplicitChoice(t *testing.T) {
	root := t.TempDir()
	path := writeAcceptedChain(t, root, acceptedOrderChain())
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	selection := task4TargetedSelection(t, task4StaleAnalysis, "RiskService.check")
	const runID = "run-task4-stale-temp"

	result, err := reviewscope.ResolveChainContexts(root, selection, []byte(task4StaleAnalysis), task4CertifiedDiscoveryOptions(t, root, runID, true))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != reviewscope.ChainResolutionReady || len(result.Contexts) != 1 {
		t.Fatalf("explicit stale fallback should use current temporary chain: %+v", result)
	}
	if result.Contexts[0].Source != "DISCOVERED" || result.Contexts[0].Status != "TEMPORARY" {
		t.Fatalf("stale fallback must not masquerade as accepted: %+v", result.Contexts[0])
	}
	assertTask4DiscoveredYAMLAndCert(t, root, runID)
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("temporary stale fallback must not refresh Project State")
	}
}

func TestFullCoverageRemainsCompleteRuleEvenWithAcceptedContext(t *testing.T) {
	root := t.TempDir()
	writeAcceptedChain(t, root, acceptedOrderChain())
	selection, err := reviewscope.Verify([]byte(`{"mode":"FULL","selectedCallChains":[],"scopedFiles":[]}`), []byte(task4Analysis))
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := reviewscope.ResolveChainContexts(root, selection, []byte(task4Analysis), reviewscope.ChainResolveOptions{RunID: "run-task4-full"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved.Contexts) == 0 {
		t.Fatalf("expected accepted review context: %+v", resolved)
	}
	coverage := reviewscope.ComputeCoverage(selection, []string{"src/main/java/OrderController.java", "src/main/java/OrderService.java"})
	if coverage.Status != "PARTIAL" || len(coverage.MissingFiles) != 1 || coverage.MissingFiles[0] != "src/main/java/UnrelatedService.java" {
		t.Fatalf("chain context weakened FULL coverage: %+v", coverage)
	}
}
