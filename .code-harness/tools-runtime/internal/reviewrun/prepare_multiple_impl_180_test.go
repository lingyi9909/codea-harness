package reviewrun

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test180MultipleImplementationsPreventAutoSingle(t *testing.T) {
	root := copyControllerReviewFixture180(t)
	useRealAstGrep180(t, root)

	second := filepath.Join(root, "src", "main", "java", "com", "example", "BackupOrderServiceImpl.java")
	content := `package com.example;

public class BackupOrderServiceImpl implements OrderService {
    @Override
    public void create() {}

    @Override
    public void cancel() {}
}
`
	if err := os.WriteFile(second, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	started, err := Start(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Prepare(context.Background(), root, started.RunID, Intent{Mode: "CURRENT_IMPLEMENTATION", Target: "OrderController.create"})
	if err != nil {
		t.Fatal(err)
	}
	if got.DiscoveryComplete || got.SelectionRequired || len(got.Chains) != 1 {
		t.Fatalf("ambiguous implementation must not manufacture AUTO_SINGLE: %+v", got)
	}
	if !strings.Contains(strings.Join(got.Gaps, "\n"), "OrderService implementations=2") {
		t.Fatalf("missing non-unique implementation gap: %+v", got.Gaps)
	}
	_, state, err := loadRun(root, started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if state.ScopeReady || state.Coverage != "PARTIAL" {
		t.Fatalf("ambiguous implementation created ready scope: %+v", state)
	}
}
