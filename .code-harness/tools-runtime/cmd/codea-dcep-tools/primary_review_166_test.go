package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/reviewselection"
	"codea-harness-tools/internal/reviewunit"
)

func writePrimaryReceipt166(t *testing.T, root, runID, kind, proposalPath, userText, menu string) {
	t.Helper()
	writeReviewerAuthorityTestReceipt(t, root, runID, kind, proposalPath)
	name := "change-analysis-reviewer-authority.json"
	if kind == "findings" {
		name = "finding-reviewer-authority.json"
	}
	receiptPath := filepath.Join(root, ".code-harness", "runs", runID, "requests", name)
	b, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	var receipt map[string]any
	if err = json.Unmarshal(b, &receipt); err != nil {
		t.Fatal(err)
	}
	receipt["version"] = 2
	receipt["agent"] = "build"
	b, _ = json.Marshal(receipt)
	if kind == "selection" {
		receiptPath = filepath.Join(root, ".code-harness", "runs", runID, "requests", "review-selection-authority.json")
	}
	writeFile(t, receiptPath, string(b))
	exportPath := os.Getenv("CODEA_TEST_OPENCODE_EXPORT")
	b, err = os.ReadFile(exportPath)
	if err != nil {
		t.Fatal(err)
	}
	var exported map[string]any
	if err = json.Unmarshal(b, &exported); err != nil {
		t.Fatal(err)
	}
	delete(exported["info"].(map[string]any), "parentID")
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	exported["info"].(map[string]any)["directory"] = rootAbs
	messages := exported["messages"].([]any)
	for _, raw := range messages {
		raw.(map[string]any)["info"].(map[string]any)["agent"] = "build"
	}
	messages[0].(map[string]any)["parts"] = []any{map[string]any{"type": "text", "text": userText}}
	if menu != "" {
		messages = append([]any{map[string]any{"info": map[string]any{"id": "menu", "role": "assistant", "agent": "build"}, "parts": []any{map[string]any{"type": "text", "text": menu}}}}, messages...)
	}
	exported["messages"] = messages
	b, _ = json.Marshal(exported)
	writeFile(t, exportPath, string(b))
}

// Existing test fixtures seal analysis directly. Advance their declared version
// before exercising the real 1.6.6 selection/units/finding/report consumers.
func primaryFixtureVersion166(t *testing.T, runID string) {
	t.Helper()
	writeFile(t, ".code-harness/VERSION", "1.6.6\n")
	p := filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.cert.json")
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	var c analysisruntime.Certificate
	if e = json.Unmarshal(b, &c); e != nil {
		t.Fatal(e)
	}
	c.RuntimeVersion = "1.6.6"
	c.SemanticSessionID = "ses_test_reviewer_child_" + runID
	if err := analysisruntime.SealPrimarySessionAuthority(".", c); err != nil {
		t.Fatal(err)
	}
	b, _ = json.MarshalIndent(c, "", "  ")
	writeFile(t, p, string(append(b, '\n')))
}

func primaryFindingFixture166(t *testing.T) string {
	t.Helper()
	request := prepareTask164FindingCertification(t, true)
	const runID = "run-task4-review"
	primaryFixtureVersion166(t, runID)
	selection := ".code-harness/runs/" + runID + "/requests/reviewer-finding-select.json"
	for _, args := range [][]string{{"review", "select", "--input", selection}, {"review", "units", "--run-id", runID}, {"review", "dispatch", "--run-id", runID}} {
		if e := run(args); e != nil {
			t.Fatal(e)
		}
	}
	return request
}

