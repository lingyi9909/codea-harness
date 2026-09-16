package reviewauthority

import "testing"

func Test180SelectionBindsToLatestMenuAcrossRuns(t *testing.T) {
	data := exportMessages180([]exportMessage180{
		{id: "menu-a", role: "assistant", text: "run-A options=hash-A\nC1 create\nC2 cancel"},
		{id: "menu-b", role: "assistant", text: "run-B options=hash-B\nC1 create\nC2 cancel"},
		{id: "reply", role: "user", text: "选择 C1"},
	})

	err := VerifySelectionTurnBytes180([]byte(data), SelectionTurnRequest180{
		SessionID:    "s",
		MessageID:    "reply",
		RunID:        "run-A",
		OptionsHash:  "hash-A",
		SelectionIDs: []string{"C1"},
		AllIDs:       []string{"C1", "C2"},
		MenuMarker:   "C1 create\nC2 cancel",
	})
	if err == nil {
		t.Fatal("selection for run A was accepted even though run B emitted the latest menu before the reply")
	}
}
