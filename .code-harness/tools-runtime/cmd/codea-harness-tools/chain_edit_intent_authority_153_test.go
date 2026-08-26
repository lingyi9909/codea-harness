package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/chain"
)

func installTask153NoopAstGrep(t *testing.T) {
	t.Helper()
	binDir := filepath.Join(".code-harness", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil { t.Fatal(err) }
	sourcePath := filepath.Join(binDir, "ast-grep-stub.go")
	if err := os.WriteFile(sourcePath, []byte("package main\nfunc main() {}\n"), 0o644); err != nil { t.Fatal(err) }
	binaryPath := filepath.Join(binDir, "ast-grep.exe")
	cmd := exec.Command("go", "build", "-o", binaryPath, sourcePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build host ast-grep stub: %v: %s", err, strings.TrimSpace(string(out)))
	}
	if err := os.Remove(sourcePath); err != nil { t.Fatal(err) }
}

func recertifyTask153AnalysisForEdit(t *testing.T, runID string, intent analysisruntime.Intent) {
	t.Helper()
	installTask153NoopAstGrep(t)
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

func Test153EditRejectsJointWorkspaceAuthorityEscalation(t *testing.T) {
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

	// The edit attempt itself must create zero write plans. Remove the discovery
	// persist plan after it has already produced the accepted Project State Chain.
	plansDir := filepath.Join(".code-harness", "runs", runID, "analysis", "chain-write-plans")
	if err := os.RemoveAll(plansDir); err != nil { t.Fatal(err) }

	// Attack: jointly rewrite every workspace-owned authority artifact so their
	// internal hashes remain self-consistent and canonical.
	intent := &analysisruntime.Intent{Mode: "CHAIN_MAINTENANCE", Target: entrypoint}
	analysisDir := filepath.Join(".code-harness", "runs", runID, "analysis")
	inventoryPath := filepath.Join(analysisDir, "entrypoint-inventory.json")
	inventoryBytes, err := os.ReadFile(inventoryPath)
	if err != nil { t.Fatal(err) }
	var inventory analysisruntime.EntrypointInventory
	if err := json.Unmarshal(inventoryBytes, &inventory); err != nil { t.Fatal(err) }
	inventory.Intent = intent
	inventoryBytes, err = json.MarshalIndent(inventory, "", "  ")
	if err != nil { t.Fatal(err) }
	inventoryBytes = append(inventoryBytes, '\n')
	if err := os.WriteFile(inventoryPath, inventoryBytes, 0o644); err != nil { t.Fatal(err) }

	certPath := filepath.Join(analysisDir, "change-analysis.cert.json")
	certBytes, err := os.ReadFile(certPath)
	if err != nil { t.Fatal(err) }
	var cert analysisruntime.Certificate
	if err := json.Unmarshal(certBytes, &cert); err != nil { t.Fatal(err) }
	cert.Intent = intent
	cert.EntrypointInventorySHA256 = sha153Fixture(inventoryBytes)
	certBytes, err = json.MarshalIndent(cert, "", "  ")
	if err != nil { t.Fatal(err) }
	certBytes = append(certBytes, '\n')
	if err := os.WriteFile(certPath, certBytes, 0o644); err != nil { t.Fatal(err) }

	requestPath := writeQueryRequest(t, runID, "chain-edit-escalated.json", `{
  "runId":"`+runID+`",
  "chainId":"`+chainID+`",
  "changeAnalysisPath":".code-harness/runs/`+runID+`/analysis/change-analysis.json",
  "operations":[{"type":"RENAME_CHAIN","name":"不应被联合篡改后的证书授权"}]
}`)
	err = run([]string{"chain", "edit", "--input", requestPath})
	if err == nil {
		t.Fatal("joint canonical cert+inventory+hash escalation must fail closed")
	}
	if _, statErr := os.Stat(task153EditCandidatePath(runID, chainID)); !os.IsNotExist(statErr) {
		t.Fatalf("joint authority escalation must produce 0 edit candidates, stat=%v", statErr)
	}
	if entries, readErr := os.ReadDir(plansDir); readErr == nil && len(entries) != 0 {
		t.Fatalf("joint authority escalation must produce 0 chain write plans, got %d", len(entries))
	} else if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatal(readErr)
	}
	after, err := os.ReadFile(projectPath)
	if err != nil { t.Fatal(err) }
	if string(after) != string(before) {
		t.Fatal("joint authority escalation must produce 0 Project State writes")
	}
}
