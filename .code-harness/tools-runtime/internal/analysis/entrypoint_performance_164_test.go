package analysis

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"codea-harness-tools/internal/changeset"
)

type task164PerformanceFixture struct {
	root     string
	baseSHA  string
	snapshot changeset.Snapshot
}

type task164CertifyResult struct {
	cert Certificate
	err  error
}

func Test164EntrypointPerformanceWindowsGate(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Task 1 performance gate is Windows-only")
	}
	astPath := strings.TrimSpace(os.Getenv("CODEA_AST_GREP_TEST_PATH"))
	if astPath == "" {
		t.Skip("pinned ast-grep path not configured")
	}
	absAst, err := filepath.Abs(astPath)
	if err != nil {
		t.Fatal(err)
	}

	small := createTask164PerformanceFixture(t, 100, absAst)
	_, smallMetrics, err := buildEntrypointInventoryWithMetrics164(small.root, "run-small", small.snapshot, Intent{Mode: "FULL"})
	if err != nil {
		t.Fatalf("small repository inventory failed: %v", err)
	}
	assertTask164PerformanceMetrics(t, small.snapshot, smallMetrics)

	large := createTask164PerformanceFixture(t, 5000, absAst)
	writeTask164CanonicalCertifyInputs(t, large.root, "run-large", large.snapshot)

	started := time.Now()
	done := make(chan task164CertifyResult, 1)
	go func() {
		cert, certifyErr := Certify(large.root, CertifyRequest{
			RunID:          "run-large",
			SnapshotPath:   ".code-harness/runs/run-large/analysis/change-set.json",
			SnapshotSHA256: large.snapshot.SnapshotSHA256,
			ProposalPath:   ".code-harness/runs/run-large/requests/change-analysis-proposal.json",
			Intent:         Intent{Mode: "FULL"},
		})
		done <- task164CertifyResult{cert: cert, err: certifyErr}
	}()

	var result task164CertifyResult
	select {
	case result = <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("TASK164_P0_CERTIFY HARD_LIMIT_FAIL elapsed>15s")
	}
	elapsed := time.Since(started)
	if result.err != nil {
		t.Fatalf("analysis certify failed: %v", result.err)
	}
	if result.cert.RunID != "run-large" || result.cert.SnapshotSHA256 != large.snapshot.SnapshotSHA256 {
		t.Fatalf("unexpected certificate identity: %+v", result.cert)
	}
	if elapsed > 10*time.Second {
		t.Fatalf("TASK164_P0_CERTIFY TARGET_FAIL elapsed=%s target<=10s", elapsed)
	}

	largeInventory, largeMetrics, err := buildEntrypointInventoryWithMetrics164(large.root, "run-large-metrics", large.snapshot, Intent{Mode: "FULL"})
	if err != nil {
		t.Fatalf("large repository inventory metrics failed: %v", err)
	}
	assertTask164PerformanceMetrics(t, large.snapshot, largeMetrics)
	assertTask164RepositorySizeInvariance(t, smallMetrics, largeMetrics)

	if len(large.snapshot.Files) != 5 {
		t.Fatalf("canonical ChangeSet=%d want=5 files=%+v", len(large.snapshot.Files), large.snapshot.Files)
	}
	if len(largeInventory.ExpectedEntrypoints) < 3 {
		t.Fatalf("expected modified/added/deleted controller obligations, got %+v", largeInventory.ExpectedEntrypoints)
	}
	javaCount := countTask164JavaFiles(t, large.root)
	if javaCount < 5000 {
		t.Fatalf("large fixture Java files=%d want>=5000", javaCount)
	}

	t.Logf("TASK164_P0_CERTIFY PASS elapsed_ms=%d java_files=%d changed_files=%d", elapsed.Milliseconds(), javaCount, len(large.snapshot.Files))
	t.Logf("TASK164_PROCESS_COUNTS PASS astGrepProcessCount=%d baseGitBatchProcessCount=%d perFileGitMergeBase=%d perFileGitShow=%d fullProjectEntrypointBaseScan=%d", largeMetrics.AstGrepProcessCount, largeMetrics.BaseGitBatchProcessCount, largeMetrics.PerFileGitMergeBaseCount, largeMetrics.PerFileGitShowCount, largeMetrics.FullProjectBaseScanCount)
	t.Logf("TASK164_REPOSITORY_SIZE_INVARIANCE PASS small_noise=100 large_noise=5000 currentRequestedFiles=%v baseRequestedFiles=%v currentScannedFiles=%v baseScannedFiles=%v astGrepProcessCount=%d baseGitBatchProcessCount=%d", largeMetrics.CurrentRequestedFiles, largeMetrics.BaseRequestedFiles, largeMetrics.CurrentScannedFiles, largeMetrics.BaseScannedFiles, largeMetrics.AstGrepProcessCount, largeMetrics.BaseGitBatchProcessCount)
}

