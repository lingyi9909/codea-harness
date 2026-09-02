package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	analysisruntime "codea-harness-tools/internal/analysis"
)

func Test162HotfixCanonicalCertificateCannotDowngradeToLegacyAuthority(t *testing.T) {
	root, runID, snapshot := new162CanonicalCertFixture(t)
	withChdir153Cmd(t, root, func() {
		write162CanonicalCertRequest(t, root, runID, snapshot)
		if err := run([]string{"analysis", "certify", "--input", ".code-harness/runs/" + runID + "/requests/analysis-certify.json"}); err != nil {
			t.Fatalf("canonical analysis certify failed: %v", err)
		}
	})

	analysisPath := filepath.Join(root, ".code-harness", "runs", runID, "analysis", "change-analysis.json")
	analysisBytes, err := os.ReadFile(analysisPath)
	if err != nil { t.Fatal(err) }
	var doc map[string]any
	if err := json.Unmarshal(analysisBytes, &doc); err != nil { t.Fatal(err) }
	scope, ok := doc["reviewScope"].(map[string]any)
	if !ok { t.Fatal("reviewScope missing") }
	scope["currentBranch"] = "forged-branch"
	scope["baseCommit"] = strings.Repeat("b", 40)
	scope["mergeBase"] = strings.Repeat("c", 40)
	forgedAnalysis, err := json.MarshalIndent(doc, "", "  ")
	if err != nil { t.Fatal(err) }
	forgedAnalysis = append(forgedAnalysis, '\n')
	if err := os.WriteFile(analysisPath, forgedAnalysis, 0o644); err != nil { t.Fatal(err) }

	certPath := filepath.Join(root, ".code-harness", "runs", runID, "analysis", "change-analysis.cert.json")
	certBytes, err := os.ReadFile(certPath)
	if err != nil { t.Fatal(err) }
	var cert analysisruntime.Certificate
	if err := json.Unmarshal(certBytes, &cert); err != nil { t.Fatal(err) }
	h := sha256.Sum256(forgedAnalysis)
	cert.AnalysisSHA256 = fmt.Sprintf("%x", h)
	// Simulate an attacker trying to turn a canonical certificate into the weaker
	// legacy authority mode while keeping all legacy hashes internally consistent.
	cert.ResolvedBaseCommit = ""
	cert.MergeBase = ""
	cert.CurrentBranch = ""
	cert.SnapshotSHA256 = ""
	forgedCert, err := json.MarshalIndent(cert, "", "  ")
	if err != nil { t.Fatal(err) }
	forgedCert = append(forgedCert, '\n')
	if err := os.WriteFile(certPath, forgedCert, 0o644); err != nil { t.Fatal(err) }

	_, _, err = analysisruntime.LoadCertified(root, filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json")))
	if err == nil || (!strings.Contains(err.Error(), "CERTIFICATE_AUTHORITY_MODE_DOWNGRADE") && !strings.Contains(err.Error(), "CERTIFICATE_SCHEMA_INVALID")) {
		t.Fatalf("canonical certificate authority downgrade must fail closed, got %v", err)
	}
}
