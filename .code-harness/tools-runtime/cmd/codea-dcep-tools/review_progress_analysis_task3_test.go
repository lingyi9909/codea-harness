package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/reviewprogress"
)

func prepareTask3AnalysisCertify(t *testing.T, runID string) (string, string) {
	t.Helper()
	root := t.TempDir()
	git153Cmd(t, root, "init")
	git153Cmd(t, root, "config", "user.email", "task3@example.test")
	git153Cmd(t, root, "config", "user.name", "Task 3")
	yml := filepath.Join(root, "src", "main", "resources", "application.yml")
	mustWrite153Cmd(t, yml, "feature:\n  enabled: false\n")
	git153Cmd(t, root, "add", ".")
	git153Cmd(t, root, "commit", "-m", "base")
	head := git153Cmd(t, root, "rev-parse", "HEAD")
	mustWrite153Cmd(t, yml, "feature:\n  enabled: true\n")

	for _, name := range []string{
		"analysis-certify-request.schema.json",
		"change-set.schema.json",
		"change-analysis-proposal.schema.json",
		"change-analysis.schema.json",
		"entrypoint-inventory.schema.json",
		"change-analysis-cert.schema.json",
	} {
		copyTask153CommandContract(t, root, name)
	}
	mustWrite153Cmd(t, filepath.Join(root, ".code-harness", "VERSION"), "1.6.4\n")

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
			"status": "COMPLETE",
			"reviewedFiles": []map[string]any{{"path": "src/main/resources/application.yml", "role": "YamlConfig", "reason": "CHANGED"}},
			"unresolvedSymbols": []any{},
		},
	}
	draftBytes, err := json.MarshalIndent(draft, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	existingAnalysisRel := filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis-existing.json"))
	mustWrite153Cmd(t, filepath.Join(root, filepath.FromSlash(existingAnalysisRel)), string(append(draftBytes, '\n')))

	intent := analysisruntime.Intent{Mode: "CHAIN_MAINTENANCE", Target: "fixture-maintenance"}
	req := canonicalAnalysisCertifyRequestFromExistingTest(t, root, runID, existingAnalysisRel, "HEAD", true, intent)
	reqBytes, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	requestRel := filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "requests", "analysis-certify.json"))
	mustWrite153Cmd(t, filepath.Join(root, filepath.FromSlash(requestRel)), string(reqBytes))

	if _, err := reviewprogress.Begin(root, runID); err != nil {
		t.Fatalf("begin progress: %v", err)
	}
	snapshotRel := filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "analysis", "change-set.json"))
	if _, err := reviewprogress.Advance(root, runID, reviewprogress.StageSnapshot, snapshotRel); err != nil {
		t.Fatalf("advance fixture to change analysis: %v", err)
	}
	return root, requestRel
}

func Test164Task3AnalysisCertifyAdvancesChangeAnalysisAndCertification(t *testing.T) {
	const runID = "review-task3-cert-success"
	root, requestRel := prepareTask3AnalysisCertify(t, runID)
	withChdir153Cmd(t, root, func() {
		if err := run([]string{"analysis", "certify", "--input", requestRel}); err != nil {
			t.Fatalf("analysis certify failed: %v", err)
		}
		state, err := reviewprogress.Read(".", runID)
		if err != nil {
			t.Fatal(err)
		}
		if state.Stages[2].Status != reviewprogress.StatusSucceeded || state.Stages[3].Status != reviewprogress.StatusSucceeded || state.CurrentStage != reviewprogress.StageReviewPlanning || state.Stages[4].Status != reviewprogress.StatusRunning {
			t.Fatalf("analysis/certification progress not completed: %+v", state)
		}
		if len(state.Stages[2].Artifacts) != 2 || len(state.Stages[3].Artifacts) != 2 {
			t.Fatalf("analysis and certification stages must bind authority artifacts: %+v / %+v", state.Stages[2], state.Stages[3])
		}
	})
}

func Test164Task3ChangeAnalysisAuthorityFailureTerminatesAtChangeAnalysis(t *testing.T) {
	const runID = "review-task3-analysis-authority-fail"
	root, requestRel := prepareTask3AnalysisCertify(t, runID)
	authority := filepath.Join(root, ".code-harness", "runs", runID, "requests", "change-analysis-reviewer-authority.json")
	if err := os.Remove(authority); err != nil {
		t.Fatalf("remove Reviewer authority for failure injection: %v", err)
	}
	withChdir153Cmd(t, root, func() {
		if err := run([]string{"analysis", "certify", "--input", requestRel}); err == nil {
			t.Fatal("missing Reviewer authority must fail change analysis")
		}
		state, err := reviewprogress.Read(".", runID)
		if err != nil {
			t.Fatal(err)
		}
		if state.Status != reviewprogress.StatusFailed || state.FailureStage != reviewprogress.StageChangeAnalysis || state.Stages[3].Status != reviewprogress.StatusBlocked {
			t.Fatalf("change analysis failure attribution missing: %+v", state)
		}
	})
}

func Test164Task3CertificationStaleSnapshotFailureTerminatesAtCertification(t *testing.T) {
	const runID = "review-task3-cert-stale-fail"
	root, requestRel := prepareTask3AnalysisCertify(t, runID)
	mustWrite153Cmd(t, filepath.Join(root, "src", "main", "resources", "application.yml"), "feature:\n  enabled: changed-after-snapshot\n")
	withChdir153Cmd(t, root, func() {
		if err := run([]string{"analysis", "certify", "--input", requestRel}); err == nil {
			t.Fatal("stale snapshot must fail Runtime certification")
		}
		state, err := reviewprogress.Read(".", runID)
		if err != nil {
			t.Fatal(err)
		}
		if state.Stages[2].Status != reviewprogress.StatusSucceeded || state.Status != reviewprogress.StatusFailed || state.FailureStage != reviewprogress.StageCertification || state.Stages[4].Status != reviewprogress.StatusBlocked {
			t.Fatalf("certification failure attribution missing: %+v", state)
		}
	})
}
