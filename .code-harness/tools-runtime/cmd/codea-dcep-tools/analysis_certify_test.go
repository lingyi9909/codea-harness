package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	analysisruntime "codea-harness-tools/internal/analysis"
)

func Test153AnalysisCertifyPublishesCertifiedBundle(t *testing.T) {
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

	copyTask153CommandContract(t, root, "change-analysis.schema.json")
	copyTask153CommandContract(t, root, "entrypoint-inventory.schema.json")
	copyTask153CommandContract(t, root, "change-analysis-cert.schema.json")
	mustWrite153Cmd(t, filepath.Join(root, ".code-harness", "VERSION"), "1.5.2\n")

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
	if err != nil { t.Fatal(err) }
	draftPath := filepath.Join(root, ".code-harness", "runs", "r153", "requests", "change-analysis-draft.json")
	mustWrite153Cmd(t, draftPath, string(append(draftBytes, '\n')))

	req := analysisruntime.CertifyRequest{
		RunID: "r153", DraftPath: ".code-harness/runs/r153/requests/change-analysis-draft.json",
		BaseRef: "HEAD", IncludeWorkingTree: true, Intent: analysisruntime.Intent{Mode: "CHAIN_MAINTENANCE", Target: "fixture-maintenance"},
	}
	reqBytes, err := json.Marshal(req)
	if err != nil { t.Fatal(err) }
	requestPath := filepath.Join(root, ".code-harness", "runs", "r153", "requests", "analysis-certify.json")
	mustWrite153Cmd(t, requestPath, string(reqBytes))

	withChdir153Cmd(t, root, func() {
		if err := run([]string{"analysis", "certify", "--input", ".code-harness/runs/r153/requests/analysis-certify.json"}); err != nil {
			t.Fatalf("analysis certify failed: %v", err)
		}
		analysisPath := ".code-harness/runs/r153/analysis/change-analysis.json"
		if _, cert, err := analysisruntime.LoadCertified(".", analysisPath); err != nil {
			t.Fatalf("published bundle must load as certified: %v", err)
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

func Test153AnalysisCertifyRejectsRunIDPathMismatch(t *testing.T) {
	root := t.TempDir()
	requestPath := filepath.Join(root, ".code-harness", "runs", "r153", "requests", "analysis-certify.json")
	mustWrite153Cmd(t, requestPath, `{"runId":"other","draftPath":".code-harness/runs/other/requests/change-analysis-draft.json","baseRef":"HEAD","includeWorkingTree":true,"intent":{"mode":"FULL"}}`)
	withChdir153Cmd(t, root, func() {
		err := run([]string{"analysis", "certify", "--input", ".code-harness/runs/r153/requests/analysis-certify.json"})
		if err == nil || !strings.Contains(err.Error(), "RUN_ID_PATH_MISMATCH") {
			t.Fatalf("expected same-run certify request rejection, got %v", err)
		}
	})
}

func copyTask153CommandContract(t *testing.T, root, name string) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok { t.Fatal("runtime.Caller failed") }
	source := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "contracts", name))
	b, err := os.ReadFile(source)
	if err != nil { t.Fatalf("read contract %s: %v", name, err) }
	mustWrite153Cmd(t, filepath.Join(root, ".code-harness", "contracts", name), string(b))
}