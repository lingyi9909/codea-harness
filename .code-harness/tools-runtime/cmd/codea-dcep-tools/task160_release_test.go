package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTask160ReleaseMetadataAndPackageWorkflow(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	version, err := os.ReadFile(filepath.Join(root, ".code-harness", "VERSION"))
	if err != nil { t.Fatal(err) }
	currentVersion := strings.TrimSpace(string(version))
	if currentVersion != "1.6.1" && currentVersion != "1.6.2" {
		t.Fatalf("VERSION must preserve retained 1.6.1 or current 1.6.2, got %q", currentVersion)
	}

	changelog, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil { t.Fatal(err) }
	text := string(changelog)
	for _, want := range []string{
		"## 1.6.0",
		"deterministic ReviewUnit",
		"deterministic Spring Rule Dispatch",
		"Finding Proposal -> Runtime Verified/Certified Finding",
		"Spring Rule Pack v1",
		"10 high-value rules",
		"24-case Review Precision benchmark",
		"1.5.3 behavior preserved",
	} {
		if !strings.Contains(text, want) { t.Fatalf("CHANGELOG missing %q", want) }
	}
	section := strings.SplitN(text, "## 1.5.3", 2)[0]
	if got := strings.Count(section, "\n- **"); got != 6 {
		t.Fatalf("1.6.0 CHANGELOG must preserve exactly 6 scoped Task160 bullets, got %d", got)
	}

	workflow, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "package-windows-x64.yml"))
	if err != nil { t.Fatal(err) }
	w := string(workflow)
	for _, want := range []string{
		"./.github/scripts/task161-release.ps1",
		"codea-harness-1.6.1-windows-x64-install",
		"codea-harness-1.6.1-windows-x64-upgrade",
		"codea-harness-1.6.1-release-checklist",
		"codea-dcep-tools-whitelist",
	} {
		if !strings.Contains(w, want) { t.Fatalf("package workflow missing %q", want) }
	}

	releaseScript, err := os.ReadFile(filepath.Join(root, ".github", "scripts", "task161-release.ps1"))
	if err != nil { t.Fatal(err) }
	r := string(releaseScript)
	for _, want := range []string{
		"task160-real-review-precision-regression.ps1",
		"c07f0a4e029a50de64d271fc4ea83015b06355a1",
		"1.6.0 -> 1.6.1",
		"codea-harness-1.6.1-windows-x64-install.zip",
		"codea-harness-1.6.1-windows-x64-upgrade.zip",
		"codea-harness-1.6.1-release-checklist.json",
		"review units --run-id __missing__",
		"codea-dcep-tools.exe",
	} {
		if !strings.Contains(r, want) { t.Fatalf("1.6.1 release driver missing %q", want) }
	}
	if strings.Contains(r, "review units --run-id __missing__ --project-root") {
		t.Fatal("review units capability probe must not pass unsupported --project-root")
	}
}
