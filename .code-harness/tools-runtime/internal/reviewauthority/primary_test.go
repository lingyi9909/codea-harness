package reviewauthority

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func primaryFixture(kind Kind, proposal string) (Receipt, map[string]any) {
	r := Receipt{Version: 2, Agent: "build", SessionID: "opaque-primary", MessageID: "assistant-submit", RunID: "review-166", ProposalKind: kind}
	user := map[string]any{"info": map[string]any{"id": "human", "role": "user", "agent": "build"}, "parts": []any{map[string]any{"type": "text", "text": "选择 C1"}}}
	menu := map[string]any{"info": map[string]any{"id": "menu", "role": "assistant"}, "parts": []any{map[string]any{"type": "text", "text": "review-166 hash-current C1 C2"}}}
	submit := map[string]any{"info": map[string]any{"id": r.MessageID, "role": "assistant", "parentID": "human", "agent": "build"}, "parts": []any{map[string]any{"type": "tool", "tool": "codea-reviewer-submit", "state": map[string]any{"status": "completed", "input": map[string]any{"runId": r.RunID, "kind": kind, "proposal": proposal}}}}}
	return r, map[string]any{"info": map[string]any{"id": r.SessionID}, "messages": []any{menu, user, submit}}
}
func TestPrimarySubmissionAttestation166(t *testing.T) {
	r, v := primaryFixture(ChangeAnalysis, `{}`)
	b, _ := json.Marshal(v)
	if err := verifySessionAttestation(b, r, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	v["info"].(map[string]any)["parentID"] = "parent"
	b, _ = json.Marshal(v)
	if err := verifySessionAttestation(b, r, []byte(`{}`)); err == nil {
		t.Fatal("child accepted as primary")
	}
}
func TestPrimarySelectionRequiresActualNextUserTurn166(t *testing.T) {
	proposal := `{"runId":"review-166","mode":"TARGETED","optionsHash":"hash-current","selectionIds":["C1"]}`
	for _, mutation := range []string{"none", "self", "synthetic", "wrong-choice", "stale-menu", "incomplete", "missing-options"} {
		t.Run(mutation, func(t *testing.T) {
			r, v := primaryFixture(Selection, proposal)
			messages := v["messages"].([]any)
			user := messages[1].(map[string]any)
			switch mutation {
			case "self":
				user["info"].(map[string]any)["role"] = "assistant"
			case "synthetic":
				user["parts"].([]any)[0].(map[string]any)["synthetic"] = true
			case "wrong-choice":
				user["parts"].([]any)[0].(map[string]any)["text"] = "选择 C2"
			case "stale-menu":
				messages[0].(map[string]any)["parts"].([]any)[0].(map[string]any)["text"] = "old menu"
			case "missing-options":
				messages[0].(map[string]any)["parts"].([]any)[0].(map[string]any)["text"] = "review-166 hash-current"
			case "incomplete":
				messages[2].(map[string]any)["parts"].([]any)[0].(map[string]any)["state"].(map[string]any)["status"] = "running"
			}
			b, _ := json.Marshal(v)
			err := verifySessionAttestation(b, r, []byte(proposal), "review-166 hash-current C1 C2")
			if mutation == "none" && err != nil {
				t.Fatal(err)
			}
			if mutation != "none" && err == nil {
				t.Fatal("invalid selection accepted")
			}
		})
	}
}

func TestPrimaryDirectoryBoundToProject166(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	for _, directory := range []string{root, other, "", filepath.Base(root)} {
		data, _ := json.Marshal(map[string]any{"info": map[string]any{"directory": directory}})
		err := verifyPrimaryDirectory(root, data)
		if directory == root && err != nil {
			t.Fatal(err)
		}
		if directory != root && err == nil {
			t.Fatalf("foreign/missing directory accepted: %q", directory)
		}
	}
}
