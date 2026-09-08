package analysis

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"codea-harness-tools/internal/changeset"
)

type task164Task2ScaleFixture struct {
	root     string
	snapshot changeset.Snapshot
}

type task164Task2CertifyResult struct {
	cert Certificate
	err  error
}

func Test164CertifyPerformanceWindowsScaleGate(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Task 2 performance matrix is Windows-only")
	}
	astPath := strings.TrimSpace(os.Getenv("CODEA_AST_GREP_TEST_PATH"))
	if astPath == "" {
		t.Skip("pinned ast-grep path not configured")
	}
	absAst, err := filepath.Abs(astPath)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name            string
		changedFiles    int
		productionJava  int
		controllers     int
		multiModule     bool
		target          time.Duration
		hard            time.Duration
	}{
		{name: "21", changedFiles: 21, productionJava: 18, controllers: 4, target: 5 * time.Second, hard: 15 * time.Second},
		{name: "50", changedFiles: 50, productionJava: 50, controllers: 10, multiModule: true, target: 10 * time.Second, hard: 20 * time.Second},
		{name: "100", changedFiles: 100, productionJava: 100, controllers: 20, target: 20 * time.Second, hard: 30 * time.Second},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := createTask164Task2ScaleFixture(t, tc.changedFiles, tc.productionJava, tc.controllers, tc.multiModule, absAst)
			runID := "run-task2-scale-" + tc.name

			inventory, metrics, err := buildEntrypointInventoryWithMetrics164(fixture.root, runID+"-proposal", fixture.snapshot, Intent{Mode: "FULL"})
			if err != nil {
				t.Fatalf("precompute proposal inventory: %v", err)
			}
			if metrics.AstGrepProcessCount > 4 || metrics.BaseGitBatchProcessCount > 1 || metrics.PerFileGitMergeBaseCount != 0 || metrics.PerFileGitShowCount != 0 || metrics.FullProjectBaseScanCount != 0 {
				t.Fatalf("forbidden process behavior in fixture %s: %+v", tc.name, metrics)
			}
			writeTask164Task2CanonicalInputs(t, fixture.root, runID, fixture.snapshot, inventory)

			started := time.Now()
			done := make(chan task164Task2CertifyResult, 1)
			go func() {
				cert, certifyErr := Certify(fixture.root, CertifyRequest{
					RunID:          runID,
					SnapshotPath:   ".code-harness/runs/" + runID + "/analysis/change-set.json",
					SnapshotSHA256: fixture.snapshot.SnapshotSHA256,
					ProposalPath:   ".code-harness/runs/" + runID + "/requests/change-analysis-proposal.json",
					Intent:         Intent{Mode: "FULL"},
				})
				done <- task164Task2CertifyResult{cert: cert, err: certifyErr}
			}()

			var result task164Task2CertifyResult
			select {
			case result = <-done:
			case <-time.After(tc.hard):
				t.Fatalf("TASK164_TASK2_FIXTURE_%s HARD_LIMIT_FAIL elapsed>%s", tc.name, tc.hard)
			}
			elapsed := time.Since(started)
			if result.err != nil {
				t.Fatalf("fixture %s analysis certify failed: %v", tc.name, result.err)
			}
			if elapsed > tc.target {
				t.Fatalf("TASK164_TASK2_FIXTURE_%s TARGET_FAIL elapsed=%s target<=%s", tc.name, elapsed, tc.target)
			}
			if result.cert.RunID != runID || result.cert.SnapshotSHA256 != fixture.snapshot.SnapshotSHA256 {
				t.Fatalf("fixture %s certificate identity mismatch: %+v", tc.name, result.cert)
			}

			perf := readTask164Task2Performance(t, fixture.root, runID)
			if perf.Status != certifyPerformanceComplete164 || perf.SnapshotSHA256 != fixture.snapshot.SnapshotSHA256 {
				t.Fatalf("fixture %s telemetry identity/status: %+v", tc.name, perf)
			}
			if perf.Counts.ChangedFiles != tc.changedFiles || perf.Counts.ProductionJavaCurrent != tc.productionJava || perf.Counts.ProductionJavaBase != tc.productionJava {
				t.Fatalf("fixture %s count mismatch: %+v", tc.name, perf.Counts)
			}
			if perf.Counts.ControllerFilesCurrent != tc.controllers || perf.Counts.ControllerFilesBase != tc.controllers {
				t.Fatalf("fixture %s controller counts: %+v want=%d", tc.name, perf.Counts, tc.controllers)
			}
			if perf.Processes.CurrentTypeAst != 1 || perf.Processes.CurrentMethodAst != 1 || perf.Processes.BaseTypeAst != 1 || perf.Processes.BaseMethodAst != 1 || perf.Processes.BaseCatFile != 1 {
				t.Fatalf("fixture %s process telemetry: %+v", tc.name, perf.Processes)
			}
			assertTask164Task2ObservedTimings(t, tc.name, perf.TimingMS)

			ratio := float64(perf.TimingMS.SnapshotFreshness) * 100 / float64(perf.TimingMS.Total)
			t.Logf("TASK164_TASK2_FIXTURE_%s PASS elapsed_ms=%d changed_files=%d production_java=%d controllers=%d total_ms=%d snapshotFreshness_ms=%d snapshotFreshness_pct=%.2f entrypointInventory_ms=%d processes=%+v", tc.name, elapsed.Milliseconds(), tc.changedFiles, tc.productionJava, tc.controllers, perf.TimingMS.Total, perf.TimingMS.SnapshotFreshness, ratio, perf.TimingMS.EntrypointInventory, perf.Processes)
		})
	}
}

