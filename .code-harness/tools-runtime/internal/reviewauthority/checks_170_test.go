package reviewauthority

import (
	"encoding/json"
	"strings"
	"testing"
)

func Test170FindingsV2ReceiptBindsChecksToSameReviewerToolCall(t *testing.T) {
	proposal := []byte("[]\n")
	checks := []byte("[{\"reviewUnitId\":\"RU-1\",\"ruleId\":\"SPRING-TX-001\",\"status\":\"COMPLETED\",\"reason\":\"\",\"sourceIds\":[]}]\n")
	receipt := Receipt{Version:2, Host:"opencode", Source:"opencode-tool-context", RunID:"run-170", ProposalKind:Findings, Agent:"reviewer", SessionID:"ses_child170", MessageID:"msg-submit", ProposalPath:".code-harness/runs/run-170/requests/finding-proposals.json", ProposalSHA256:hashBytes170(proposal), ChecksPath:".code-harness/runs/run-170/requests/review-checks.json", ChecksSHA256:hashBytes170(checks)}
	exported := reviewerExport170(t, receipt, string(proposal), string(checks))
	if err := verifySessionAttestation170(exported, receipt, proposal, checks); err != nil {
		t.Fatalf("valid v2 reviewer attestation must pass: %v", err)
	}

	badChecks := []byte("[]\n")
	if err := verifySessionAttestation170(exported, receipt, proposal, badChecks); err == nil || !strings.Contains(err.Error(), "checks") {
		t.Fatalf("checks from another invocation/hash must fail, got %v", err)
	}

	downgrade := receipt
	downgrade.Version = 1
	if err := verifySessionAttestation170(exported, downgrade, proposal, checks); err == nil || !strings.Contains(err.Error(), "v2") {
		t.Fatalf("findings v1 downgrade must fail, got %v", err)
	}
}

func Test170ChangeAnalysisRetainsV1ReviewerSemantics(t *testing.T) {
	receipt := Receipt{Version:1, Host:"opencode", Source:"opencode-tool-context", RunID:"run-170", ProposalKind:ChangeAnalysis, Agent:"reviewer", SessionID:"ses_child170", MessageID:"msg-submit", ProposalPath:".code-harness/runs/run-170/requests/change-analysis-proposal.json"}
	proposal := []byte("{}\n")
	receipt.ProposalSHA256 = hashBytes170(proposal)
	exported := reviewerExport170(t, receipt, string(proposal), "")
	if err := verifySessionAttestation170(exported, receipt, proposal, nil); err != nil {
		t.Fatalf("change-analysis v1 must remain valid: %v", err)
	}
}

func reviewerExport170(t *testing.T, receipt Receipt, proposal, checks string) []byte {
	t.Helper()
	input := map[string]any{"runId":receipt.RunID, "kind":string(receipt.ProposalKind), "proposal":proposal}
	if receipt.ProposalKind == Findings { input["checks"] = checks }
	value := map[string]any{
		"info":map[string]any{"id":receipt.SessionID, "parentID":"ses_parent"},
		"messages":[]any{
			map[string]any{"info":map[string]any{"id":"msg-user", "role":"user", "agent":"reviewer"}, "parts":[]any{}},
			map[string]any{"info":map[string]any{"id":receipt.MessageID, "role":"assistant", "parentID":"msg-user"}, "parts":[]any{map[string]any{"type":"tool", "tool":"codea-reviewer-submit", "state":map[string]any{"status":"completed", "input":input}}}},
		},
	}
	data, err := json.Marshal(value)
	if err != nil { t.Fatal(err) }
	return data
}
