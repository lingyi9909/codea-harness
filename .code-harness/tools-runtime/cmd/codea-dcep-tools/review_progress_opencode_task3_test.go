package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test164Task3OpenCodeRendersOnlyRuntimeProgressEvents(t *testing.T) {
	bootstrapPath := filepath.Clean(filepath.Join("..", "..", "..", "bootstrap.md"))
	data, err := os.ReadFile(bootstrapPath)
	if err != nil {
		t.Fatalf("read bootstrap: %v", err)
	}
	text := string(data)
	for _, required := range []string{
		"review progress --run-id <runId>",
		"events[].display",
		"Runtime-derived",
		"不得自行生成或宣告阶段 PASS/FAIL/RUNNING",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("bootstrap missing Task 3 OpenCode progress contract %q", required)
		}
	}
	for _, forbidden := range []string{
		"review progress advance",
		"review progress fail",
		"review progress complete",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("bootstrap exposes forbidden Agent progress mutation command %q", forbidden)
		}
	}
}
