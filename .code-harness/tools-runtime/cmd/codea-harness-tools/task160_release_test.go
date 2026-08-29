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
	if strings.TrimSpace(string(version)) != "1.6.0" { t.Fatalf("VERSION must be 1.6.0, got %q", strings.TrimSpace(string(version))) }

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
		t.Fatalf("1.6.0 CHANGELOG must contain exactly 6 scoped bullets, got %d", got)
	}

	workflow, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "package-windows-x64.yml"))
	if err != nil { t.Fatal(err) }
	w := string(workflow)
	for _, want := range []string{
		"Task160 review precision gate before staging",
		"6f4c050783a7ec21f370799c1a8c69c9b51a9e92",
		"1.5.3 -> 1.6.0",
		"codea-harness-1.6.0-windows-x64-install.zip",
		"codea-harness-1.6.0-windows-x64-upgrade.zip",
		"codea-harness-1.6.0-release-checklist",
		"review units --run-id __missing__",
		"review-rules/spring-v1.yaml",
		"@('review','units','--run-id','__missing__')",
	} {
		if !strings.Contains(w, want) { t.Fatalf("package workflow missing %q", want) }
	}
	if strings.Contains(w, "@('review','units','--run-id','__missing__','--project-root'") {
		t.Fatal("review units capability probe must not pass unsupported --project-root")
	}
}
