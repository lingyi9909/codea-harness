package reviewrun

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test180MultipleInterfaceImplementationPreventsAutoSingle(t *testing.T) {
	root := copyControllerReviewFixture180(t)
	useRealAstGrep180(t, root)

	second := filepath.Join(root, "src", "main", "java", "com", "example", "BackupOrderServiceImpl.java")
	content := `package com.example;

import java.io.Serializable;

public class BackupOrderServiceImpl implements Serializable, OrderService {
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
	if got.DiscoveryComplete || len(got.Chains) != 1 {
		t.Fatalf("multi-interface second implementation manufactured a complete single chain: %+v", got)
	}
	if !strings.Contains(strings.Join(got.Gaps, "\n"), "OrderService implementations=2") {
		t.Fatalf("missing ambiguous implementation gap: %+v", got.Gaps)
	}
	_, state, err := loadRun(root, started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if state.ScopeReady {
		t.Fatalf("ambiguous implementation created ready scope: %+v", state)
	}
}

func Test180ChangesServiceKeepsAffectedControllerEntrypoint(t *testing.T) {
	root := copyControllerReviewFixture180(t)
	useRealAstGrep180(t, root)
	initControllerReviewGitBaseline180(t, root)

	service := filepath.Join(root, "src", "main", "java", "com", "example", "OrderServiceImpl.java")
	f, err := os.OpenFile(service, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("\n// service implementation changed\n"); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	started, err := Start(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Prepare(context.Background(), root, started.RunID, Intent{Mode: "CHANGES", Target: "OrderController.create"})
	if err != nil {
		t.Fatal(err)
	}
	if !got.DiscoveryComplete || len(got.Chains) != 1 || got.Chains[0].Name != "OrderController.create" {
		t.Fatalf("service-only change lost the affected controller entrypoint: %+v", got)
	}
}

func Test180MapperXMLRequiresFullNamespaceIdentity(t *testing.T) {
	root := copyControllerReviewFixture180(t)
	useRealAstGrep180(t, root)

	xmlPath := filepath.Join(root, "src", "main", "resources", "mapper", "OrderMapper.xml")
	data, err := os.ReadFile(xmlPath)
	if err != nil {
		t.Fatal(err)
	}
	mutated := strings.Replace(string(data), `namespace="com.example.OrderMapper"`, `namespace="com.unrelated.OrderMapper"`, 1)
	if mutated == string(data) {
		t.Fatal("fixture mapper namespace replacement failed")
	}
	if err := os.WriteFile(xmlPath, []byte(mutated), 0o600); err != nil {
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
	if got.DiscoveryComplete {
		t.Fatalf("unrelated mapper namespace was accepted as a complete SQL chain: %+v", got)
	}
	if !strings.Contains(strings.Join(got.Gaps, "\n"), "OrderMapper.insertOrder XML statement unresolved") {
		t.Fatalf("missing mapper namespace identity gap: %+v", got.Gaps)
	}
}

func Test180PrepareCommitCannotOverwriteCancelledRun(t *testing.T) {
	root := t.TempDir()
	started, err := Start(root)
	if err != nil {
		t.Fatal(err)
	}
	runDir, stale, err := loadRun(root, started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Cancel(root, started.RunID, "cancel won before prepare commit"); err != nil {
		t.Fatal(err)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := persistPrepared180(runDir, stale, Intent{Mode: "CURRENT_IMPLEMENTATION"}, rootAbs, []string{}, "snapshot", []Chain{}, true, []string{}, 0); err == nil || !strings.Contains(err.Error(), "REVIEW_PREPARE_CANCELLED") {
		t.Fatalf("stale prepare commit did not preserve cancellation: %v", err)
	}
	if _, err := Finish(context.Background(), root, FinishRequest{RunID: started.RunID}); err == nil || !strings.Contains(err.Error(), "REVIEW_FINISH_CANCELLED") {
		t.Fatalf("finish escaped cancelled terminal state: %v", err)
	}
}
