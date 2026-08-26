package main

import "testing"

func Test153Task6ArtifactAuthorityReleaseContract(t *testing.T) {
	workflow := task153Task6Read(t, ".github/workflows/task153-chain-reliability.yml")
	script := task153Task6Read(t, ".github/scripts/task153-real-review-chain-regression.ps1")
	task153Task6RequireContains(t, workflow,
		"Task 2 Certified ChangeAnalysis gate",
		"Task 3 Chain artifact authority gate",
	)
	task153Task6RequireContains(t, script,
		"CERTIFIED_ANALYSIS_TAMPER_REJECTED",
		"CHAIN_CANDIDATE_TAMPER_REJECTED",
	)
}
