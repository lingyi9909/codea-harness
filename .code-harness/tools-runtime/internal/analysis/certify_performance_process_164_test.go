package analysis

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test164CertifyPerformanceProductionReportsActualPhaseProcesses(t *testing.T) {
	astPath := strings.TrimSpace(os.Getenv("CODEA_AST_GREP_TEST_PATH"))
	if astPath == "" {
		t.Skip("pinned ast-grep path not configured")
	}
	absAst, err := filepath.Abs(astPath)
	if err != nil {
		t.Fatal(err)
	}

	fixture := createTask164PerformanceFixture(t, 0, absAst)
	runID := "run-performance-processes"
	writeTask164CanonicalCertifyInputs(t, fixture.root, runID, fixture.snapshot)
	if _, err := Certify(fixture.root, CertifyRequest{
		RunID:          runID,
		SnapshotPath:   ".code-harness/runs/" + runID + "/analysis/change-set.json",
		SnapshotSHA256: fixture.snapshot.SnapshotSHA256,
		ProposalPath:   ".code-harness/runs/" + runID + "/requests/change-analysis-proposal.json",
		Intent:         Intent{Mode: "FULL"},
	}); err != nil {
		t.Fatalf("production canonical certify failed: %v", err)
	}

	data, err := os.ReadFile(task164PerformanceArtifactPath(fixture.root, runID))
	if err != nil {
		t.Fatal(err)
	}
	var perf certifyPerformance164
	if err := json.Unmarshal(data, &perf); err != nil {
		t.Fatal(err)
	}
	if perf.Processes.CurrentTypeAst != 1 || perf.Processes.CurrentMethodAst != 1 || perf.Processes.BaseTypeAst != 1 || perf.Processes.BaseMethodAst != 1 || perf.Processes.BaseCatFile != 1 {
		t.Fatalf("actual process phases not recorded: %+v", perf.Processes)
	}
	if perf.Counts.ProductionJavaCurrent != 4 || perf.Counts.ProductionJavaBase != 4 {
		t.Fatalf("production Java side counts=%+v want current=4 base=4", perf.Counts)
	}
	if perf.Counts.ControllerFilesCurrent != 2 || perf.Counts.ControllerFilesBase != 2 {
		t.Fatalf("controller file counts=%+v want current=2 base=2", perf.Counts)
	}
	if perf.TimingMS.EntrypointInventory <= 0 {
		t.Fatalf("entrypoint inventory timing must be observed: %+v", perf.TimingMS)
	}
	if perf.TimingMS.CurrentTypeAst < 0 || perf.TimingMS.CurrentMethodAst < 0 || perf.TimingMS.BaseTypeAst < 0 || perf.TimingMS.BaseMethodAst < 0 || perf.TimingMS.BaseSourceLoad < 0 {
		t.Fatalf("phase timings must be non-negative: %+v", perf.TimingMS)
	}
}
