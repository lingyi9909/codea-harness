package reviewauthority

import (
	"encoding/json"
	"testing"
)

func Test180SelectionUsesLastMenuWithinAssistantMessage(t *testing.T) {
	const marker = "C1 create\nC2 cancel"
	const a = "run-A options=hash-A\n" + marker
	const b = "run-B options=hash-B\n" + marker
	for _, tc := range []struct {
		name      string
		parts     []string
		run, hash string
		accept    bool
	}{
		{"separate parts reject A", []string{a, b}, "run-A", "hash-A", false},
		{"same part reject A", []string{a + "\n" + b}, "run-A", "hash-A", false},
		{"separate parts accept B", []string{a, b}, "run-B", "hash-B", true},
		{"same part accept B", []string{a + "\n" + b}, "run-B", "hash-B", true},
		{"split latest menu", []string{a, "run-B options=hash-B", marker}, "run-B", "hash-B", true},
		{"new hash same run", []string{a + "\nrun-A options=hash-new\n" + marker}, "run-A", "hash-A", false},
		{"hash prefix is not identity", []string{"run-A options=hash-A-extra\n" + marker}, "run-A", "hash-A", false},
		{"cannot borrow old marker", []string{a + "\nrun-B options=hash-B\nC1 other"}, "run-B", "hash-B", false},
		{"malformed latest header", []string{a, "run-B options="}, "run-A", "hash-A", false},
		{"unordered inline headers reject", []string{a + " run-B options=hash-B"}, "run-A", "hash-A", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parts := []map[string]any{}
			for _, text := range tc.parts {
				parts = append(parts, map[string]any{"type": "text", "text": text})
			}
			data, err := json.Marshal(map[string]any{
				"info": map[string]any{"id": "s", "directory": "C:/repo"},
				"messages": []map[string]any{
					{"info": map[string]any{"id": "menu", "role": "assistant"}, "parts": parts},
					{"info": map[string]any{"id": "reply", "role": "user"}, "parts": []map[string]any{{"type": "text", "text": "选择 C1"}}},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			err = VerifySelectionTurnBytes180(data, SelectionTurnRequest180{
				SessionID: "s", MessageID: "reply", RunID: tc.run, OptionsHash: tc.hash,
				SelectionIDs: []string{"C1"}, AllIDs: []string{"C1", "C2"}, MenuMarker: marker,
			})
			if (err == nil) != tc.accept {
				t.Fatalf("accept=%v, err=%v", tc.accept, err)
			}
		})
	}
}
