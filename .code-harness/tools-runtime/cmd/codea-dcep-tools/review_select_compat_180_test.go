package main

import "testing"

func Test180ReviewSelectLegacyInputIsNotCapturedByNewRoute(t *testing.T) {
	for _, args := range [][]string{
		{"--input", ".code-harness/runs/r1/requests/select.json"},
		{"-input", ".code-harness/runs/r1/requests/select.json"},
		{"--input=.code-harness/runs/r1/requests/select.json"},
	} {
		if !usesLegacyReviewSelectInput180(args) {
			t.Fatalf("legacy select args were not recognized: %v", args)
		}
	}
	if usesLegacyReviewSelectInput180([]string{"--run-id", "r1", "--options-hash", "h", "--ids", "C1", "--session-id", "s", "--message-id", "m"}) {
		t.Fatal("1.8 select args must stay on the new route")
	}
}
