package main

import (
	analysisruntime "codea-harness-tools/internal/analysis"
	"encoding/json"
	"path/filepath"
	"testing"
)

func Test166PrimaryAnalysisCertifyPublishesCertifiedBundle(t *testing.T) {
	root := t.TempDir()
	git153Cmd(t, root, "init")
	git153Cmd(t, root, "config", "user.email", "task153@example.test")
	git153Cmd(t, root, "config", "user.name", "Task 153")
	yml := filepath.Join(root, "src", "main", "resources", "application.yml")
	mustWrite153Cmd(t, yml, "feature:\n  enabled: false\n")
	git153Cmd(t, root, "add", ".")
	git153Cmd(t, root, "commit", "-m", "base")
	head := git153Cmd(t, root, "rev-parse", "HEAD")
	mustWrite153Cmd(t, yml, "feature:\n  enabled: true\n")

	for _, name := range []string{
		"change-set.schema.json",
		"change-analysis-proposal.schema.json",
		"change-analysis.schema.json",
		"entrypoint-inventory.schema.json",
		"change-analysis-cert.schema.json",
	} {
		copyTask153CommandContract(t, root, name)
	}
	mustWrite153Cmd(t, filepath.Join(root, ".code-harness", "VERSION"), "1.6.6\n")

	draft := map[string]any{
		"reviewScope": map[string]any{
			"currentBranch": "master", "baseRef": "HEAD", "baseCommit": head,
			"mergeBase": head, "headCommit": head, "includeWorkingTree": true,
		},
		"changedFiles": []map[string]any{{
			"path": "src/main/resources/application.yml", "role": "YamlConfig", "sources": []string{"UNSTAGED"},
		}},
		"affectedControllers": []any{}, "callChains": []any{}, "symbolLocations": []any{}, "resourceRelations": []any{},
		"externalDependencies": []any{}, "riskAreas": []any{},
		"reviewCoverage": map[string]any{
			"status":            "COMPLETE",
			"reviewedFiles":     []map[string]any{{"path": "src/main/resources/application.yml", "role": "YamlConfig", "reason": "CHANGED"}},
			"unresolvedSymbols": []any{},
		},
	}
	draftBytes, err := json.MarshalIndent(draft, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	existingAnalysisRel := ".code-harness/runs/r153/analysis/change-analysis-existing.json"
	mustWrite153Cmd(t, filepath.Join(root, filepath.FromSlash(existingAnalysisRel)), string(append(draftBytes, '\n')))

	intent := analysisruntime.Intent{Mode: "CHAIN_MAINTENANCE", Target: "fixture-maintenance"}
	req := canonicalAnalysisCertifyRequestFromExistingTest(t, root, "r153", existingAnalysisRel, "HEAD", true, intent)
	writePrimaryReceipt166(t, root, "r153", "change-analysis", req.ProposalPath, "harness review", "")
	reqBytes, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	requestPath := filepath.Join(root, ".code-harness", "runs", "r153", "requests", "analysis-certify.json")
	mustWrite153Cmd(t, requestPath, string(reqBytes))

	withChdir153Cmd(t, root, func() {
		if err := run([]string{"analysis", "certify", "--input", ".code-harness/runs/r153/requests/analysis-certify.json"}); err != nil {
			t.Fatalf("analysis certify failed: %v", err)
		}
		analysisPath := ".code-harness/runs/r153/analysis/change-analysis.json"
		if _, cert, err := analysisruntime.LoadCertified(".", analysisPath); err != nil {
			t.Fatalf("published bundle must load as certified: %v", err)
		} else if cert.SemanticSessionID != "ses_test_reviewer_child_r153" {
			t.Fatalf("primary session not bound: %+v", cert)
		} else if cert.RunID != "r153" || cert.ChangeSetSHA256 == "" || cert.AnalysisSHA256 == "" || cert.EntrypointInventorySHA256 == "" {
			t.Fatalf("unexpected certificate: %+v", cert)
		} else if cert.Intent == nil || cert.Intent.Mode != "CHAIN_MAINTENANCE" || cert.Intent.Target != "fixture-maintenance" {
			t.Fatalf("certificate must bind certify intent, got %+v", cert.Intent)
		}
		if _, runID, err := loadVerifiedChainAnalysis(analysisPath); err != nil {
			t.Fatalf("Chain consumer must accept Runtime-certified ChangeAnalysis: %v", err)
		} else if runID != "r153" {
			t.Fatalf("Chain consumer certified runId=%q want r153", runID)
		}
	})
}
