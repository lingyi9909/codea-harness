package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/chain"
)

func recertifyTask153AnalysisForEdit(t *testing.T, runID string, intent analysisruntime.Intent) {
	t.Helper()
	analysisPath := filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json")
	analysisBytes, err := os.ReadFile(analysisPath)
	if err != nil { t.Fatal(err) }
	var meta struct {
		ReviewScope struct {
			BaseRef            string `json:"baseRef"`
			IncludeWorkingTree bool   `json:"includeWorkingTree"`
		} `json:"reviewScope"`
	}
	if err := json.Unmarshal(analysisBytes, &meta); err != nil { t.Fatal(err) }
	if strings.TrimSpace(meta.ReviewScope.BaseRef) == "" {
		t.Fatal("certified analysis fixture missing reviewScope.baseRef")
	}
	draftPath := filepath.Join(".code-harness", "runs", runID, "requests", "change-analysis-draft.json")
	if err := os.WriteFile(draftPath, analysisBytes, 0o644); err != nil { t.Fatal(err) }
	req := analysisruntime.CertifyRequest{
		RunID:              runID,
		DraftPath:          filepath.ToSlash(draftPath),
		BaseRef:            meta.ReviewScope.BaseRef,
		IncludeWorkingTree: meta.ReviewScope.IncludeWorkingTree,
		Intent:             intent,
	}
	reqBytes, err := json.Marshal(req)
	if err != nil { t.Fatal(err) }
	requestPath := writeQueryRequest(t, runID, "analysis-certify-edit.json", string(reqBytes))
	if err := run([]string{"analysis", "certify", "--input", requestPath}); err != nil {
		t.Fatalf("real analysis certify for edit fixture failed: %v", err)
	}
}

func rerunTask153DiscoveryAfterCertify(t *testing.T, runID string) {
	t.Helper()
	requestPath := filepath.Join(".code-harness", "runs", runID, "requests", "chain-discover.json")
	if err := run([]string{"chain", "discover", "--input", requestPath}); err != nil {
		t.Fatalf("rediscover after real analysis certify: %v", err)
	}
}

func Test153EditRejectsCanonicalCertIntentEscalation(t *testing.T) {
	runID := "run-153-edit-intent-escalation"
	candidatePath, chainID := setupTask153AuthorityDiscovery(t, runID)
	discovered, err := chain.Load(candidatePath)
	if err != nil { t.Fatal(err) }
	if len(discovered.EntryPoints) == 0 || strings.TrimSpace(discovered.EntryPoints[0].Symbol) == "" {
		t.Fatal("discovered Chain missing exact EntryPoint")
	}
	entrypoint := discovered.EntryPoints[0].Symbol

	// Establish a genuine Runtime-certified FULL analysis first, then regenerate
	// the discovery candidate so all provenance is bound to that certification.
	recertifyTask153AnalysisForEdit(t, runID, analysisruntime.Intent{Mode: "FULL"})
	rerunTask153DiscoveryAfterCertify(t, runID)
	planID := sealTask153Candidate(t, runID, candidatePath)
	if err := persistTask153Plan(t, runID, planID); err != nil {
		t.Fatalf("persist FULL-certified discovery candidate: %v", err)
	}
	projectPath, err := chain.ChainPath(".", chainID)
	if err != nil { t.Fatal(err) }
	before, err := os.ReadFile(projectPath)
	if err != nil { t.Fatal(err) }

	// Attack: rewrite only cert.Intent to a schema-valid/canonical maintenance
	// intent. The Runtime must reject this privilege escalation independently of
	// JSON schema and canonical-byte checks.
	certPath := filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.cert.json")
	certBytes, err := os.ReadFile(certPath)
	if err != nil { t.Fatal(err) }
	var cert analysisruntime.Certificate
	if err := json.Unmarshal(certBytes, &cert); err != nil { t.Fatal(err) }
	cert.Intent = &analysisruntime.Intent{Mode: "CHAIN_MAINTENANCE", Target: entrypoint}
	canonical, err := json.MarshalIndent(cert, "", "  ")
	if err != nil { t.Fatal(err) }
	canonical = append(canonical, '\n')
	if err := os.WriteFile(certPath, canonical, 0o644); err != nil { t.Fatal(err) }

	requestPath := writeQueryRequest(t, runID, "chain-edit-escalated.json", `{
  "runId":"`+runID+`",
  "chainId":"`+chainID+`",
  "changeAnalysisPath":".code-harness/runs/`+runID+`/analysis/change-analysis.json",
  "operations":[{"type":"RENAME_CHAIN","name":"不应被提权后的证书授权"}]
}`)
	err = run([]string{"chain", "edit", "--input", requestPath})
	if err == nil {
		t.Fatal("schema-valid canonical cert.Intent escalation must fail closed")
	}
	if !strings.Contains(err.Error(), "CHAIN_EDIT_ANALYSIS_NOT_CERTIFIED") && !strings.Contains(err.Error(), "INTENT") {
		t.Fatalf("unexpected escalation rejection: %v", err)
	}
	if _, statErr := os.Stat(task153EditCandidatePath(runID, chainID)); !os.IsNotExist(statErr) {
		t.Fatalf("intent escalation must produce 0 edit candidates, stat=%v", statErr)
	}
	after, err := os.ReadFile(projectPath)
	if err != nil { t.Fatal(err) }
	if string(after) != string(before) {
		t.Fatal("intent escalation must produce 0 Project State writes")
	}
}
