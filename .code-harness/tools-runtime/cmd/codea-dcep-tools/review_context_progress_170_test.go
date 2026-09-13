package main

import (
	"testing"

	"codea-harness-tools/internal/reviewprogress"
)

func Test170ContextPhaseAuthority(t *testing.T) {
	cases := []struct{ name string; state reviewprogress.State; phase string; ok bool }{
		{"discovery-during-analysis", reviewprogress.State{Status:reviewprogress.StatusRunning, CurrentStage:reviewprogress.StageChangeAnalysis}, "DISCOVERY", true},
		{"rules-during-planning", reviewprogress.State{Status:reviewprogress.StatusRunning, CurrentStage:reviewprogress.StageReviewPlanning}, "RULES", true},
		{"rules-too-early", reviewprogress.State{Status:reviewprogress.StatusRunning, CurrentStage:reviewprogress.StageChangeAnalysis}, "RULES", false},
		{"discovery-too-late", reviewprogress.State{Status:reviewprogress.StatusRunning, CurrentStage:reviewprogress.StageReviewPlanning}, "DISCOVERY", false},
		{"terminal-rejected", reviewprogress.State{Status:reviewprogress.StatusFailed, CurrentStage:reviewprogress.StageReviewPlanning}, "RULES", false},
	}
	for _, tc := range cases { t.Run(tc.name, func(t *testing.T) {
		err := validateReviewContextProgress170(tc.state, tc.phase)
		if tc.ok && err != nil { t.Fatal(err) }
		if !tc.ok && err == nil { t.Fatal("invalid phase/state accepted") }
	}) }
}
