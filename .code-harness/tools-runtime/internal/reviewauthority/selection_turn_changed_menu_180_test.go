package reviewauthority

import "testing"

func Test180SelectRejectsNewerChangedMenuBeforeUser(t *testing.T) {
	data := exportMessages180([]exportMessage180{
		{id: "old", role: "assistant", text: "run-1 options=hash-1\nC1 create\nC2 cancel"},
		{id: "new", role: "assistant", text: "run-1 options=hash-2\nC1 create\nC3 refund"},
		{id: "m-user", role: "user", text: "选择 C1"},
	})
	err := VerifySelectionTurnBytes180([]byte(data), SelectionTurnRequest180{
		SessionID: "s", MessageID: "m-user", RunID: "run-1", OptionsHash: "hash-1",
		SelectionIDs: []string{"C1"}, AllIDs: []string{"C1", "C2"}, MenuMarker: "C1 create\nC2 cancel",
	})
	if err == nil {
		t.Fatal("selection from old menu accepted after a newer changed menu")
	}
}

func Test180SelectRejectsNewerChangedMenuAfterUser(t *testing.T) {
	data := exportMessages180([]exportMessage180{
		{id: "old", role: "assistant", text: "run-1 options=hash-1\nC1 create\nC2 cancel"},
		{id: "m-user", role: "user", text: "选择 C1"},
		{id: "new", role: "assistant", text: "run-1 options=hash-2\nC1 create\nC3 refund"},
	})
	err := VerifySelectionTurnBytes180([]byte(data), SelectionTurnRequest180{
		SessionID: "s", MessageID: "m-user", RunID: "run-1", OptionsHash: "hash-1",
		SelectionIDs: []string{"C1"}, AllIDs: []string{"C1", "C2"}, MenuMarker: "C1 create\nC2 cancel",
	})
	if err == nil {
		t.Fatal("selection from old menu accepted after a newer changed menu was emitted")
	}
}
