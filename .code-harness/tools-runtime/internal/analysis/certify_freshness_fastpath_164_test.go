package analysis

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/changeset"
)

type task164FreshnessFastPathRuntime struct {
	snapshot       changeset.Snapshot
	inventory      EntrypointInventory
	verifyErr      error
	realVerify     bool
	computeCalls   int
	verifyCalls    int
	inventoryCalls int
	verified       changeset.Snapshot
}

func (r *task164FreshnessFastPathRuntime) Compute(_ string, _ string, _ bool) (changeset.Snapshot, error) {
	r.computeCalls++
	return r.snapshot, nil
}

func (r *task164FreshnessFastPathRuntime) VerifyFreshness(root string, snapshot changeset.Snapshot) error {
	r.verifyCalls++
	r.verified = snapshot
	if r.realVerify {
		return changeset.VerifyFreshness(root, snapshot)
	}
	return r.verifyErr
}

func (r *task164FreshnessFastPathRuntime) Inventory(_ string, _ string, _ changeset.Snapshot, _ Intent) (EntrypointInventory, error) {
	r.inventoryCalls++
	return r.inventory, nil
}

func Test164CanonicalCertifyPrefersSnapshotFreshnessFastPath(t *testing.T) {
	root, snapshot, req, contract := createTask164CertifyPerformanceContractFixture(t, "run-task3-fastpath")
	runtime := &task164FreshnessFastPathRuntime{snapshot: snapshot, inventory: contract.inventory}

	cert, err := certifyWithRuntime153(root, req, runtime)
	if err != nil {
		t.Fatalf("canonical certify failed: %v", err)
	}
	if runtime.verifyCalls != 1 {
		t.Fatalf("fast freshness calls=%d want=1", runtime.verifyCalls)
	}
	if runtime.computeCalls != 0 {
		t.Fatalf("canonical certify rebuilt full snapshot %d times; fast path must avoid Compute", runtime.computeCalls)
	}
	if runtime.inventoryCalls != 1 {
		t.Fatalf("inventory calls=%d want=1", runtime.inventoryCalls)
	}
	if runtime.verified.SnapshotSHA256 != snapshot.SnapshotSHA256 || runtime.verified.GitStateSHA256 != snapshot.GitStateSHA256 {
		t.Fatalf("fast path verified different authority: got=%+v want snapshotSha=%s gitState=%s", runtime.verified, snapshot.SnapshotSHA256, snapshot.GitStateSHA256)
	}
	if cert.SnapshotSHA256 != snapshot.SnapshotSHA256 || cert.ChangeSetSHA256 != snapshot.SHA256 {
		t.Fatalf("certificate authority changed: %+v", cert)
	}
}

func Test164CanonicalCertifyFreshnessFastPathFailsClosedBeforeInventory(t *testing.T) {
	root, snapshot, req, contract := createTask164CertifyPerformanceContractFixture(t, "run-task3-fastpath-stale")
	runtime := &task164FreshnessFastPathRuntime{
		snapshot:  snapshot,
		inventory: contract.inventory,
		verifyErr: fmt.Errorf("CHANGE_SET_SNAPSHOT_STALE: injected task3 mutation"),
	}

	_, err := certifyWithRuntime153(root, req, runtime)
	if err == nil || err.Error() != "CHANGE_SET_SNAPSHOT_STALE: injected task3 mutation" {
		t.Fatalf("formal freshness failure must be preserved, got %v", err)
	}
	if runtime.verifyCalls != 1 || runtime.computeCalls != 0 {
		t.Fatalf("unexpected freshness execution verify=%d compute=%d", runtime.verifyCalls, runtime.computeCalls)
	}
	if runtime.inventoryCalls != 0 {
		t.Fatalf("stale snapshot must stop before inventory, calls=%d", runtime.inventoryCalls)
	}
	assertNoAuthoritativeAnalysis153(t, root, req.RunID)
}

func Test164CanonicalCertifyRehashedProjectionTamperFailsClosedBeforeInventory(t *testing.T) {
	root, snapshot, req, contract := createTask164CertifyPerformanceContractFixture(t, "run-task3-projection-stale")
	if len(snapshot.Files) == 0 {
		t.Fatal("fixture requires a canonical changed file")
	}
	tampered := snapshot
	tampered.Files = append([]changeset.File(nil), snapshot.Files...)
	tampered.Files[0].Status = "A"
	tampered = rehashTask164AnalysisSnapshot(t, tampered)
	if tampered.GitStateSHA256 != snapshot.GitStateSHA256 {
		t.Fatal("projection tamper must preserve GitStateSHA256")
	}
	if tampered.SnapshotSHA256 == snapshot.SnapshotSHA256 {
		t.Fatal("projection tamper must be rehashed to a distinct self-consistent snapshot")
	}

	snapshotBytes, err := changeset.CanonicalBytes(tampered)
	if err != nil {
		t.Fatalf("encode tampered snapshot: %v", err)
	}
	snapshotPath := filepath.Join(root, filepath.FromSlash(req.SnapshotPath))
	if err := os.WriteFile(snapshotPath, snapshotBytes, 0o644); err != nil {
		t.Fatalf("replace snapshot artifact: %v", err)
	}
	req.SnapshotSHA256 = tampered.SnapshotSHA256

	runtime := &task164FreshnessFastPathRuntime{
		snapshot:   tampered,
		inventory: contract.inventory,
		realVerify: true,
	}
	_, err = certifyWithRuntime153(root, req, runtime)
	if err == nil || !strings.Contains(err.Error(), "CHANGE_SET_SNAPSHOT_STALE") {
		t.Fatalf("self-consistent projection tamper must fail with stale authority, got %v", err)
	}
	if runtime.verifyCalls != 1 || runtime.computeCalls != 0 {
		t.Fatalf("projection tamper must use fast verifier only, verify=%d compute=%d", runtime.verifyCalls, runtime.computeCalls)
	}
	if runtime.inventoryCalls != 0 {
		t.Fatalf("projection tamper must stop before inventory, calls=%d", runtime.inventoryCalls)
	}
	assertNoAuthoritativeAnalysis153(t, root, req.RunID)
}

func rehashTask164AnalysisSnapshot(t *testing.T, snapshot changeset.Snapshot) changeset.Snapshot {
	t.Helper()
	identity := struct {
		ResolvedBaseCommit string           `json:"resolvedBaseCommit"`
		MergeBase          string           `json:"mergeBase"`
		HeadCommit         string           `json:"headCommit"`
		IncludeWorkingTree bool             `json:"includeWorkingTree"`
		Files              []changeset.File `json:"files"`
		GitStateSHA256     string           `json:"gitStateSha256"`
	}{
		ResolvedBaseCommit: snapshot.ResolvedBaseCommit,
		MergeBase:          snapshot.MergeBase,
		HeadCommit:         snapshot.HeadCommit,
		IncludeWorkingTree: snapshot.IncludeWorkingTree,
		Files:              snapshot.Files,
		GitStateSHA256:     snapshot.GitStateSHA256,
	}
	canonical, err := json.Marshal(identity)
	if err != nil {
		t.Fatalf("canonicalize tampered identity: %v", err)
	}
	snapshot.SnapshotSHA256 = fmt.Sprintf("%x", sha256.Sum256(canonical))
	snapshot.SHA256 = snapshot.SnapshotSHA256
	return snapshot
}
