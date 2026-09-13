package main

import (
	"path/filepath"
	"testing"
)

func Test170DiscoveryAuthorityIsChangeSetNotCertifiedAnalysis(t *testing.T) {
	path, certified, err := reviewContextAuthority170("review-case", "DISCOVERY")
	if err != nil { t.Fatal(err) }
	want := filepath.ToSlash(filepath.Join(".code-harness", "runs", "review-case", "analysis", "change-set.json"))
	if path != want || certified { t.Fatalf("path=%q certified=%v want=%q", path, certified, want) }

	path, certified, err = reviewContextAuthority170("review-case", "RULES")
	if err != nil { t.Fatal(err) }
	want = filepath.ToSlash(filepath.Join(".code-harness", "runs", "review-case", "analysis", "change-analysis.json"))
	if path != want || !certified { t.Fatalf("path=%q certified=%v want=%q", path, certified, want) }
}

func Test170ContextArtifactNamesFollowTwoPhaseContract(t *testing.T) {
	if got := reviewContextArtifactPath170("review-case", "DISCOVERY"); got != ".code-harness/runs/review-case/analysis/review-call-context.json" { t.Fatalf("DISCOVERY artifact=%q", got) }
	if got := reviewContextArtifactPath170("review-case", "RULES"); got != ".code-harness/runs/review-case/analysis/review-rule-context.json" { t.Fatalf("RULES artifact=%q", got) }
}