func createTask164PerformanceFixture(t *testing.T, noiseFiles int, astPath string) task164PerformanceFixture {
	t.Helper()
	root := t.TempDir()
	gitTask164(t, root, "init", "-b", "feature")
	gitTask164(t, root, "config", "user.email", "task164@example.invalid")
	gitTask164(t, root, "config", "user.name", "Task 164 Gate")
	gitTask164(t, root, "config", "core.autocrlf", "false")
	infoExclude := filepath.Join(root, ".git", "info", "exclude")
	if err := os.WriteFile(infoExclude, []byte(".code-harness/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	writeTask164Java(t, root, "src/main/java/acme/AController.java", `package acme;
@RestController
public class AController {
    @GetMapping
    public String get() { return "base"; }
}
`)
	writeTask164Java(t, root, "src/main/java/acme/AService.java", `package acme;
public class AService {
    public int work() { return 1; }
}
`)
	writeTask164Java(t, root, "src/main/java/acme/DeletedController.java", `package acme;
@RestController
public class DeletedController {
    @DeleteMapping
    public String remove() { return "base"; }
}
`)
	writeTask164Java(t, root, "src/main/java/acme/PlainUtility.java", `package acme;
public class PlainUtility {
    public int value() { return 1; }
}
`)
	for i := 0; i < noiseFiles; i++ {
		path := fmt.Sprintf("src/main/java/noise/Noise%05d.java", i)
		writeTask164Java(t, root, path, fmt.Sprintf("package noise; public class Noise%05d { public int value() { return %d; } }\n", i, i))
	}
	gitTask164(t, root, "add", "src/main/java")
	gitTask164(t, root, "commit", "-m", "base fixture")
	baseSHA := strings.TrimSpace(gitTask164(t, root, "rev-parse", "HEAD"))

	writeTask164Java(t, root, "src/main/java/acme/AController.java", `package acme;
@RestController
public class AController {
    @GetMapping
    public String get() { return "current"; }
}
`)
	writeTask164Java(t, root, "src/main/java/acme/AService.java", `package acme;
public class AService {
    public int work() { return 2; }
}
`)
	writeTask164Java(t, root, "src/main/java/acme/PlainUtility.java", `package acme;
public class PlainUtility {
    public int value() { return 2; }
}
`)
	if err := os.Remove(filepath.Join(root, filepath.FromSlash("src/main/java/acme/DeletedController.java"))); err != nil {
		t.Fatal(err)
	}
	writeTask164Java(t, root, "src/main/java/acme/AddedController.java", `package acme;
@RestController
public class AddedController {
    @PostMapping
    public String add() { return "added"; }
}
`)

	snapshot, err := changeset.Compute(root, baseSHA, true)
	if err != nil {
		t.Fatalf("compute canonical snapshot: %v", err)
	}
	if len(snapshot.Files) != 5 {
		t.Fatalf("fixture canonical ChangeSet=%d want=5 files=%+v", len(snapshot.Files), snapshot.Files)
	}
	assertTask164SnapshotSides(t, snapshot)

	astDst := filepath.Join(root, ".code-harness", "bin", "ast-grep.exe")
	copyTask164File(t, astPath, astDst, 0o755)
	return task164PerformanceFixture{root: root, baseSHA: baseSHA, snapshot: snapshot}
}

func assertTask164SnapshotSides(t *testing.T, snapshot changeset.Snapshot) {
	t.Helper()
	status := map[string]string{}
	for _, file := range snapshot.Files {
		status[file.Path] = strings.ToUpper(strings.TrimSpace(file.Status))
	}
	want := map[string]string{
		"src/main/java/acme/AController.java":       "M",
		"src/main/java/acme/AService.java":          "M",
		"src/main/java/acme/DeletedController.java": "D",
		"src/main/java/acme/AddedController.java":   "A",
		"src/main/java/acme/PlainUtility.java":       "M",
	}
	if !reflect.DeepEqual(status, want) {
		t.Fatalf("snapshot statuses=%v want=%v", status, want)
	}
}

func assertTask164PerformanceMetrics(t *testing.T, snapshot changeset.Snapshot, metrics entrypointExecutionMetrics164) {
	t.Helper()
	wantCurrent := []string{
		"src/main/java/acme/AController.java",
		"src/main/java/acme/AService.java",
		"src/main/java/acme/AddedController.java",
		"src/main/java/acme/PlainUtility.java",
	}
	wantBase := []string{
		"src/main/java/acme/AController.java",
		"src/main/java/acme/AService.java",
		"src/main/java/acme/DeletedController.java",
		"src/main/java/acme/PlainUtility.java",
	}
	sort.Strings(wantCurrent)
	sort.Strings(wantBase)
	if !reflect.DeepEqual(metrics.CurrentRequestedFiles, wantCurrent) || !reflect.DeepEqual(metrics.CurrentScannedFiles, wantCurrent) {
		t.Fatalf("current scope drift requested=%v scanned=%v want=%v snapshot=%+v", metrics.CurrentRequestedFiles, metrics.CurrentScannedFiles, wantCurrent, snapshot.Files)
	}
	if !reflect.DeepEqual(metrics.BaseRequestedFiles, wantBase) || !reflect.DeepEqual(metrics.BaseScannedFiles, wantBase) {
		t.Fatalf("base scope drift requested=%v scanned=%v want=%v snapshot=%+v", metrics.BaseRequestedFiles, metrics.BaseScannedFiles, wantBase, snapshot.Files)
	}
	if metrics.AstGrepProcessCount > 4 {
		t.Fatalf("astGrepProcessCount=%d want<=4", metrics.AstGrepProcessCount)
	}
	if metrics.AstGrepProcessCount != 4 {
		t.Fatalf("fixture must exercise all four AST phases, got %d", metrics.AstGrepProcessCount)
	}
	if metrics.BaseGitBatchProcessCount != 1 {
		t.Fatalf("baseGitBatchProcessCount=%d want=1", metrics.BaseGitBatchProcessCount)
	}
	if metrics.PerFileGitMergeBaseCount != 0 || metrics.PerFileGitShowCount != 0 || metrics.FullProjectBaseScanCount != 0 {
		t.Fatalf("forbidden process/scope counts: %+v", metrics)
	}
}

func assertTask164RepositorySizeInvariance(t *testing.T, small, large entrypointExecutionMetrics164) {
	t.Helper()
	if !reflect.DeepEqual(small.CurrentRequestedFiles, large.CurrentRequestedFiles) {
		t.Fatalf("currentRequestedFiles varies with repository size: small=%v large=%v", small.CurrentRequestedFiles, large.CurrentRequestedFiles)
	}
	if !reflect.DeepEqual(small.BaseRequestedFiles, large.BaseRequestedFiles) {
		t.Fatalf("baseRequestedFiles varies with repository size: small=%v large=%v", small.BaseRequestedFiles, large.BaseRequestedFiles)
	}
	if !reflect.DeepEqual(small.CurrentScannedFiles, large.CurrentScannedFiles) {
		t.Fatalf("currentScannedFiles varies with repository size: small=%v large=%v", small.CurrentScannedFiles, large.CurrentScannedFiles)
	}
	if !reflect.DeepEqual(small.BaseScannedFiles, large.BaseScannedFiles) {
		t.Fatalf("baseScannedFiles varies with repository size: small=%v large=%v", small.BaseScannedFiles, large.BaseScannedFiles)
	}
	if small.AstGrepProcessCount != large.AstGrepProcessCount {
		t.Fatalf("astGrepProcessCount varies with repository size: small=%d large=%d", small.AstGrepProcessCount, large.AstGrepProcessCount)
	}
	if small.BaseGitBatchProcessCount != large.BaseGitBatchProcessCount {
		t.Fatalf("baseGitBatchProcessCount varies with repository size: small=%d large=%d", small.BaseGitBatchProcessCount, large.BaseGitBatchProcessCount)
	}
}

func writeTask164CanonicalCertifyInputs(t *testing.T, root, runID string, snapshot changeset.Snapshot) {
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
	if err := os.WriteFile(versionPath, []byte("1.6.4-task1-test\n"), 0o644); err != nil {
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
		role := task164RoleForPath(file.Path)
		roles = append(roles, map[string]any{"path": file.Path, "role": role})
		reviewed = append(reviewed, map[string]any{"path": file.Path, "role": role, "reason": "CHANGED"})
	}
	proposal := map[string]any{
		"changedFileRoles": roles,
		"affectedControllers": []map[string]any{
			{"controller": "AController", "endpoints": []string{"AController.get"}, "impactType": "DIRECT_CHANGE", "sourceSymbols": []string{"AController.get"}},
			{"controller": "AddedController", "endpoints": []string{"AddedController.add"}, "impactType": "DIRECT_CHANGE", "sourceSymbols": []string{"AddedController.add"}},
		},
		"callChains": []map[string]any{
			{"entryPoint": "AController.get", "chain": []string{"AController.get"}},
			{"entryPoint": "AddedController.add", "chain": []string{"AddedController.add"}},
		},
		"symbolLocations": []map[string]any{
			{"symbol": "AController.get", "path": "src/main/java/acme/AController.java", "role": "Controller", "source": "FIND_SYMBOL"},
			{"symbol": "AddedController.add", "path": "src/main/java/acme/AddedController.java", "role": "Controller", "source": "FIND_SYMBOL"},
		},
		"resourceRelations":    []any{},
		"externalDependencies": []any{},
		"riskAreas":             []any{},
		"reviewCoverage": map[string]any{
			"status":            "COMPLETE",
			"reviewedFiles":     reviewed,
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

func task164RoleForPath(path string) string {
	switch {
	case strings.Contains(path, "Controller.java"):
		return "Controller"
	case strings.HasSuffix(path, "AService.java"):
		return "Service"
	case strings.HasSuffix(path, "PlainUtility.java"):
		return "Utility"
	default:
		return "Other"
	}
}

func writeTask164Java(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitTask164(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func copyTask164File(t *testing.T, source, destination string, mode os.FileMode) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, data, mode); err != nil {
		t.Fatal(err)
	}
}

func countTask164JavaFiles(t *testing.T, root string) int {
	t.Helper()
	count := 0
	sourceRoot := filepath.Join(root, "src", "main", "java")
	if err := filepath.WalkDir(sourceRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".java") {
			count++
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return count
}
