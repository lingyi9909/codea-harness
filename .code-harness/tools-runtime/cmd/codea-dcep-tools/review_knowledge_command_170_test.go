package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/knowledge"
	"codea-harness-tools/internal/reviewprogress"
	"codea-harness-tools/internal/reviewunit"
)

func Test170KnowledgeRequestIsRuntimeOwned(t *testing.T) {
	req, err := decodeReviewKnowledgeRequest170("run-1", []byte(`{"runId":"run-1"}`))
	if err != nil || req.RunID != "run-1" {
		t.Fatalf("valid request rejected: %+v %v", req, err)
	}
	for _, raw := range []string{
		`{"runId":"other"}`,
		`{"runId":"run-1","teamRoot":"C:/agent-choice"}`,
		`{"runId":"run-1"}{"runId":"run-1"}`,
	} {
		if _, err := decodeReviewKnowledgeRequest170("run-1", []byte(raw)); err == nil {
			t.Fatalf("agent-controlled/invalid request accepted: %s", raw)
		}
	}
}

func Test170KnowledgeRequiresReviewPlanning(t *testing.T) {
	valid := reviewprogress.State{
		ProtocolVersion: reviewprogress.Protocol170,
		Status:          reviewprogress.StatusRunning,
		CurrentStage:    reviewprogress.StageReviewPlanning,
	}
	if err := validateReviewKnowledgeProgress170(valid); err != nil {
		t.Fatal(err)
	}
	for _, state := range []reviewprogress.State{
		{ProtocolVersion: "", Status: reviewprogress.StatusRunning, CurrentStage: reviewprogress.StageReviewPlanning},
		{ProtocolVersion: reviewprogress.Protocol170, Status: reviewprogress.StatusRunning, CurrentStage: reviewprogress.StageCertification},
		{ProtocolVersion: reviewprogress.Protocol170, Status: reviewprogress.StatusSucceeded, CurrentStage: reviewprogress.StageReviewPlanning},
	} {
		if err := validateReviewKnowledgeProgress170(state); err == nil {
			t.Fatalf("illegal state accepted: %+v", state)
		}
	}
}

func Test170KnowledgeUnitsNeverInventMethodSignatures(t *testing.T) {
	manifest := reviewunit.Manifest{Units: []reviewunit.Unit{
		{
			ID:         "u-bare",
			EntryPoint: "RiskService.check",
			Files: []reviewunit.FileRef{
				{Path: "src/main/java/RiskService.java", Workspace: "current"},
				{Path: "providers/X.java", Workspace: "provider-x"},
			},
		},
		{
			ID:         "u-exact",
			EntryPoint: "com.acme.RiskService#check(java.lang.String)",
			Files:      []reviewunit.FileRef{{Path: "src/main/java/RiskService.java", Workspace: "current"}},
		},
	}}
	units := knowledgeUnits170(manifest)
	if len(units) != 2 {
		t.Fatalf("units=%+v", units)
	}
	if len(units[0].EntryPoints) != 0 {
		t.Fatalf("bare method was upgraded into exact signature: %+v", units[0])
	}
	if len(units[0].Paths) != 1 || units[0].Paths[0] != "src/main/java/RiskService.java" {
		t.Fatalf("provider path leaked into project knowledge applicability: %+v", units[0])
	}
	if len(units[1].EntryPoints) != 1 || units[1].EntryPoints[0] != "com.acme.RiskService#check(java.lang.String)" {
		t.Fatalf("exact identity lost: %+v", units[1])
	}
}

func Test170KnowledgeNotConfiguredIsTechnicalNoop(t *testing.T) {
	root := t.TempDir()
	loaded, err := loadReviewKnowledge170(root, "run-absent", []knowledge.Unit170{{ID: "u"}})
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Manifest.Status != "NOT_CONFIGURED" || loaded.Manifest.BindingSHA256 != "ABSENT" || loaded.Manifest.ProjectID != "UNCONFIGURED" {
		t.Fatalf("unexpected absent result: %+v", loaded.Manifest)
	}
	if len(loaded.Manifest.Sources) != 0 || len(loaded.Manifest.Checks) != 0 || len(loaded.Documents) != 0 {
		t.Fatalf("absent config must not fabricate knowledge: %+v", loaded)
	}
}

func Test170KnowledgeInvalidConfigIsManifestInvalidWithoutBroadScan(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".code-harness"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".code-harness", "context.yaml"), []byte("version: 1\nprojectId: p\nsources:\n  - {id: bad, root: PROJECT, path: '../escape.md', kind: RULES, required: true}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "escape.md"), []byte("must not be scanned"), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadReviewKnowledge170(root, "run-invalid", []knowledge.Unit170{{ID: "u"}})
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Manifest.Status != "INVALID" || loaded.Manifest.BindingSHA256 == "ABSENT" || len(loaded.Manifest.Issues) == 0 {
		t.Fatalf("invalid binding did not stay explicit: %+v", loaded.Manifest)
	}
	if len(loaded.Documents) != 0 || len(loaded.Manifest.Sources) != 0 {
		t.Fatalf("invalid binding triggered broad reads: %+v", loaded)
	}
}

func Test170KnowledgeConfiguredUsesOnlyExplicitSources(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".code-harness"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	binding := "version: 1\nprojectId: order-service\nsources:\n  - id: ref\n    root: PROJECT\n    path: docs/ref.md\n    kind: REFERENCE\n    required: true\n"
	if err := os.WriteFile(filepath.Join(root, ".code-harness", "context.yaml"), []byte(binding), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "ref.md"), []byte("bound reference\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "unbound.md"), []byte("must not be read\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadReviewKnowledge170(root, "run-ready", []knowledge.Unit170{{ID: "u", Paths: []string{"src/main/java/A.java"}}})
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Manifest.Status != "READY" || len(loaded.Documents) != 1 || loaded.Documents[0].SourceID != "ref" {
		t.Fatalf("unexpected configured result: %+v", loaded)
	}
	if strings.Contains(loaded.Documents[0].Content, "must not be read") {
		t.Fatal("unbound document leaked into knowledge")
	}
}

func Test170KnowledgeUseTimeDetectsSourceChange(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".code-harness"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	binding := "version: 1\nprojectId: order-service\nsources:\n  - id: ref\n    root: PROJECT\n    path: docs/ref.md\n    kind: REFERENCE\n    required: true\n"
	if err := os.WriteFile(filepath.Join(root, ".code-harness", "context.yaml"), []byte(binding), 0o644); err != nil {
		t.Fatal(err)
	}
	docPath := filepath.Join(root, "docs", "ref.md")
	if err := os.WriteFile(docPath, []byte("approved reference v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	units := []knowledge.Unit170{{ID: "u", Paths: []string{"src/main/java/A.java"}}}
	loaded, err := loadReviewKnowledge170(root, "run-fresh", units)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(docPath, []byte("approved reference v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyReviewKnowledgeUse170(root, "run-fresh", units, loaded.Manifest); err == nil || !strings.Contains(err.Error(), "KNOWLEDGE_SOURCE_CHANGED") {
		t.Fatalf("changed knowledge source passed use-time verification: %v", err)
	}
}

func Test170KnowledgeRouteIsRegistered(t *testing.T) {
	if err := runReview170([]string{"knowledge"}); err == nil || !strings.Contains(err.Error(), "requires --input") {
		t.Fatalf("review knowledge route missing or wrong: %v", err)
	}
}
