package analysis

import (
	"fmt"
	"testing"

	"codea-harness-tools/internal/changeset"
)

type task164FreshnessFastPathRuntime struct {
	snapshot       changeset.Snapshot
	inventory      EntrypointInventory
	verifyErr      error
	computeCalls   int
	verifyCalls    int
	inventoryCalls int
	verified       changeset.Snapshot
}

func (r *task164FreshnessFastPathRuntime) Compute(_ string, _ string, _ bool) (changeset.Snapshot, error) {
	r.computeCalls++
	return r.snapshot, nil
}

func (r *task164FreshnessFastPathRuntime) VerifyFreshness(_ string, snapshot changeset.Snapshot) error {
	r.verifyCalls++
	r.verified = snapshot
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
