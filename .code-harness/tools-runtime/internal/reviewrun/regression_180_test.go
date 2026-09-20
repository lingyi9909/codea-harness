package reviewrun

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type historicalEvidence180 struct {
	ID          string
	Evidence    string
	Disposition string
}

var historicalRegressionMatrix180 = []historicalEvidence180{
	{ID: "H01", Evidence: "Test180ReviewFinishIngressIsBOMCompatibleAndStrict; Test180StartWritesIncompleteReport", Disposition: "COVERED"},
	{ID: "H02", Evidence: "Test180PrimaryReviewInstructionsDoNotRequireLegacyReviewerAuthority", Disposition: "COVERED"},
	{ID: "H03", Evidence: "Test180PrimaryReviewFinishPathDoesNotUseOpaqueHostID; Test180SelectedSubsetIsExact", Disposition: "COVERED"},
	{ID: "H04", Evidence: "Test180FinishWithoutScopeKeepsReport; Test180ReviewFinishCommandRejectsRunIDMismatch", Disposition: "COVERED"},
	{ID: "H05", Evidence: "Test180ReviewStartCommandWritesIncompleteReport; Test180ReviewFinishCommandRejectsRunIDMismatch", Disposition: "COVERED"},
	{ID: "H06", Evidence: "Test180FinishPreparedStateWritesDurableReport; HostScenario REVIEW180_REAL_MODEL PASS", Disposition: "COVERED"},
	{ID: "H07", Evidence: "Test180NodesRejectLegacyParallelRefs; Test180ReviewFinishIngressIsBOMCompatibleAndStrict/legacy_chainRefs", Disposition: "COVERED"},
	{ID: "H08", Evidence: "Test180PrepareTwoEndpointsRequiresSelection (15s fixture budget and NavigationProcesses=5)", Disposition: "COVERED"},
	{ID: "H09", Evidence: "Test180PrepareMissingAstGrepFailsClosed; formal package delivery/version remains Task 5", Disposition: "COVERED_WITH_TASK5_FINAL"},
	{ID: "H10", Evidence: "Test180WindowsOccupiedReportPreventsFalseCompleteAndRetryRepairs; REVIEW180_REPORT_IDENTITY_REGRESSION PASS", Disposition: "COVERED_WITH_TASK5_FINAL"},
	{ID: "H11", Evidence: "Test180SelectNeedsNextActualUser; Test180SelectedSubsetIsExact; HostScenario REVIEW180_HOST_MULTI PASS actualUserReply=true", Disposition: "COVERED_WITH_TASK5_FINAL"},
	{ID: "H12", Evidence: "Task 5 required: formal 1.8.0 package and real 1.6.7→1.8.0 transactional upgrade", Disposition: "TASK5_REQUIRED"},
	{ID: "H13", Evidence: "Test180CurrentWorkflowDoesNotRestoreTask153; Test180HistoricalCompatibilityAssetsRemain", Disposition: "COVERED"},
	{ID: "H14", Evidence: "Test180SmokeDriversUseNativeCommandPathAndDoNotScriptFinish; HostScenario REVIEW180_REAL_MODEL PASS", Disposition: "COVERED_WITH_TASK5_FINAL"},
	{ID: "H15", Evidence: "Test180ReportGoldens; Test180ConclusionMatrix; Test180ReportZeroOptionsAndCurrentImplementationMode", Disposition: "COVERED"},
	{ID: "H16", Evidence: "Test180ConcurrentStartsDoNotOverlap; Test180ConcurrentFinishSameRunSerializes; Test180ResultSavedReportFailureSameContentRetryRepairs; Test180CancelRejectsFinish", Disposition: "COVERED"},
	{ID: "H17", Evidence: "Test180HistoricalCompatibilityAssetsRemain; fresh go test -count=1 ./... and go vet ./...", Disposition: "COVERED"},
}

func Test180HistoricalRegressionMatrixIsExplicit(t *testing.T) {
	if len(historicalRegressionMatrix180) != 17 {
		t.Fatalf("history matrix entries=%d want 17", len(historicalRegressionMatrix180))
	}
	seen := map[string]bool{}
	for _, item := range historicalRegressionMatrix180 {
		if seen[item.ID] {
			t.Fatalf("duplicate history id %s", item.ID)
		}
		seen[item.ID] = true
		if strings.TrimSpace(item.Evidence) == "" {
			t.Fatalf("%s has no concrete testName/HostScenario evidence", item.ID)
		}
		switch item.Disposition {
		case "COVERED", "COVERED_WITH_TASK5_FINAL", "TASK5_REQUIRED":
		default:
			t.Fatalf("%s has non-auditable disposition %q", item.ID, item.Disposition)
		}
		if strings.Contains(strings.ToLower(item.Evidence), "considered") || strings.Contains(item.Evidence, "已考虑") {
			t.Fatalf("%s uses prose consideration instead of evidence: %s", item.ID, item.Evidence)
		}
	}
	for i := 1; i <= 17; i++ {
		id := "H" + twoDigits180(i)
		if !seen[id] {
			t.Fatalf("missing historical regression %s", id)
		}
	}
	if historicalRegressionMatrix180[11].ID != "H12" || historicalRegressionMatrix180[11].Disposition != "TASK5_REQUIRED" {
		t.Fatal("H12 release/upgrade evidence must remain explicitly deferred to Task 5")
	}
}

func Test180CurrentWorkflowDoesNotRestoreTask153(t *testing.T) {
	root := repoRootForRegression180(t)
	removed := filepath.Join(root, ".github", "workflows", "task153-chain-reliability.yml")
	if _, err := os.Stat(removed); !os.IsNotExist(err) {
		t.Fatalf("stale Task153 workflow must stay absent, stat err=%v", err)
	}
	active := filepath.Join(root, ".github", "workflows", "runtime-regression-windows-x64.yml")
	data, err := os.ReadFile(active)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "task153-chain-reliability.yml") {
		t.Fatalf("active Runtime Regression still depends on removed Task153 workflow:\n%s", data)
	}
}

func Test180HistoricalCompatibilityAssetsRemain(t *testing.T) {
	root := repoRootForRegression180(t)
	required := []string{
		".code-harness/tools-runtime/cmd/codea-dcep-tools/task153_chain_edit_test.go",
		".code-harness/tools-runtime/cmd/codea-dcep-tools/task153_artifact_authority_test.go",
		".code-harness/tools-runtime/internal/upgrade/release_167_test.go",
		".code-harness/tools-runtime/internal/upgrade/upgrade_test.go",
		".code-harness/skills/upgrade-harness/SKILL.md",
	}
	for _, rel := range required {
		if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil || info.IsDir() {
			t.Fatalf("historical compatibility asset missing %s: %v", rel, err)
		}
	}
}

func repoRootForRegression180(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func twoDigits180(value int) string {
	if value < 10 {
		return "0" + string(rune('0'+value))
	}
	return "1" + string(rune('0'+value-10))
}
