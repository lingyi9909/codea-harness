package finding

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCertifiedRejectsChangedAnalysisBytes(t *testing.T) {
	root, runID := writeCertifiedLoadFixture160(t)
	tamper160(t, filepath.Join(root, ".code-harness", "runs", runID, "analysis", "change-analysis.json"))
	if _, err := LoadCertified(root, runID); err == nil {
		t.Fatal("changed Certified ChangeAnalysis bytes must invalidate Certified Findings")
	}
}

func TestLoadCertifiedRejectsChangedReviewUnitBytes(t *testing.T) {
	root, runID := writeCertifiedLoadFixture160(t)
	tamper160(t, filepath.Join(root, ".code-harness", "runs", runID, "analysis", "review-units.json"))
	if _, err := LoadCertified(root, runID); err == nil {
		t.Fatal("changed ReviewUnit bytes must invalidate Certified Findings")
	}
}

func TestLoadCertifiedRejectsChangedRuleDispatchBytes(t *testing.T) {
	root, runID := writeCertifiedLoadFixture160(t)
	tamper160(t, filepath.Join(root, ".code-harness", "runs", runID, "analysis", "rule-dispatch.json"))
	if _, err := LoadCertified(root, runID); err == nil {
		t.Fatal("changed RuleDispatch bytes must invalidate Certified Findings")
	}
}

func TestLoadCertifiedRejectsChangedProposalBytes(t *testing.T) {
	root, runID := writeCertifiedLoadFixture160(t)
	tamper160(t, filepath.Join(root, ".code-harness", "runs", runID, "requests", "finding-proposals.json"))
	if _, err := LoadCertified(root, runID); err == nil {
		t.Fatal("changed Finding Proposal bytes must invalidate Certified Findings")
	}
}

func writeCertifiedLoadFixture160(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	runID := "run-task4-load"
	installFindingContracts160(t, root)
	analysisDir := filepath.Join(root, ".code-harness", "runs", runID, "analysis")
	requestsDir := filepath.Join(root, ".code-harness", "runs", runID, "requests")
	if err := os.MkdirAll(analysisDir, 0o755); err != nil { t.Fatal(err) }
	if err := os.MkdirAll(requestsDir, 0o755); err != nil { t.Fatal(err) }

	authorities := map[string][]byte{
		filepath.Join(analysisDir, "change-analysis.json"): []byte("{\"authority\":\"analysis\"}\n"),
		filepath.Join(analysisDir, "review-units.json"): []byte("{\"authority\":\"units\"}\n"),
		filepath.Join(analysisDir, "rule-dispatch.json"): []byte("{\"authority\":\"dispatch\"}\n"),
		filepath.Join(requestsDir, "finding-proposals.json"): []byte("[]\n"),
	}
	for p, data := range authorities {
		if err := os.WriteFile(p, data, 0o644); err != nil { t.Fatal(err) }
	}
	set := CertifiedSet{
		RunID: runID,
		HarnessVersion: "1.6.0",
		ChangeSetSHA256: strings.Repeat("a", 64),
		ChangeAnalysisSHA256: hashTest160(authorities[filepath.Join(analysisDir, "change-analysis.json")]),
		ReviewUnitsSHA256: hashTest160(authorities[filepath.Join(analysisDir, "review-units.json")]),
		RuleDispatchSHA256: hashTest160(authorities[filepath.Join(analysisDir, "rule-dispatch.json")]),
		FindingProposalsSHA256: hashTest160(authorities[filepath.Join(requestsDir, "finding-proposals.json")]),
		Findings: []CertifiedFinding{},
	}
	cert := Certificate{RunID: runID, ChangeSetSHA256: set.ChangeSetSHA256, ChangeAnalysisSHA256: set.ChangeAnalysisSHA256, ReviewUnitsSHA256: set.ReviewUnitsSHA256, RuleDispatchSHA256: set.RuleDispatchSHA256, FindingProposalsSHA256: set.FindingProposalsSHA256, Mode: "FULL"}
	if err := WriteCertified(root, set, cert); err != nil {
		t.Fatalf("write Certified Findings fixture: %v", err)
	}
	return root, runID
}

func installFindingContracts160(t *testing.T, root string) {
	t.Helper()
	for _, name := range []string{"certified-findings.schema.json", "certified-findings-cert.schema.json"} {
		source := filepath.Clean(filepath.Join("..", "..", "..", "contracts", name))
		data, err := os.ReadFile(source)
		if err != nil { t.Fatalf("read contract %s: %v", source, err) }
		destination := filepath.Join(root, ".code-harness", "contracts", name)
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil { t.Fatal(err) }
		if err := os.WriteFile(destination, data, 0o644); err != nil { t.Fatal(err) }
	}
}

func hashTest160(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

func tamper160(t *testing.T, path string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil { t.Fatal(err) }
	if _, err := f.WriteString(" "); err != nil { _ = f.Close(); t.Fatal(err) }
	if err := f.Close(); err != nil { t.Fatal(err) }
}
