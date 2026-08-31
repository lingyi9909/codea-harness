package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func installTask153ReviewContextAuthoritySchemas(t *testing.T) {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	contracts := filepath.Join(filepath.Dir(testFile), "..", "..", "..", "contracts")
	for _, name := range []string{"chain-candidate-cert.schema.json", "chain-write-plan.schema.json"} {
		data, err := os.ReadFile(filepath.Join(contracts, name))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(".code-harness", "contracts", name), string(data))
	}
}

func task153ReviewContextRequest(allowTemporaryForStale bool) string {
	allow := ""
	if allowTemporaryForStale {
		allow = ",\n\t  \"allowTemporaryForStale\":true"
	}
	return `{
	  "runId":"run-task4-review",
	  "changeAnalysisPath":".code-harness/runs/run-task4-review/analysis/change-analysis.json",
	  "reviewScope":{
	    "mode":"TARGETED",
	    "target":{"symbol":"OrderController.approve","kind":"METHOD"},
	    "selectedCallChains":[{"entryPoint":"OrderController.approve","chain":["OrderController.approve","OrderService.approve","OrderServiceImpl.approve","OrderMapper.updateStatus"]}],
	    "scopedFiles":[
	      "src/main/java/com/example/order/OrderController.java",
	      "src/main/java/com/example/order/OrderService.java",
	      "src/main/java/com/example/order/OrderServiceImpl.java",
	      "src/main/java/com/example/order/OrderMapper.java",
	      "src/main/resources/mapper/OrderMapper.xml"
	    ]
	  }` + allow + `
	}`
}

func task153ReviewContextCandidatePaths(t *testing.T) (string, string) {
	t.Helper()
	dir := filepath.Join(".code-harness", "runs", "run-task4-review", "analysis", "discovered-chains")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var yamlPath, certPath string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		switch {
		case strings.HasSuffix(entry.Name(), ".cert.json"):
			certPath = filepath.ToSlash(filepath.Join(dir, entry.Name()))
		case strings.EqualFold(filepath.Ext(entry.Name()), ".yaml"):
			yamlPath = filepath.ToSlash(filepath.Join(dir, entry.Name()))
		}
	}
	if yamlPath == "" || certPath == "" {
		t.Fatalf("lazy discovery must publish YAML + provenance cert, entries=%v", entries)
	}
	if certPath != strings.TrimSuffix(yamlPath, filepath.Ext(yamlPath))+".cert.json" {
		t.Fatalf("candidate/cert identity mismatch: yaml=%s cert=%s", yamlPath, certPath)
	}
	return yamlPath, certPath
}

func assertTask153ReviewContextCandidateCertificate(t *testing.T, analysisPath, candidatePath, certPath string) {
	t.Helper()
	_, analysisCert, err := loadCertifiedAnalysis153(".", analysisPath)
	if err != nil {
		t.Fatalf("load certified analysis: %v", err)
	}
	data, err := os.ReadFile(filepath.FromSlash(certPath))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`"runId": "run-task4-review"`,
		`"kind": "DISCOVERED"`,
		`"candidatePath": "` + candidatePath + `"`,
		`"analysisHash": "` + analysisCert.AnalysisSHA256 + `"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("candidate certificate missing %s: %s", want, text)
		}
	}
}

func Test153ChainReviewContextLazyDiscoveryPublishesProvenanceAndCanSeal(t *testing.T) {
	analysisPath := setupTask4ReviewContextProject(t)
	installTask153ReviewContextAuthoritySchemas(t)
	if err := os.Remove(filepath.Join(".code-harness", "chains", "order-approve.yaml")); err != nil {
		t.Fatal(err)
	}
	prepareCommittedCertifiedAnalysisFixture153(t, "run-task4-review", analysisPath)
	requestPath := writeQueryRequest(t, "run-task4-review", "chain-review-context-lazy-authority.json", task153ReviewContextRequest(false))
	if err := run([]string{"chain", "review-context", "--input", requestPath}); err != nil {
		t.Fatalf("lazy review-context discovery failed: %v", err)
	}

	candidatePath, certPath := task153ReviewContextCandidatePaths(t)
	assertTask153ReviewContextCandidateCertificate(t, analysisPath, candidatePath, certPath)
	planID := sealTask153Candidate(t, "run-task4-review", candidatePath)
	if !strings.HasPrefix(planID, "chain-write-") {
		t.Fatalf("lazy discovered candidate must seal to immutable planId, got %q", planID)
	}
}

func Test153ChainReviewContextStaleTemporaryDiscoveryPublishesProvenance(t *testing.T) {
	analysisPath := setupTask4ReviewContextProject(t)
	installTask153ReviewContextAuthoritySchemas(t)
	projectPath := filepath.Join(".code-harness", "chains", "order-approve.yaml")
	projectBytes, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	staleBytes := []byte(strings.Replace(string(projectBytes), "status: ACCEPTED", "status: STALE", 1))
	if string(staleBytes) == string(projectBytes) {
		t.Fatal("fixture did not contain ACCEPTED status")
	}
	if err := os.WriteFile(projectPath, staleBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	prepareCommittedCertifiedAnalysisFixture153(t, "run-task4-review", analysisPath)
	requestPath := writeQueryRequest(t, "run-task4-review", "chain-review-context-stale-authority.json", task153ReviewContextRequest(true))
	if err := run([]string{"chain", "review-context", "--input", requestPath}); err != nil {
		t.Fatalf("stale temporary review-context discovery failed: %v", err)
	}

	candidatePath, certPath := task153ReviewContextCandidatePaths(t)
	assertTask153ReviewContextCandidateCertificate(t, analysisPath, candidatePath, certPath)
}
