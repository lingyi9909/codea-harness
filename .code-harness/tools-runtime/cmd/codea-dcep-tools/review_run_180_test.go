package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func Test180ReviewStartCommandWritesIncompleteReport(t *testing.T) {
	withTempProject(t)
	if err := run([]string{"review", "start"}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(".code-harness", "runs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		t.Fatalf("expected exactly one review run directory, got %+v", entries)
	}
	reportPath := filepath.Join(".code-harness", "runs", entries[0].Name(), "review.md")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("评审未完成")) {
		t.Fatalf("missing incomplete report marker: %s", data)
	}
	if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		t.Fatal("BOM")
	}
}
