package reviewauthority

import (
	"encoding/json"
	"testing"
)

func Test180SelectionAcceptsCurrentAssistantContextAnchorOnlyWhenParentIsRealUser(t *testing.T) {
	const marker = "C1 OrderController.create\nC2 OrderController.cancel"
	data, err := json.Marshal(map[string]any{
		"info": map[string]any{"id": "session-opaque", "directory": "C:/repo"},
		"messages": []map[string]any{
			{
				"info": map[string]any{"id": "menu", "role": "assistant"},
				"parts": []map[string]any{{"type": "text", "text": "run-1 options=hash-1\n" + marker}},
			},
			{
				"info": map[string]any{"id": "user-choice", "role": "user"},
				"parts": []map[string]any{{"type": "text", "text": "选择 C1"}},
			},
			{
				"info": map[string]any{"id": "assistant-tool", "role": "assistant", "parentID": "user-choice"},
				"parts": []map[string]any{{"type": "tool", "tool": "codea-review"}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	err = VerifySelectionTurnBytes180(data, SelectionTurnRequest180{
		SessionID: "session-opaque", MessageID: "assistant-tool", RunID: "run-1", OptionsHash: "hash-1",
		SelectionIDs: []string{"C1"}, AllIDs: []string{"C1", "C2"}, MenuMarker: marker,
	})
	if err != nil {
		t.Fatalf("real OpenCode tool context should bind through assistant parent user: %v", err)
	}
}

func Test180SelectionRejectsAssistantContextWithoutRealUserParent(t *testing.T) {
	const marker = "C1 OrderController.create\nC2 OrderController.cancel"
	data, err := json.Marshal(map[string]any{
		"info": map[string]any{"id": "session-opaque", "directory": "C:/repo"},
		"messages": []map[string]any{
			{"info": map[string]any{"id": "menu", "role": "assistant"}, "parts": []map[string]any{{"type": "text", "text": "run-1 options=hash-1\n" + marker}}},
			{"info": map[string]any{"id": "assistant-tool", "role": "assistant", "parentID": "missing-user"}, "parts": []map[string]any{{"type": "tool", "tool": "codea-review"}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifySelectionTurnBytes180(data, SelectionTurnRequest180{
		SessionID: "session-opaque", MessageID: "assistant-tool", RunID: "run-1", OptionsHash: "hash-1",
		SelectionIDs: []string{"C1"}, AllIDs: []string{"C1", "C2"}, MenuMarker: marker,
	}); err == nil {
		t.Fatal("assistant context without a real parent user must fail closed")
	}
}
