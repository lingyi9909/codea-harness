package knowledge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test170KnowledgeRequiredMustBeExplicitForEverySource(t *testing.T) {
	missing := map[string]string{
		"reference": "version: 1\nprojectId: p\nsources:\n  - id: ref\n    root: PROJECT\n    path: docs/ref.md\n    kind: REFERENCE\n",
		"experience": "version: 1\nprojectId: p\nsources:\n  - id: exp\n    root: PROJECT\n    path: docs/exp.md\n    kind: EXPERIENCE\n",
	}
	for name, raw := range missing {
		t.Run(name+"-missing", func(t *testing.T) {
			if _, _, err := ParseBinding170([]byte(raw)); err == nil {
				t.Fatalf("%s without explicit required was accepted", name)
			}
		})
	}

	for name, required := range map[string]string{"false": "false", "true": "true"} {
		t.Run("reference-required-"+name, func(t *testing.T) {
			raw := "version: 1\nprojectId: p\nsources:\n  - id: ref\n    root: PROJECT\n    path: docs/ref.md\n    kind: REFERENCE\n    required: " + required + "\n"
			binding, _, err := ParseBinding170([]byte(raw))
			if err != nil {
				t.Fatalf("explicit required=%s rejected: %v", required, err)
			}
			if len(binding.Sources) != 1 || binding.Sources[0].Required != (required == "true") {
				t.Fatalf("explicit required=%s not preserved: %+v", required, binding.Sources)
			}
		})
	}
}

func Test170KnowledgeOptionalBudgetPreservesDeclarationOrder(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	important := strings.Repeat("z", 160*1024)
	lowPriority := strings.Repeat("a", 160*1024)
	if err := os.WriteFile(filepath.Join(repo, "docs", "important.md"), []byte(important), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "docs", "low.md"), []byte(lowPriority), 0o644); err != nil {
		t.Fatal(err)
	}
	binding := Binding170{Version: 1, ProjectID: "p", Sources: []Source170{
		{ID: "z-important", Root: "PROJECT", Path: "docs/important.md", Kind: "REFERENCE", Required: false},
		{ID: "a-low-priority", Root: "PROJECT", Path: "docs/low.md", Kind: "REFERENCE", Required: false},
	}}
	result, err := Load170(LoadInput170{
		RunID:         "review-t7-order",
		RepoRoot:      repo,
		Binding:       binding,
		BindingSHA256: strings.Repeat("a", 64),
		Units:         []Unit170{{ID: "u", Paths: []string{"src/main/java/A.java"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Documents) != 1 {
		t.Fatalf("budget must admit exactly one optional document, got %+v", result.Documents)
	}
	if result.Documents[0].SourceID != "z-important" || result.Documents[0].Content != important {
		t.Fatalf("optional declaration order was not preserved: %+v", result.Documents)
	}
	var importantStatus, lowStatus string
	for _, source := range result.Manifest.Sources {
		switch source.SourceID {
		case "z-important":
			importantStatus = source.Status
		case "a-low-priority":
			lowStatus = source.Status
		}
	}
	if importantStatus != "READY" || lowStatus != "BLOCKED" {
		t.Fatalf("wrong optional source selected at budget boundary: %+v", result.Manifest.Sources)
	}
}
