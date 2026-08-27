package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/chain"
	"gopkg.in/yaml.v3"
)

const task3AcceptedYAML = `version: 1
id: order-approve
name: 订单审批
status: ACCEPTED
entryPoints:
  - symbol: OrderController.approve
    path: src/main/java/com/example/order/OrderController.java
nodes:
  - symbol: OrderService.approve
    path: src/main/java/com/example/order/OrderService.java
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
notes: 核心审批链路
`

func task3AnalysisJSON(includeRisk bool) string {
	chainNodes := `"OrderController.approve","OrderService.approve","OrderServiceImpl.approve","OrderMapper.updateStatus"`
	riskLocation := ""
	if includeRisk {
		chainNodes = `"OrderController.approve","OrderService.approve","OrderServiceImpl.approve","RiskService.check","OrderMapper.updateStatus"`
		riskLocation = `,{"symbol":"RiskService.check","path":"src/main/java/com/example/order/RiskService.java","role":"Service","source":"FIND_SYMBOL"}`
	}
	return fmt.Sprintf(`{
  "reviewScope":{"currentBranch":"develop","baseRef":"origin/develop","baseCommit":"a","mergeBase":"a","headCommit":"b","includeWorkingTree":true},
  "changedFiles":[{"path":"src/main/java/com/example/order/OrderServiceImpl.java","role":"Service","sources":["COMMITTED"]}],
  "affectedControllers":[{"controller":"OrderController","endpoints":["OrderController.approve"],"impactType":"AFFECTED_BY_CALL_CHAIN","sourceSymbols":["OrderServiceImpl.approve"]}],
  "callChains":[{"entryPoint":"OrderController.approve","chain":[%s]}],
  "symbolLocations":[
    {"symbol":"OrderController.approve","path":"src/main/java/com/example/order/OrderController.java","role":"Controller","source":"FIND_SYMBOL"},
    {"symbol":"OrderService.approve","path":"src/main/java/com/example/order/OrderService.java","role":"Service","source":"FIND_SYMBOL"},
    {"symbol":"OrderServiceImpl.approve","path":"src/main/java/com/example/order/OrderServiceImpl.java","role":"Service","source":"FIND_IMPLEMENTATIONS","from":"OrderService.approve"},
    {"symbol":"OrderMapper.updateStatus","path":"src/main/java/com/example/order/OrderMapper.java","role":"Mapper","source":"FIND_SYMBOL"}%s
  ],
  "resourceRelations":[{"path":"src/main/resources/mapper/OrderMapper.xml","role":"MapperXml","resource":"OrderMapper.xml#updateStatus","fromSymbol":"OrderMapper.updateStatus","fromKind":"METHOD","source":"MAPPER_STATEMENT","evidence":"exact mapper statement"}],
  "externalDependencies":[],
  "riskAreas":[],
  "reviewCoverage":{"status":"COMPLETE","reviewedFiles":[
    {"path":"src/main/java/com/example/order/OrderServiceImpl.java","role":"Service","reason":"CHANGED"},
    {"path":"src/main/java/com/example/order/OrderController.java","role":"Controller","reason":"CALL_CHAIN"},
    {"path":"src/main/java/com/example/order/OrderService.java","role":"Service","reason":"CALL_CHAIN"},
    {"path":"src/main/java/com/example/order/OrderMapper.java","role":"Mapper","reason":"CALL_CHAIN"}
  ],"unresolvedSymbols":[]}
}`, chainNodes, riskLocation)
}

func setupTask3CommandProject(t *testing.T, includeRisk bool) string {
	t.Helper()
	withTempProject(t)
	installChangeAnalysisSchema(t)
	writeFile(t, filepath.Join("src", "main", "resources", "mapper", "OrderMapper.xml"), "<mapper/>")
	writeFile(t, filepath.Join(".code-harness", "chains", "order-approve.yaml"), task3AcceptedYAML)
	runID := "run-task3"
	analysisPath := filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json")
	writeFile(t, analysisPath, task3AnalysisJSON(includeRisk))
	return analysisPath
}

func certifyTask3Candidate(t *testing.T, analysisPath, candidatePath, kind string) {
	t.Helper()
	_, cert, err := loadCertifiedAnalysis153(".", filepath.ToSlash(analysisPath))
	if err != nil { t.Fatal(err) }
	candidate, err := chain.Load(candidatePath)
	if err != nil { t.Fatal(err) }
	var candidateBytes []byte
	if strings.EqualFold(kind, "DISCOVERED") {
		candidateBytes, err = yaml.Marshal(candidate)
	} else {
		candidateBytes, err = chain.MarshalYAML(candidate)
	}
	if err != nil { t.Fatal(err) }
	if err := os.WriteFile(candidatePath, candidateBytes, 0o644); err != nil { t.Fatal(err) }
	if _, err := chain.CertifyCandidate(".", candidate, filepath.ToSlash(candidatePath), kind, cert); err != nil {
		t.Fatalf("certify Task 3 candidate: %v", err)
	}
}

func TestChainListShowAndValidateCommands(t *testing.T) {
	analysisPath := setupTask3CommandProject(t, false)
	prepareCommittedCertifiedAnalysisFixture153(t, "run-task3", analysisPath)
	if err := run([]string{"chain", "list"}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"chain", "show", "--target", "order-approve"}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"chain", "validate", "--id", "order-approve", "--change-analysis", analysisPath}); err != nil {
		t.Fatalf("valid accepted chain rejected: %v", err)
	}
}

