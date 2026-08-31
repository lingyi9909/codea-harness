package main

import "testing"

func Test153Task6ChainEditAndReleaseContract(t *testing.T) {
	workflow := task153Task6Read(t, ".github/workflows/task153-chain-reliability.yml")
	packageWorkflow := task153Task6Read(t, ".github/workflows/package-windows-x64.yml")
	releaseDriver := task153Task6Read(t, ".github/scripts/task161-release.ps1")
	script := task153Task6Read(t, ".github/scripts/task153-real-review-chain-regression.ps1")
	changelog := task153Task6Read(t, "CHANGELOG.md")

	task153Task6RequireContains(t, workflow,
		"Task 5 Chain edit gate",
		"Full Go test",
		"Go vet",
	)
	task153Task6RequireContains(t, script,
		"CHAIN_EDIT_VERIFIED",
		"UNVERIFIED_EDIT_REJECTED",
		"TASK153_REAL_REVIEW_CHAIN_RELIABILITY PASS",
	)
	task153Task6RequireContains(t, changelog,
		"## 1.5.3",
		"Controller",
		"Certified ChangeAnalysis",
		"AUTO_SINGLE",
		"Chain Edit",
	)
	// This is a historical 1.5.3 compatibility contract. Verify the regression
	// remains wired through the current release driver without pinning the
	// repository's current patch release, artifact names, or workflow display text.
	task153Task6RequireContains(t, packageWorkflow,
		"task161-release.ps1",
	)
	task153Task6RequireContains(t, releaseDriver,
		"task153-real-review-chain-regression.ps1",
	)
}
