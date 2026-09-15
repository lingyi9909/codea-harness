package reviewrun

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
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
	status, err := Status(root, got.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if status.Execution != "INCOMPLETE" || status.ReportPath != got.ReportPath {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func Test180StartFailsWhenReportUnwritable(t *testing.T) {
	root := t.TempDir()
	harnessRoot := filepath.Join(root, ".code-harness")
	if err := os.MkdirAll(harnessRoot, 0o755); err != nil {
		t.Fatal(err)
	}
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

func Test180ConcurrentStartsDoNotOverlap(t *testing.T) {
	root := t.TempDir()
	const count = 8
	outcomes := make(chan Outcome, count)
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := Start(root)
			if err != nil {
				errs <- err
				return
			}
			outcomes <- got
		}()
	}
	wg.Wait()
	close(outcomes)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for got := range outcomes {
		if seen[got.RunID] {
			t.Fatalf("duplicate run id %s", got.RunID)
		}
		seen[got.RunID] = true
		if _, err := os.Stat(got.ReportPath); err != nil {
			t.Fatal(err)
		}
	}
	if len(seen) != count {
		t.Fatalf("got %d runs, want %d", len(seen), count)
	}
}