func assertTask164Task2ObservedTimings(t *testing.T, fixture string, timing certifyPerformanceTiming164) {
	t.Helper()
	observed := map[string]int64{
		"snapshotFreshness": timing.SnapshotFreshness,
		"snapshotSchemaAndDecode": timing.SnapshotSchemaAndDecode,
		"proposalSchema": timing.ProposalSchema,
		"analysisAssembly": timing.AnalysisAssembly,
		"entrypointInventory": timing.EntrypointInventory,
		"currentTypeAst": timing.CurrentTypeAst,
		"currentMethodAst": timing.CurrentMethodAst,
		"baseSourceLoad": timing.BaseSourceLoad,
		"baseTypeAst": timing.BaseTypeAst,
		"baseMethodAst": timing.BaseMethodAst,
		"entrypointVerification": timing.EntrypointVerification,
		"evidenceValidation": timing.EvidenceValidation,
		"coverageValidation": timing.CoverageValidation,
		"publish": timing.Publish,
		"total": timing.Total,
	}
	for name, value := range observed {
		if value <= 0 {
			t.Fatalf("fixture %s timing %s=%d must be positively observed", fixture, name, value)
		}
	}
}

func createTask164Task2ScaleFixture(t *testing.T, changedFiles, productionJava, controllers int, multiModule bool, astPath string) task164Task2ScaleFixture {
	t.Helper()
	if productionJava > changedFiles || controllers > productionJava {
		t.Fatalf("invalid scale fixture changed=%d java=%d controllers=%d", changedFiles, productionJava, controllers)
	}
	root := t.TempDir()
	gitTask164(t, root, "init", "-b", "feature")
	gitTask164(t, root, "config", "user.email", "task164@example.invalid")
	gitTask164(t, root, "config", "user.name", "Task 164 Task 2")
	gitTask164(t, root, "config", "core.autocrlf", "false")
	if err := os.WriteFile(filepath.Join(root, ".git", "info", "exclude"), []byte(".code-harness/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	javaPaths := make([]string, 0, productionJava)
	for i := 0; i < productionJava; i++ {
		path := task164Task2JavaPath(i, controllers, multiModule)
		javaPaths = append(javaPaths, path)
		writeTask164Java(t, root, path, task164Task2JavaSource(i, controllers, false))
	}
	for i := productionJava; i < changedFiles; i++ {
		writeTask164Text(t, root, fmt.Sprintf("src/main/resources/perf-%03d.txt", i-productionJava), fmt.Sprintf("base-%d\n", i))
	}
	gitTask164(t, root, "add", ".")
	gitTask164(t, root, "commit", "-m", "task2 scale base")
	baseSHA := strings.TrimSpace(gitTask164(t, root, "rev-parse", "HEAD"))

	for i, path := range javaPaths {
		writeTask164Java(t, root, path, task164Task2JavaSource(i, controllers, true))
	}
	for i := productionJava; i < changedFiles; i++ {
		writeTask164Text(t, root, fmt.Sprintf("src/main/resources/perf-%03d.txt", i-productionJava), fmt.Sprintf("current-%d\n", i))
	}
	snapshot, err := changeset.Compute(root, baseSHA, true)
	if err != nil {
		t.Fatalf("compute scale snapshot: %v", err)
	}
	if len(snapshot.Files) != changedFiles {
		t.Fatalf("scale snapshot files=%d want=%d", len(snapshot.Files), changedFiles)
	}
	copyTask164File(t, astPath, filepath.Join(root, ".code-harness", "bin", "ast-grep.exe"), 0o755)
	return task164Task2ScaleFixture{root: root, snapshot: snapshot}
}

func task164Task2JavaPath(index, controllers int, multiModule bool) string {
	prefix := ""
	if multiModule {
		if index%2 == 0 {
			prefix = "module-a/"
		} else {
			prefix = "module-b/"
		}
	}
	if index < controllers {
		return fmt.Sprintf("%ssrc/main/java/perf/Controller%03d.java", prefix, index)
	}
	kind := []string{"Service", "ServiceImpl", "Mapper", "Dto"}[(index-controllers)%4]
	return fmt.Sprintf("%ssrc/main/java/perf/%s%03d.java", prefix, kind, index)
}

func task164Task2JavaSource(index, controllers int, current bool) string {
	version := "base"
	value := 1
	if current {
		version = "current"
		value = 2
	}
	if index < controllers {
		return fmt.Sprintf("package perf;\n@RestController\npublic class Controller%03d {\n    @GetMapping\n    public String endpoint() { return \"%s-%03d\"; }\n}\n", index, version, index)
	}
	kind := []string{"Service", "ServiceImpl", "Mapper", "Dto"}[(index-controllers)%4]
	return fmt.Sprintf("package perf;\npublic class %s%03d { public int value() { return %d; } }\n", kind, index, value)
}

func writeTask164Text(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeTask164Task2CanonicalInputs(t *testing.T, root, runID string, snapshot changeset.Snapshot, inventory EntrypointInventory) {
	t.Helper()
	for _, name := range []string{
		"change-set.schema.json",
		"change-analysis-proposal.schema.json",
		"change-analysis.schema.json",
		"entrypoint-inventory.schema.json",
		"change-analysis-cert.schema.json",
	} {
		copyAnalysisContract153(t, root, name)
	}
	versionPath := filepath.Join(root, ".code-harness", "VERSION")
	if err := os.MkdirAll(filepath.Dir(versionPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(versionPath, []byte("1.6.4-task2-test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	snapshotBytes, err := changeset.CanonicalBytes(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	snapshotPath := filepath.Join(root, ".code-harness", "runs", runID, "analysis", "change-set.json")
	if err := os.MkdirAll(filepath.Dir(snapshotPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(snapshotPath, snapshotBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	roles := make([]map[string]any, 0, len(snapshot.Files))
	reviewed := make([]map[string]any, 0, len(snapshot.Files))
	for _, file := range snapshot.Files {
		role := task164Task2Role(file.Path)
		roles = append(roles, map[string]any{"path": file.Path, "role": role})
		reviewed = append(reviewed, map[string]any{"path": file.Path, "role": role, "reason": "CHANGED"})
	}
	affected := make([]map[string]any, 0)
	chains := make([]map[string]any, 0)
	locations := make([]map[string]any, 0)
	for _, expected := range inventory.ExpectedEntrypoints {
		if expected.Disposition == DispositionRemoved {
			continue
		}
		controller := owner153(expected.Symbol)
		affected = append(affected, map[string]any{
			"controller": controller,
			"endpoints": []string{expected.Symbol},
			"impactType": "DIRECT_CHANGE",
			"sourceSymbols": []string{expected.Symbol},
		})
		chains = append(chains, map[string]any{"entryPoint": expected.Symbol, "chain": []string{expected.Symbol}})
		locations = append(locations, map[string]any{
			"symbol": expected.Symbol,
			"path": expected.Path,
			"role": "Controller",
			"source": "FIND_SYMBOL",
		})
	}
	proposal := map[string]any{
		"changedFileRoles": roles,
		"affectedControllers": affected,
		"callChains": chains,
		"symbolLocations": locations,
		"resourceRelations": []any{},
		"externalDependencies": []any{},
		"riskAreas": []any{},
		"reviewCoverage": map[string]any{
			"status": "COMPLETE",
			"reviewedFiles": reviewed,
			"unresolvedSymbols": []any{},
		},
	}
	proposalBytes, err := json.MarshalIndent(proposal, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	proposalPath := filepath.Join(root, ".code-harness", "runs", runID, "requests", "change-analysis-proposal.json")
	if err := os.MkdirAll(filepath.Dir(proposalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(proposalPath, append(proposalBytes, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func task164Task2Role(path string) string {
	switch {
	case strings.Contains(path, "Controller"):
		return "Controller"
	case strings.Contains(path, "ServiceImpl"):
		return "ServiceImpl"
	case strings.Contains(path, "Service"):
		return "Service"
	case strings.Contains(path, "Mapper"):
		return "Mapper"
	case strings.Contains(path, "Dto"):
		return "DTO"
	case strings.Contains(path, "/resources/"):
		return "Resource"
	default:
		return "Other"
	}
}

func readTask164Task2Performance(t *testing.T, root, runID string) certifyPerformance164 {
	t.Helper()
	data, err := os.ReadFile(task164PerformanceArtifactPath(root, runID))
	if err != nil {
		t.Fatal(err)
	}
	var perf certifyPerformance164
	if err := json.Unmarshal(data, &perf); err != nil {
		t.Fatal(err)
	}
	return perf
}