func Test166MultiChainNeedsHumanAndPreservesSelectedScope(t *testing.T) {
	for _, mode := range []string{"missing", "FULL", "TARGETED", "LIST", "human", "scope-tamper", "list-after-selection", "cross-session"} {
		t.Run(mode, func(t *testing.T) {
			withTempProject(t)
			options := groupedReviewOptions(t, "")
			primaryFixtureVersion166(t, options.RunID)
			input := reviewunit.BuildInput{RunID: options.RunID, RepoRoot: "."}
			if mode == "missing" {
				if _, e := reviewunit.Build(input); e == nil || !strings.Contains(e.Error(), "HUMAN_SELECTION_REQUIRED") {
					t.Fatalf("missing choice accepted: %v", e)
				}
				return
			}
			req := reviewselection.SelectionRequest{RunID: options.RunID, Mode: "TARGETED", SelectionIDs: []string{options.Chains[0].SelectionID}, OptionsHash: options.OptionsHash}
			if mode == "FULL" || mode == "LIST" {
				req.Mode = mode
				req.SelectionIDs = nil
			}
			b, _ := json.Marshal(req)
			p := writeQueryRequest(t, options.RunID, "review-selection.json", string(b))
			authorized := mode == "human" || mode == "scope-tamper" || mode == "list-after-selection" || mode == "cross-session"
			if authorized {
				writePrimaryReceipt166(t, ".", options.RunID, "selection", p, "选择 "+req.SelectionIDs[0], reviewselection.SelectionPrompt(options))
			}
			if mode == "cross-session" {
				certPath := filepath.Join(".code-harness", "runs", options.RunID, "analysis", "change-analysis.cert.json")
				raw, _ := os.ReadFile(certPath)
				var cert analysisruntime.Certificate
				if err := json.Unmarshal(raw, &cert); err != nil {
					t.Fatal(err)
				}
				cert.SemanticSessionID = "another-primary-session"
				raw, _ = json.MarshalIndent(cert, "", "  ")
				writeFile(t, certPath, string(append(raw, '\n')))
			}
			e := run([]string{"review", "select", "--input", p})
			if !authorized || mode == "cross-session" {
				if e == nil || (!strings.Contains(e.Error(), "HUMAN_SELECTION_REQUIRED") && !strings.Contains(e.Error(), "PRIMARY_SESSION_AUTHORITY_MISMATCH")) {
					t.Fatalf("Agent choice accepted: %v", e)
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			if mode == "scope-tamper" {
				scopePath := filepath.Join(".code-harness", "runs", options.RunID, "analysis", "review-scope.json")
				writeFile(t, scopePath, `{"mode":"FULL","selectedCallChains":[],"scopedFiles":[]}`)
			}
			if mode == "list-after-selection" {
				req.Mode = "LIST"
				req.SelectionIDs = nil
				b, _ = json.Marshal(req)
				writeFile(t, p, string(b))
				writePrimaryReceipt166(t, ".", options.RunID, "selection", p, "仅列出", reviewselection.SelectionPrompt(options))
				if e := run([]string{"review", "select", "--input", p}); e != nil {
					t.Fatal(e)
				}
			}
			units, e := reviewunit.Build(input)
			if mode != "human" {
				if e == nil {
					t.Fatal("altered scope/LIST authorized review")
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			confirmed, err := reviewselection.VerifyAndBuildScope(".", req)
			if err != nil {
				t.Fatal(err)
			}
			confirmed.SelectedCallChains = append(confirmed.SelectedCallChains, options.Chains[1].CallChain)
			proposed, _ := json.Marshal(confirmed)
			selectedJSON, err := reviewselection.ReportScopeJSON(".", options.RunID, nil, proposed)
			if err != nil {
				t.Fatal(err)
			}
			var actual struct {
				SelectedCallChains []json.RawMessage `json:"selectedCallChains"`
			}
			if err := json.Unmarshal(selectedJSON, &actual); err != nil {
				t.Fatal(err)
			}
			if len(actual.SelectedCallChains) != 1 {
				t.Fatal("report transport expanded confirmed scope")
			}

			if len(units.Units) != 1 || units.Units[0].EntryPoint != options.Chains[0].CallChain.EntryPoint {
				t.Fatalf("scope expanded: %+v", units)
			}
		})
	}
}