func TestChainRefreshIsDiffFirstAndPersistUsesExpectedHash(t *testing.T) {
	analysisPath := setupTask3CommandProject(t, true)
	prepareCommittedCertifiedAnalysisFixture153(t, "run-task3", analysisPath)
	discovered := strings.Replace(task3AcceptedYAML, "status: ACCEPTED", "status: DISCOVERED", 1)
	discovered = strings.Replace(discovered,
		"  - symbol: OrderMapper.updateStatus\n    path: src/main/java/com/example/order/OrderMapper.java\n    role: MAPPER",
		"  - symbol: RiskService.check\n    path: src/main/java/com/example/order/RiskService.java\n    role: SERVICE\n  - symbol: OrderMapper.updateStatus\n    path: src/main/java/com/example/order/OrderMapper.java\n    role: MAPPER", 1)
	discoveredPath := filepath.Join(".code-harness", "runs", "run-task3", "analysis", "discovered-chains", "order-approve.yaml")
	writeFile(t, discoveredPath, discovered)
	certifyTask3Candidate(t, analysisPath, discoveredPath, "DISCOVERED")
	refreshRequest := writeQueryRequest(t, "run-task3", "chain-refresh.json", `{"runId":"run-task3","id":"order-approve","discoveredPath":".code-harness/runs/run-task3/analysis/discovered-chains/order-approve.yaml"}`)
	if err := run([]string{"chain", "refresh", "--input", refreshRequest}); err != nil {
		t.Fatal(err)
	}
	candidatePath := filepath.Join(".code-harness", "runs", "run-task3", "analysis", "refresh-candidates", "order-approve.yaml")
	if _, err := os.Stat(candidatePath); err != nil {
		t.Fatalf("refresh candidate missing: %v", err)
	}
	if _, err := os.Stat(strings.TrimSuffix(candidatePath, ".yaml") + ".cert.json"); err != nil {
		t.Fatalf("refresh candidate provenance missing: %v", err)
	}
	existingPath := filepath.Join(".code-harness", "chains", "order-approve.yaml")
	before, err := os.ReadFile(existingPath)
	if err != nil { t.Fatal(err) }
	sum := sha256.Sum256(before)
	expectedHash := fmt.Sprintf("%x", sum[:])

	badRequest := writeQueryRequest(t, "run-task3", "chain-seal-bad.json", `{"runId":"run-task3","candidatePath":".code-harness/runs/run-task3/analysis/refresh-candidates/order-approve.yaml","expectedExistingHash":"deadbeef"}`)
	if err := run([]string{"chain", "seal-persist", "--input", badRequest}); err == nil {
		t.Fatal("seal-persist must reject stale expected hash")
	}
	afterBad, _ := os.ReadFile(existingPath)
	if string(afterBad) != string(before) {
		t.Fatal("failed seal changed Project State")
	}

	goodSeal := writeQueryRequest(t, "run-task3", "chain-seal-good.json", fmt.Sprintf(`{"runId":"run-task3","candidatePath":".code-harness/runs/run-task3/analysis/refresh-candidates/order-approve.yaml","expectedExistingHash":"%s"}`, expectedHash))
	if err := run([]string{"chain", "seal-persist", "--input", goodSeal}); err != nil {
		t.Fatalf("seal-persist failed: %v", err)
	}
	planDir := filepath.Join(".code-harness", "runs", "run-task3", "analysis", "chain-write-plans")
	entries, err := os.ReadDir(planDir)
	if err != nil || len(entries) != 1 { t.Fatalf("expected one write plan: entries=%v err=%v", entries, err) }
	planID := strings.TrimSuffix(entries[0].Name(), ".json")
	goodRequest := writeQueryRequest(t, "run-task3", "chain-persist-good.json", fmt.Sprintf(`{"runId":"run-task3","planId":"%s"}`, planID))
	if err := run([]string{"chain", "persist", "--input", goodRequest}); err != nil {
		t.Fatalf("confirmed persist failed: %v", err)
	}
	after, err := os.ReadFile(existingPath)
	if err != nil { t.Fatal(err) }
	if !strings.Contains(string(after), "RiskService.check") {
		t.Fatalf("confirmed refresh was not persisted:\n%s", after)
	}
}

func TestChainPersistValidationFailureMakesZeroProjectStateWrites(t *testing.T) {
	analysisPath := setupTask3CommandProject(t, false)
	prepareCommittedCertifiedAnalysisFixture153(t, "run-task3", analysisPath)
	candidate := strings.Replace(task3AcceptedYAML, "OrderServiceImpl.approve", "MissingService.approve", 1)
	candidatePath := filepath.Join(".code-harness", "runs", "run-task3", "analysis", "refresh-candidates", "order-approve.yaml")
	writeFile(t, candidatePath, candidate)
	certifyTask3Candidate(t, analysisPath, candidatePath, "REFRESH")
	existingPath := filepath.Join(".code-harness", "chains", "order-approve.yaml")
	before, _ := os.ReadFile(existingPath)
	requestPath := writeQueryRequest(t, "run-task3", "chain-seal-invalid.json", `{"runId":"run-task3","candidatePath":".code-harness/runs/run-task3/analysis/refresh-candidates/order-approve.yaml"}`)
	if err := run([]string{"chain", "seal-persist", "--input", requestPath}); err == nil {
		t.Fatal("unverified candidate must not seal")
	}
	after, _ := os.ReadFile(existingPath)
	if string(after) != string(before) {
		t.Fatal("validation failure modified Project State")
	}
}
