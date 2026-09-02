package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test162HotfixAnalysisCertifyConsumesRuntimeSnapshotAndSemanticProposal(t *testing.T) {
	root, runID, snapshot := new162CanonicalCertFixture(t)
	withChdir153Cmd(t, root, func() {
		write162CanonicalCertRequest(t, root, runID, snapshot)
		if err := run([]string{"analysis", "certify", "--input", ".code-harness/runs/" + runID + "/requests/analysis-certify.json"}); err != nil {
			t.Fatalf("canonical analysis certify failed: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json"))
		if err != nil { t.Fatal(err) }
		var doc struct {
			ReviewScope struct {
				CurrentBranch      string `json:"currentBranch"`
				BaseRef            string `json:"baseRef"`
				BaseCommit         string `json:"baseCommit"`
				MergeBase          string `json:"mergeBase"`
				HeadCommit         string `json:"headCommit"`
				IncludeWorkingTree bool   `json:"includeWorkingTree"`
			} `json:"reviewScope"`
			ChangedFiles []struct {
				Path string `json:"path"`
				Role string `json:"role"`
				Sources []string `json:"sources"`
			} `json:"changedFiles"`
		}
		if err := json.Unmarshal(data, &doc); err != nil { t.Fatal(err) }
		if doc.ReviewScope.BaseRef != snapshot.RequestedBaseRef || doc.ReviewScope.BaseCommit != snapshot.ResolvedBaseCommit || doc.ReviewScope.MergeBase != snapshot.MergeBase || doc.ReviewScope.HeadCommit != snapshot.HeadCommit || doc.ReviewScope.CurrentBranch != snapshot.CurrentBranch || doc.ReviewScope.IncludeWorkingTree != snapshot.IncludeWorkingTree {
			t.Fatalf("Certified ChangeAnalysis did not copy Runtime Git identity: %+v snapshot=%+v", doc.ReviewScope, snapshot)
		}
		if len(doc.ChangedFiles) != 1 || doc.ChangedFiles[0].Path != "src/main/resources/application.yml" || doc.ChangedFiles[0].Role != "YamlConfig" || len(doc.ChangedFiles[0].Sources) != 1 || doc.ChangedFiles[0].Sources[0] != "UNSTAGED" {
			t.Fatalf("Certified changedFiles were not assembled from Runtime snapshot + semantic role: %+v", doc.ChangedFiles)
		}
	})
}

func Test162HotfixAnalysisCertifyRejectsStaleSnapshotAfterGitBytesChange(t *testing.T) {
	root, runID, snapshot := new162CanonicalCertFixture(t)
	mustWrite153Cmd(t, filepath.Join(root, "src", "main", "resources", "application.yml"), "feature: maybe\n")
	withChdir153Cmd(t, root, func() {
		write162CanonicalCertRequest(t, root, runID, snapshot)
		err := run([]string{"analysis", "certify", "--input", ".code-harness/runs/" + runID + "/requests/analysis-certify.json"})
		if err == nil || !strings.Contains(err.Error(), "CHANGE_SET_SNAPSHOT_STALE") {
			t.Fatalf("stale canonical snapshot must fail closed, got %v", err)
		}
		for _, name := range []string{"change-analysis.json", "entrypoint-inventory.json", "change-analysis.cert.json"} {
			if _, statErr := os.Stat(filepath.Join(".code-harness", "runs", runID, "analysis", name)); !os.IsNotExist(statErr) {
				t.Fatalf("stale snapshot published authoritative %s: %v", name, statErr)
			}
		}
	})
}

func new162CanonicalCertFixture(t *testing.T) (string, string, canonicalSnapshot162Test) {
	t.Helper()
	root := t.TempDir()
	git153Cmd(t, root, "init")
	git153Cmd(t, root, "config", "user.email", "task162@example.test")
	git153Cmd(t, root, "config", "user.name", "Task 162 Hotfix")
	configPath := filepath.Join(root, "src", "main", "resources", "application.yml")
	mustWrite153Cmd(t, configPath, "feature: false\n")
	git153Cmd(t, root, "add", ".")
	git153Cmd(t, root, "commit", "-m", "base")
	mustWrite153Cmd(t, configPath, "feature: true\n")

	for _, name := range []string{"change-set.schema.json", "change-analysis-proposal.schema.json", "change-analysis.schema.json", "entrypoint-inventory.schema.json", "change-analysis-cert.schema.json"} {
		copyTask153CommandContract(t, root, name)
	}
	mustWrite153Cmd(t, filepath.Join(root, ".code-harness", "VERSION"), "1.6.2\n")
	const runID = "r162canonical"
	requestDir := filepath.Join(root, ".code-harness", "runs", runID, "requests")
	mustWrite153Cmd(t, filepath.Join(requestDir, "change-set-request.json"), `{"runId":"r162canonical","baseRef":"HEAD","includeWorkingTree":true}`)
	withChdir153Cmd(t, root, func() {
		if err := run([]string{"analysis", "snapshot", "--input", ".code-harness/runs/r162canonical/requests/change-set-request.json"}); err != nil { t.Fatal(err) }
	})
	data, err := os.ReadFile(filepath.Join(root, ".code-harness", "runs", runID, "analysis", "change-set.json"))
	if err != nil { t.Fatal(err) }
	var snapshot canonicalSnapshot162Test
	if err := json.Unmarshal(data, &snapshot); err != nil { t.Fatal(err) }

	proposal := map[string]any{
		"changedFileRoles": []map[string]any{{"path": "src/main/resources/application.yml", "role": "YamlConfig"}},
		"affectedControllers": []any{}, "callChains": []any{}, "symbolLocations": []any{}, "resourceRelations": []any{},
		"externalDependencies": []any{}, "riskAreas": []any{},
		"reviewCoverage": map[string]any{"status": "COMPLETE", "reviewedFiles": []map[string]any{{"path": "src/main/resources/application.yml", "role": "YamlConfig", "reason": "CHANGED"}}, "unresolvedSymbols": []any{}},
	}
	proposalBytes, err := json.MarshalIndent(proposal, "", "  ")
	if err != nil { t.Fatal(err) }
	mustWrite153Cmd(t, filepath.Join(requestDir, "change-analysis-proposal.json"), string(proposalBytes))
	return root, runID, snapshot
}

type canonicalSnapshot162Test struct {
	RequestedBaseRef   string `json:"requestedBaseRef"`
	ResolvedBaseCommit string `json:"resolvedBaseCommit"`
	MergeBase          string `json:"mergeBase"`
	HeadCommit         string `json:"headCommit"`
	CurrentBranch      string `json:"currentBranch"`
	IncludeWorkingTree bool   `json:"includeWorkingTree"`
	SnapshotSHA256     string `json:"snapshotSha256"`
}

func write162CanonicalCertRequest(t *testing.T, root, runID string, snapshot canonicalSnapshot162Test) {
	t.Helper()
	req := map[string]any{
		"runId": runID,
		"snapshotPath": ".code-harness/runs/" + runID + "/analysis/change-set.json",
		"snapshotSha256": snapshot.SnapshotSHA256,
		"proposalPath": ".code-harness/runs/" + runID + "/requests/change-analysis-proposal.json",
		"intent": map[string]any{"mode": "FULL"},
	}
	b, err := json.Marshal(req)
	if err != nil { t.Fatal(err) }
	mustWrite153Cmd(t, filepath.Join(root, ".code-harness", "runs", runID, "requests", "analysis-certify.json"), string(b))
}
