package reviewauthority

import (
	"encoding/json"
	"testing"
)

func TestSessionAttestationAcceptsSubmissionNumberNormalization(t *testing.T) {
	// The submission tool parses and stringifies the proposal, while OpenCode
	// retains the original input string in the completed tool call.
	original := `[{"confidence":1.0,"values":[0.90,1e-7,-0]}]`
	persisted := []byte("[\n  {\"confidence\": 1, \"values\": [0.9, 0.0000001, 0]}\n]\n")
	receipt := Receipt{RunID: "run-numbers", SessionID: "ses_child", MessageID: "msg_assistant", ProposalKind: Findings}
	exported := map[string]any{
		"info": map[string]any{"id": "ses_child", "parentID": "ses_parent"},
		"messages": []any{
			map[string]any{"info": map[string]any{"id": "msg_user", "role": "user", "agent": "reviewer"}},
			map[string]any{
				"info": map[string]any{"id": "msg_assistant", "role": "assistant", "parentID": "msg_user"},
				"parts": []any{map[string]any{
					"type": "tool", "tool": "codea-reviewer-submit",
					"state": map[string]any{"status": "completed", "input": map[string]any{
						"runId": receipt.RunID, "kind": "findings", "proposal": original,
					}},
				}},
			},
		},
	}
	exportBytes, err := json.Marshal(exported)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifySessionAttestation(exportBytes, receipt, persisted); err != nil {
		t.Fatalf("submission normalized by the official tool must attest: %v", err)
	}
	if err := verifySessionAttestation(exportBytes, receipt, []byte(`[{"confidence":0.9,"values":[0.9,0.0000001,0]}]`)); err == nil {
		t.Fatal("changed confidence must not attest")
	}
}

func TestSameJSONSemanticEquality(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{"integer decimal", `1.0`, `1`, true},
		{"fraction trailing zero", `0.90`, `0.9`, true},
		{"positive exponent", `1.20E+03`, `1200`, true},
		{"negative exponent", `1e-7`, `0.0000001`, true},
		{"negative number", `-1.20e1`, `-12`, true},
		{"negative zero", `-0.00e+30`, `0`, true},
		{"large integer normalization", `9007199254740993.0`, `9007199254740993`, true},
		{"huge exponent normalization", `10e999999999999999999999999`, `1e1000000000000000000000000`, true},
		{"huge negative exponent normalization", `0.1e-999999999999999999999999`, `1e-1000000000000000000000000`, true},
		{"nested reordered object", `{"a":[1.0,{"b":0.90}],"c":true}`, `{"c":true,"a":[1,{"b":0.9}]}`, true},
		{"string escapes", `"\u0061"`, `"a"`, true},
		{"null", `null`, `null`, true},
		{"changed number", `0.9`, `0.91`, false},
		{"distinct large integers", `9007199254740992`, `9007199254740993`, false},
		{"distinct precise decimals", `0.90000000000000000001`, `0.9`, false},
		{"tiny nonzero", `1e-1000000000000000000000000`, `0`, false},
		{"opposite signs", `1`, `-1`, false},
		{"number string", `1`, `"1"`, false},
		{"changed string", `"a"`, `"b"`, false},
		{"changed bool", `true`, `false`, false},
		{"changed key", `{"a":null}`, `{"b":null}`, false},
		{"missing key", `{"a":null}`, `{}`, false},
		{"array order", `[1,2]`, `[2,1]`, false},
		{"array length", `[1]`, `[1,2]`, false},
		{"array object", `[]`, `{}`, false},
		{"null array", `null`, `[]`, false},
		{"null object", `null`, `{}`, false},
		{"null string", `null`, `""`, false},
		{"trailing JSON", `1 2`, `1`, false},
		{"trailing garbage", `1x`, `1`, false},
		{"invalid number", `01`, `1`, false},
		{"malformed JSON", `[`, `[]`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sameJSON([]byte(tt.a), []byte(tt.b)); got != tt.want {
				t.Errorf("sameJSON(%s, %s) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
			if got := sameJSON([]byte(tt.b), []byte(tt.a)); got != tt.want {
				t.Errorf("reverse comparison = %v, want %v", got, tt.want)
			}
		})
	}
}
