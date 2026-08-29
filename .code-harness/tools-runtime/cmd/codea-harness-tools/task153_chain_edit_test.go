package main

import (
	"strings"
	"testing"
)

func Test153Task6ChainEditAndReleaseContract(t *testing.T) {
	workflow := task153Task6Read(t, ".github/workflows/task153-chain-reliability.yml")
	packageWorkflow := task153Task6Read(t, ".github/workflows/package-windows-x64.yml")
	script := task153Task6Read(t, ".github/scripts/task153-real-review-chain-regression.ps1")
	version := task153Task6Read(t, ".code-harness/VERSION")
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
	if strings.TrimSpace(version) != "1.6.0" {
		t.Fatalf("current release VERSION=%q want 1.6.0", version)
	}
	task153Task6RequireContains(t, changelog,
		"## 1.5.3",
		"Controller",
		"Certified ChangeAnalysis",
		"AUTO_SINGLE",
		"Chain Edit",
	)
	task153Task6RequireContains(t, packageWorkflow,
		"Task 1.5.3 real review/chain reliability regression",
		"6f4c050783a7ec21f370799c1a8c69c9b51a9e92",
		"codea-harness-1.6.0-windows-x64-install.zip",
		"codea-harness-1.6.0-windows-x64-upgrade.zip",
		"1.5.3 -> 1.6.0",
	)
}
