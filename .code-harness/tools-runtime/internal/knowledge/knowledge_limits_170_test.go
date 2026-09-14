package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test170KnowledgeDocumentAndByteBudgetsFailClosed(t *testing.T) {
	t.Run("document-count", func(t *testing.T) {
		repo := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repo, "docs"), 0o755); err != nil { t.Fatal(err) }
		sources := make([]Source170, 0, 21)
		for i := 0; i < 21; i++ {
			name := fmt.Sprintf("ref-%02d.md", i)
			if err := os.WriteFile(filepath.Join(repo, "docs", name), []byte("x"), 0o644); err != nil { t.Fatal(err) }
			sources = append(sources, Source170{ID: fmt.Sprintf("ref-%02d", i), Root: "PROJECT", Path: "docs/" + name, Kind: "REFERENCE", Required: true})
		}
		result, err := Load170(LoadInput170{RunID: "review-t7-doc-cap", RepoRoot: repo, Binding: Binding170{Version: 1, ProjectID: "p", Sources: sources}, BindingSHA256: strings.Repeat("1", 64), Units: []Unit170{{ID: "u"}}})
		if err != nil { t.Fatal(err) }
		if len(result.Documents) != 20 || result.Manifest.Status != "PARTIAL" { t.Fatalf("document cap not enforced: docs=%d manifest=%+v", len(result.Documents), result.Manifest) }
		blocked := 0
		for _, source := range result.Manifest.Sources { if source.Status == "BLOCKED" && strings.Contains(strings.Join(source.Reasons, " "), "BUDGET_DOCUMENTS") { blocked++ } }
		if blocked != 1 { t.Fatalf("expected exactly one document-budget block, got %d", blocked) }
	})

	t.Run("byte-count", func(t *testing.T) {
		repo := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repo, "docs"), 0o755); err != nil { t.Fatal(err) }
		if err := os.WriteFile(filepath.Join(repo, "docs", "a.md"), []byte(strings.Repeat("a", 140*1024)), 0o644); err != nil { t.Fatal(err) }
		if err := os.WriteFile(filepath.Join(repo, "docs", "b.md"), []byte(strings.Repeat("b", 140*1024)), 0o644); err != nil { t.Fatal(err) }
		binding := Binding170{Version: 1, ProjectID: "p", Sources: []Source170{
			{ID: "a", Root: "PROJECT", Path: "docs/a.md", Kind: "REFERENCE", Required: true},
			{ID: "b", Root: "PROJECT", Path: "docs/b.md", Kind: "REFERENCE", Required: true},
		}}
		result, err := Load170(LoadInput170{RunID: "review-t7-byte-cap", RepoRoot: repo, Binding: binding, BindingSHA256: strings.Repeat("2", 64), Units: []Unit170{{ID: "u"}}})
		if err != nil { t.Fatal(err) }
		if len(result.Documents) != 1 || result.Manifest.Status != "PARTIAL" { t.Fatalf("byte cap not enforced: docs=%d manifest=%+v", len(result.Documents), result.Manifest) }
		if result.Manifest.Sources[1].Status != "BLOCKED" || !strings.Contains(strings.Join(result.Manifest.Sources[1].Reasons, " "), "BUDGET_BYTES") { t.Fatalf("second source must be byte-budget blocked: %+v", result.Manifest.Sources) }
	})
}

func Test170KnowledgeDraftRetiredAndMetadataNeverBecomeActiveRules(t *testing.T) {
	for _, status := range []string{"DRAFT", "RETIRED"} {
		t.Run(status, func(t *testing.T) {
			repo := t.TempDir()
			if err := os.MkdirAll(filepath.Join(repo, "rules"), 0o755); err != nil { t.Fatal(err) }
			raw := strings.Replace(string(activeRule170("ORDER-001", "order-service", "./")), "status: ACTIVE", "status: "+status, 1)
			if err := os.WriteFile(filepath.Join(repo, "rules", "rule.md"), []byte(raw), 0o644); err != nil { t.Fatal(err) }
			binding := Binding170{Version: 1, ProjectID: "order-service", Sources: []Source170{{ID: "rule", Root: "PROJECT", Path: "rules/rule.md", Kind: "RULES", Required: true}}}
			result, err := Load170(LoadInput170{RunID: "review-t7-inactive-"+strings.ToLower(status), RepoRoot: repo, Binding: binding, BindingSHA256: strings.Repeat("3", 64), Units: []Unit170{{ID: "u", Paths: []string{"src/main/java/A.java"}}}})
			if err != nil { t.Fatal(err) }
			if result.Manifest.Status != "READY" || len(result.Manifest.Checks) != 0 || len(result.Manifest.Sources) != 1 || result.Manifest.Sources[0].Status != "NOT_APPLICABLE" { t.Fatalf("inactive rule became effective: %+v", result.Manifest) }
		})
	}

	missingApproval := []byte(`---
ruleId: R-1
projectId: p
status: ACTIVE
version: "1"
owner: o
source: s
approvalRef: ""
appliesTo: {paths: ['./'], entryPoints: []}
supersedes: []
exceptions: []
---
body
`)
	if _, _, err := ParseRule170(missingApproval); err == nil { t.Fatal("ACTIVE rule without approvalRef accepted") }

	metadataOnly := []byte(`---
ruleId: R-2
projectId: p
status: ACTIVE
version: "1"
owner: o
source: "https://example.invalid/must-not-fetch"
approvalRef: "$(must-not-execute)"
appliesTo: {paths: ['./'], entryPoints: []}
supersedes: []
exceptions: []
---
body
`)
	if rule, _, err := ParseRule170(metadataOnly); err != nil || rule.Source == "" || rule.ApprovalRef == "" { t.Fatalf("source/approvalRef must remain opaque metadata, rule=%+v err=%v", rule, err) }
}
