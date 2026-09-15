package main

import "testing"

func Test153Task6ReviewSelectionReleaseContract(t *testing.T) {
	script := task153Task6Read(t, ".github/scripts/task153-real-review-chain-regression.ps1")
	task153Task6RequireContains(t, script,
		"AUTO_SINGLE_NO_SELECTION",
		"MULTI_CHAIN_SELECTION_VERIFIED",
	)
}
