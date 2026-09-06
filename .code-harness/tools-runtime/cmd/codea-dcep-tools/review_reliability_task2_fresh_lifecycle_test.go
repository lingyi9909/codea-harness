package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test162ReviewReliabilityTask2BeginCreatesFreshRunWithoutSnapshot(t *testing.T) {
	withTempProject(t)

	if err := run([]string{"review", "begin"}); err != nil {
		t.Fatalf("first review begin failed: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(".code-harness", "runs"))
	if err != nil {
		t.Fatalf("read runs after first begin: %v", err)
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		t.Fatalf("first review begin must create exactly one run directory, entries=%v", entries)
	}
	firstRunID := entries[0].Name()
	if !strings.HasPrefix(firstRunID, "review-") || strings.TrimSpace(firstRunID) != firstRunID {
		t.Fatalf("unexpected first runId %q", firstRunID)
	}
	if _, err := os.Stat(filepath.Join(".code-harness", "runs", firstRunID, "analysis", "change-set.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("review begin must not create Canonical Snapshot, err=%v", err)
	}

	if err := run([]string{"review", "begin"}); err != nil {
		t.Fatalf("second review begin failed: %v", err)
	}
	entries, err = os.ReadDir(filepath.Join(".code-harness", "runs"))
	if err != nil {
		t.Fatalf("read runs after second begin: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("two review begin invocations must create two distinct runs, count=%d", len(entries))
	}
	secondRunID := ""
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != firstRunID {
			secondRunID = entry.Name()
		}
	}
	if secondRunID == "" || secondRunID == firstRunID {
		t.Fatalf("second runId must be fresh, first=%q second=%q", firstRunID, secondRunID)
	}
	if _, err := os.Stat(filepath.Join(".code-harness", "runs", secondRunID, "analysis", "change-set.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("second review begin must not create Canonical Snapshot, err=%v", err)
	}
}

func Test162ReviewReliabilityTask2BeginRejectsArguments(t *testing.T) {
	withTempProject(t)
	err := run([]string{"review", "begin", "unexpected"})
	if err == nil {
		t.Fatal("review begin must reject positional arguments")
	}
}
