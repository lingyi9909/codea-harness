package main

import (
	"strings"
	"testing"
)

func TestApplySubcommandIsWired(t *testing.T) {
	err := run([]string{"apply"})
	if err == nil || !strings.Contains(err.Error(), "apply requires --input") {
		t.Fatalf("apply subcommand not wired: %v", err)
	}
}

func TestApplyDoesNotAcceptPatchCLIArguments(t *testing.T) {
	err := run([]string{"apply", "--input", ".code-harness/runs/run-1/requests/apply.json", "--patch", "evil.diff"})
	if err == nil {
		t.Fatal("apply must reject positional/raw patch CLI arguments")
	}
}

func TestUsageListsApply(t *testing.T) {
	err := run(nil)
	if err == nil || !strings.Contains(err.Error(), "report|apply") {
		t.Fatalf("usage missing apply: %v", err)
	}
}
