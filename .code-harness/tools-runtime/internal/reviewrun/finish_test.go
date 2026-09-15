package reviewrun

import (
	"bytes"
	"context"
	"os"
	"testing"
)

func Test180FinishWithoutScopeKeepsReport(t *testing.T) {
	root := t.TempDir()
	started, err := Start(root)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(started.ReportPath)
	if err != nil {
		t.Fatal(err)
	}

	got, err := Finish(context.Background(), root, FinishRequest{RunID: started.RunID})
	if err == nil {
		t.Fatalf("expected finish rejection without prepared scope, got %+v", got)
	}
	after, readErr := os.ReadFile(started.ReportPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("failed finish must preserve existing report\nbefore:\n%s\nafter:\n%s", before, after)
	}
	status, statusErr := Status(root, started.RunID)
	if statusErr != nil {
		t.Fatal(statusErr)
	}
	if status.Execution != "INCOMPLETE" {
		t.Fatalf("failed finish must remain INCOMPLETE: %+v", status)
	}
}
