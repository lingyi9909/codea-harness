package reviewauthority

import "testing"

func Test180SelectAllMeansExactlyTheCurrentMenu(t *testing.T) {
	data := export180(
		"m-menu", "assistant", "run-1 options=hash-1\nC1 create\nC2 cancel",
		"m-user", "user", "全部",
	)
	base := SelectionTurnRequest180{
		SessionID:   "s",
		MessageID:   "m-user",
		RunID:       "run-1",
		OptionsHash: "hash-1",
		AllIDs:      []string{"C1", "C2"},
		MenuMarker:  "C1 create\nC2 cancel",
	}

	all := base
	all.SelectionIDs = []string{"C2", "C1"}
	if err := VerifySelectionTurnBytes180([]byte(data), all); err != nil {
		t.Fatalf("current-menu 全部 must select exactly C1+C2: %v", err)
	}

	partial := base
	partial.SelectionIDs = []string{"C1"}
	if err := VerifySelectionTurnBytes180([]byte(data), partial); err == nil {
		t.Fatal("全部 must not authorize only a subset of the current menu")
	}
}
