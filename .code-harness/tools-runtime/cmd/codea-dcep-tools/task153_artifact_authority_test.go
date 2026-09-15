package main

import "testing"

func Test153Task6ArtifactAuthorityReleaseContract(t *testing.T) {
	script := task153Task6Read(t, ".github/scripts/task153-real-review-chain-regression.ps1")
	task153Task6RequireContains(t, script,
		"CERTIFIED_ANALYSIS_TAMPER_REJECTED",
		"CHAIN_CANDIDATE_TAMPER_REJECTED",
	)
}
