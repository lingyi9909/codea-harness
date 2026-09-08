package analysis

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"codea-harness-tools/internal/changeset"
)

type task164PerformanceContractRuntime struct {
	snapshot     changeset.Snapshot
	inventory    EntrypointInventory
	computeErr   error
	inventoryErr error
}

func (r task164PerformanceContractRuntime) Compute(_ string, _ string, _ bool) (changeset.Snapshot, error) {
	if r.computeErr != nil {
		return changeset.Snapshot{}, r.computeErr
	}
	return r.snapshot, nil
}

func (r task164PerformanceContractRuntime) Inventory(_ string, _ string, _ changeset.Snapshot, _ Intent) (EntrypointInventory, error) {
	if r.inventoryErr != nil {
		return EntrypointInventory{}, r.inventoryErr
	}
	return r.inventory, nil
}

func Test164CertifyPerformanceSuccessArtifactIsRuntimeOwnedAndNonAuthoritative(t *testing.T) {
	root, snapshot, req, runtime := createTask164CertifyPerformanceContractFixture(t, "run-performance-success")

	cert, err := certifyWithRuntime153(root, req, runtime)
	if err != nil {
		t.Fatalf("canonical certify failed: %v", err)
	}
	perfPath := task164PerformanceArtifactPath(root, req.RunID)
	perfBytes, err := os.ReadFile(perfPath)
	if err != nil {
		t.Fatalf("Runtime-owned performance artifact missing: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(perfBytes, &doc); err != nil {
		t.Fatalf("decode performance artifact: %v", err)
	}
	if got := int(doc["schemaVersion"].(float64)); got != 1 {
		t.Fatalf("schemaVersion=%d want=1", got)
	}
	if doc["runId"] != req.RunID || doc["snapshotSha256"] != snapshot.SnapshotSHA256 || doc["status"] != "COMPLETE" {
		t.Fatalf("unexpected performance identity/status: %v", doc)
	}
	allowed := map[string]bool{
		"schemaVersion": true, "runId": true, "snapshotSha256": true, "status": true,
		"errorCode": true, "counts": true, "timingMs": true, "processes": true,
	}
	for key := range doc {
		if !allowed[key] {
			t.Fatalf("unexpected performance field %q", key)
		}
	}
	if _, ok := doc["counts"].(map[string]any); !ok {
		t.Fatalf("counts missing or invalid: %T", doc["counts"])
	}
	if _, ok := doc["timingMs"].(map[string]any); !ok {
		t.Fatalf("timingMs missing or invalid: %T", doc["timingMs"])
	}
	if _, ok := doc["processes"].(map[string]any); !ok {
		t.Fatalf("processes missing or invalid: %T", doc["processes"])
	}

	requestType := reflect.TypeOf(CertifyRequest{})
	for i := 0; i < requestType.NumField(); i++ {
		field := requestType.Field(i)
		if strings.Contains(strings.ToLower(field.Name), "performance") || strings.Contains(strings.ToLower(field.Tag.Get("json")), "performance") {
			t.Fatalf("Agent-controlled performance input is forbidden: field=%s tag=%s", field.Name, field.Tag.Get("json"))
		}
	}

	certPath := filepath.Join(root, ".code-harness", "runs", req.RunID, "analysis", "change-analysis.cert.json")
	certBytesBefore, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(certBytesBefore), "certify-performance") {
		t.Fatalf("performance artifact must not participate in certificate payload")
	}
	if err := os.WriteFile(perfPath, []byte("{\"tampered\":true}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	certBytesAfter, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(certBytesBefore) != string(certBytesAfter) {
		t.Fatalf("telemetry mutation changed authoritative certificate bytes")
	}
	var persisted Certificate
	if err := json.Unmarshal(certBytesAfter, &persisted); err != nil {
		t.Fatal(err)
	}
	if cert.AnalysisSHA256 != persisted.AnalysisSHA256 || cert.EntrypointInventorySHA256 != persisted.EntrypointInventorySHA256 || cert.ChangeSetSHA256 != persisted.ChangeSetSHA256 || cert.SnapshotSHA256 != persisted.SnapshotSHA256 {
		t.Fatalf("telemetry mutation changed authoritative hashes: returned=%+v persisted=%+v", cert, persisted)
	}
}

func Test164CertifyPerformanceFailurePreservesFormalErrorAndWritesDiagnosticOnly(t *testing.T) {
	root, _, req, runtime := createTask164CertifyPerformanceContractFixture(t, "run-performance-failed")
	runtime.computeErr = fmt.Errorf("CHANGE_SET_SNAPSHOT_STALE: injected freshness failure")

	_, err := certifyWithRuntime153(root, req, runtime)
	if err == nil || !strings.Contains(err.Error(), "CHANGE_SET_SNAPSHOT_STALE") {
		t.Fatalf("formal fail-closed error must be preserved, got %v", err)
	}
	assertNoAuthoritativeAnalysis153(t, root, req.RunID)

	perfBytes, readErr := os.ReadFile(task164PerformanceArtifactPath(root, req.RunID))
	if readErr != nil {
		t.Fatalf("failure performance artifact missing: %v", readErr)
	}
	var doc map[string]any
	if jsonErr := json.Unmarshal(perfBytes, &doc); jsonErr != nil {
		t.Fatalf("decode failed performance artifact: %v", jsonErr)
	}
	if doc["status"] != "FAILED" || doc["errorCode"] != "CHANGE_SET_SNAPSHOT_STALE" {
		t.Fatalf("failure telemetry lost formal error code: %v", doc)
	}
	serialized := string(perfBytes)
	if strings.Contains(serialized, root) || strings.Contains(serialized, "return 2") {
		t.Fatalf("telemetry must not contain absolute path or source content: %s", serialized)
	}
}

func task164PerformanceArtifactPath(root, runID string) string {
	return filepath.Join(root, ".code-harness", "runs", runID, "analysis", "certify-performance.json")
}

func createTask164CertifyPerformanceContractFixture(t *testing.T, runID string) (string, changeset.Snapshot, CertifyRequest, task164PerformanceContractRuntime) {
	t.Helper()
	root := t.TempDir()
	gitTask164(t, root, "init", "-b", "feature")
	gitTask164(t, root, "config", "user.email", "task164@example.invalid")
	gitTask164(t, root, "config", "user.name", "Task 164 Performance")
	gitTask164(t, root, "config", "core.autocrlf", "false")
	infoExclude := filepath.Join(root, ".git", "info", "exclude")
	if err := os.WriteFile(infoExclude, []byte(".code-harness/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	javaPath := "src/main/java/acme/Utility.java"
	writeTask164Java(t, root, javaPath, "package acme; public class Utility { public int value() { return 1; } }\n")
	gitTask164(t, root, "add", javaPath)
	gitTask164(t, root, "commit", "-m", "base")
	baseSHA := strings.TrimSpace(gitTask164(t, root, "rev-parse", "HEAD"))
	writeTask164Java(t, root, javaPath, "package acme; public class Utility { public int value() { return 2; } }\n")

	snapshot, err := changeset.Compute(root, baseSHA, true)
	if err != nil {
		t.Fatalf("compute snapshot: %v", err)
	}
	if len(snapshot.Files) != 1 || snapshot.Files[0].Path != javaPath {
		t.Fatalf("unexpected fixture ChangeSet: %+v", snapshot.Files)
	}

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

	proposal := map[string]any{
		"changedFileRoles": []map[string]any{{"path": javaPath, "role": "Utility"}},
		"affectedControllers": []any{},
		"callChains": []any{},
		"symbolLocations": []any{},
		"resourceRelations": []any{},
		"externalDependencies": []any{},
		"riskAreas": []any{},
		"reviewCoverage": map[string]any{
			"status": "COMPLETE",
			"reviewedFiles": []map[string]any{{"path": javaPath, "role": "Utility", "reason": "CHANGED"}},
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

	req := CertifyRequest{
		RunID: runID,
		SnapshotPath: ".code-harness/runs/" + runID + "/analysis/change-set.json",
		SnapshotSHA256: snapshot.SnapshotSHA256,
		ProposalPath: ".code-harness/runs/" + runID + "/requests/change-analysis-proposal.json",
		Intent: Intent{Mode: "FULL"},
	}
	runtime := task164PerformanceContractRuntime{
		snapshot: snapshot,
		inventory: EntrypointInventory{RunID: runID, Status: inventoryComplete153, ChangeSetSHA256: snapshot.SHA256, ExpectedEntrypoints: []ExpectedEntrypoint{}},
	}
	return root, snapshot, req, runtime
}
