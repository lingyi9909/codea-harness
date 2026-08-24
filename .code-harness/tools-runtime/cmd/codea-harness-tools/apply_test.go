package main

import (
	"strings"
	"testing"
)

func TestSealApplySubcommandIsWired(t *testing.T) {
	err := run([]string{"seal-apply"})
	if err == nil || !strings.Contains(err.Error(), "seal-apply requires --input") {
		t.Fatalf("seal-apply subcommand not wired: %v", err)
	}
}

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

func TestUsageListsSealAndApply(t *testing.T) {
	err := run(nil)
	if err == nil || !strings.Contains(err.Error(), "seal-apply|apply") {
		t.Fatalf("usage missing seal/apply: %v", err)
	}
}
