package reviewrun

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func Test180StartWritesIncompleteReport(t *testing.T) {
	root := t.TempDir()
	got, err := Start(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Execution != "INCOMPLETE" || got.RunID == "" {
		t.Fatalf("unexpected outcome: %+v", got)
	}
	data, err := os.ReadFile(got.ReportPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("评审未完成")) {
		t.Fatalf("missing incomplete marker: %s", data)
	}
	if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		t.Fatal("BOM")
	}
}

func Test180StartFailsWhenReportUnwritable(t *testing.T) {
	root := t.TempDir()
	harnessRoot := filepath.Join(root, ".code-harness")
	if err := os.MkdirAll(harnessRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	// A regular file at the runs path is a deterministic cross-platform
	// stand-in for an unwritable/conflicting report destination.
	if err := os.WriteFile(filepath.Join(harnessRoot, "runs"), []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Start(root)
	if err == nil {
		t.Fatalf("expected start failure, got outcome %+v", got)
	}
	if got.Execution == "COMPLETE" {
		t.Fatalf("start failure must never report COMPLETE: %+v", got)
	}
}
