package reviewauthority

import (
	"encoding/json"
	"testing"
)

func Test180SelectNeedsNextActualUser(t *testing.T) {
	cases := []struct {
		name      string
		data      string
		messageID string
		want      bool
	}{
		{"next user selects", export180("m-menu", "assistant", "run-1 options=hash-1\nC1 create\nC2 cancel", "m-user", "user", "选择 C1"), "m-user", true},
		{"assistant selects", export180("m-menu", "assistant", "run-1 options=hash-1\nC1 create\nC2 cancel", "m-a", "assistant", "选择 C1"), "m-a", false},
		{"user before menu", export180("m-user", "user", "选择 C1", "m-menu", "assistant", "run-1 options=hash-1\nC1 create\nC2 cancel"), "m-user", false},
		{"continue is not choice", export180("m-menu", "assistant", "run-1 options=hash-1\nC1 create\nC2 cancel", "m-user", "user", "继续"), "m-user", false},
		{"old menu user then new menu", exportMessages180([]exportMessage180{
			{id: "old", role: "assistant", text: "run-1 options=hash-1\nC1 create\nC2 cancel"},
			{id: "m-user", role: "user", text: "全部"},
			{id: "new", role: "assistant", text: "run-1 options=hash-2\nC1 create\nC2 cancel"},
		}), "m-user", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifySelectionTurnBytes180([]byte(tc.data), SelectionTurnRequest180{SessionID: "s", MessageID: tc.messageID, RunID: "run-1", OptionsHash: "hash-1", SelectionIDs: []string{"C1"}, AllIDs: []string{"C1", "C2"}, MenuMarker: "C1 create\nC2 cancel"})
			if tc.want && err != nil {
				t.Fatalf("expected accept: %v", err)
			}
			if !tc.want && err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func Test180SelectTurnRejectsOtherProject(t *testing.T) {
	data := export180("m-menu", "assistant", "run-1 options=hash-1\nC1 create\nC2 cancel", "m-user", "user", "选择 C1")
	err := verifySelectionTurnBytesForRoot180([]byte(data), `D:\other`, SelectionTurnRequest180{SessionID: "s", MessageID: "m-user", RunID: "run-1", OptionsHash: "hash-1", SelectionIDs: []string{"C1"}, AllIDs: []string{"C1", "C2"}, MenuMarker: "C1 create\nC2 cancel"})
	if err == nil {
		t.Fatal("cross-project user turn accepted")
	}
}

type exportMessage180 struct {
	id   string
	role string
	text string
}

func export180(id1, role1, text1, id2, role2, text2 string) string {
	return exportMessages180([]exportMessage180{{id: id1, role: role1, text: text1}, {id: id2, role: role2, text: text2}})
}

func exportMessages180(messages []exportMessage180) string {
	encoded := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		encoded = append(encoded, map[string]any{
			"info":  map[string]any{"id": message.id, "role": message.role},
			"parts": []map[string]any{{"type": "text", "text": message.text}},
		})
	}
	data, err := json.Marshal(map[string]any{
		"info":     map[string]any{"id": "s", "directory": "C:/repo"},
		"messages": encoded,
	})
	if err != nil {
		panic(err)
	}
	return string(data)
}
