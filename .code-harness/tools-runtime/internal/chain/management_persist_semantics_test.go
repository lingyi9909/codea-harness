package chain

import "testing"

func TestValidateDiscoveredFactMismatchIsInvalidNotStale(t *testing.T) {
	root := t.TempDir()
	writeTask3Resource(t, root)
	c := task3Chain()
	c.Status = StatusDiscovered
	c.Nodes[1].Path = "src/main/java/com/example/order/RenamedServiceImpl.java"
	got := Validate(root, c, EvidenceSnapshot(task3Evidence()))
	if got.Status != ValidationInvalid {
		t.Fatalf("DISCOVERED fact mismatch must be INVALID, got %+v", got)
	}
}
