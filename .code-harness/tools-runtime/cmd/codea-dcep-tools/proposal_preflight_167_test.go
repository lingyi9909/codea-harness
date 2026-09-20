package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/reviewprogress"
)

func Test167MalformedProposalDoesNotAdvanceOrFailRun(t *testing.T) {
	for _, kind := range []string{"analysis", "findings"} {
		t.Run(kind, func(t *testing.T) {
			withTempProject(t)
			id := "preflight167"
			requests := filepath.Join(".code-harness", "runs", id, "requests")
			if err := os.MkdirAll(requests, 0755); err != nil {
				t.Fatal(err)
			}
			if _, err := reviewprogress.Begin(".", id); err != nil {
				t.Fatal(err)
			}
			progressPath, _ := reviewprogress.Path(id)
			before, err := os.ReadFile(progressPath)
			if err != nil {
				t.Fatal(err)
			}
			var command, request string
			if kind == "analysis" {
				copyTask153CommandContract(t, ".", "change-analysis-proposal.schema.json")
				writeFile(t, filepath.Join(requests, "change-analysis-proposal.json"), `{}`)
				request = `{"runId":"preflight167","snapshotPath":".code-harness/runs/preflight167/analysis/change-set.json","snapshotSha256":"` + strings.Repeat("a", 64) + `","proposalPath":".code-harness/runs/preflight167/requests/change-analysis-proposal.json","intent":{"mode":"FULL"}}`
				command = "certify"
			} else {
				copyTask153CommandContract(t, ".", "finding-proposals.schema.json")
				writeFile(t, filepath.Join(requests, "finding-proposals.json"), `[{}]`)
				request = `{"runId":"preflight167","proposalsPath":".code-harness/runs/preflight167/requests/finding-proposals.json"}`
				command = "certify-findings"
			}
			input := filepath.ToSlash(filepath.Join(requests, "certify.json"))
			writeFile(t, input, request)
			group := "analysis"
			if kind == "findings" {
				group = "review"
			}
			err = run([]string{group, command, "--input", input})
			if err == nil || !strings.Contains(err.Error(), "PROPOSAL_PREFLIGHT_FAILED") {
				t.Fatalf("expected correctable preflight rejection, got %v", err)
			}
			after, err := os.ReadFile(progressPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatal("preflight mutated progress")
			}
		})
	}
}
