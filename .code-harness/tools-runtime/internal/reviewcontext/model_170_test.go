package reviewcontext

import "testing"

func Test170DefaultBudget(t *testing.T) {
	got := DefaultBudget170()
	want := Budget170{MaxFiles:40, MaxCandidates:200, MaxSourceBytes:1048576, MaxUpstreamDepth:3, MaxDownstreamDepth:6, MaxMillis:15000}
	if got != want { t.Fatalf("DefaultBudget170() = %+v, want %+v", got, want) }
}
