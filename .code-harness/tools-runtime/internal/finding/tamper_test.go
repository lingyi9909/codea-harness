package finding

import (
	"os"
	"path/filepath"
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
	if err := WriteCertifiedForTest(root, runID, authorities); err != nil {
		t.Fatalf("write Certified Findings fixture: %v", err)
	}
	return root, runID
}

func tamper160(t *testing.T, path string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil { t.Fatal(err) }
	if _, err := f.WriteString(" "); err != nil { _ = f.Close(); t.Fatal(err) }
	if err := f.Close(); err != nil { t.Fatal(err) }
}
