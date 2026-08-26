package main

import "testing"

func Test153Task6ReviewSelectionReleaseContract(t *testing.T) {
	workflow := task153Task6Read(t, ".github/workflows/task153-chain-reliability.yml")
	script := task153Task6Read(t, ".github/scripts/task153-real-review-chain-regression.ps1")
	task153Task6RequireContains(t, workflow,
		"Task 4 Review options and AUTO_SINGLE gate",
	)
	task153Task6RequireContains(t, script,
		"AUTO_SINGLE_NO_SELECTION",
		"MULTI_CHAIN_SELECTION_VERIFIED",
	)
}
