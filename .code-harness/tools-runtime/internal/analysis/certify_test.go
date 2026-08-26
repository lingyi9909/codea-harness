package analysis

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"codea-harness-tools/internal/changeset"
)

type fakeCertificationRuntime153 struct {
	snapshot  changeset.Snapshot
	inventory EntrypointInventory
}

func (f fakeCertificationRuntime153) Compute(_ string, _ string, _ bool) (changeset.Snapshot, error) {
	return f.snapshot, nil
}

func (f fakeCertificationRuntime153) Inventory(_ string, _ string, _ changeset.Snapshot, _ Intent) (EntrypointInventory, error) {
	return f.inventory, nil
}

func Test153CertRejectsIncompleteEntrypointDraft(t *testing.T) {
	root := t.TempDir()
	copyAnalysisContract153(t, root, "change-analysis.schema.json")
	copyAnalysisContract153(t, root, "entrypoint-inventory.schema.json")

	snapshot := task153Snapshot([]changeset.File{
		{Path: "src/main/java/acme/AController.java", Status: "A", Sources: []changeset.Source{changeset.SourceStaged}},
		{Path: "src/main/java/acme/BController.java", Status: "A", Sources: []changeset.Source{changeset.SourceUntracked}},
		{Path: "src/main/java/acme/CController.java", Status: "M", Sources: []changeset.Source{changeset.SourceUnstaged}},
	})
	inventory := EntrypointInventory{RunID: "r153", Status: "COMPLETE", ChangeSetSHA256: snapshot.SHA256, ExpectedEntrypoints: []ExpectedEntrypoint{
		{Symbol: "AController.create", Path: "src/main/java/acme/AController.java"},
		{Symbol: "BController.submit", Path: "src/main/java/acme/BController.java"},
		{Symbol: "CController.update", Path: "src/main/java/acme/CController.java"},
	}}

	draft := validCertificationDraft153([]string{
		"src/main/java/acme/AController.java",
		"src/main/java/acme/BController.java",
		"src/main/java/acme/CController.java",
	})
	writeCertificationDraft153(t, root, "r153", draft)

	_, err := certifyWithRuntime153(root, CertifyRequest{
		RunID: "r153", DraftPath: ".code-harness/runs/r153/requests/change-analysis-draft.json",
		BaseRef: "develop", IncludeWorkingTree: true, Intent: Intent{Mode: "FULL"},
	}, fakeCertificationRuntime153{snapshot: snapshot, inventory: inventory})
	if err == nil || !strings.Contains(err.Error(), "ENTRYPOINT_COMPLETENESS_INCOMPLETE") || !strings.Contains(err.Error(), "BController.submit") || !strings.Contains(err.Error(), "CController.update") {
		t.Fatalf("incomplete draft must fail with both missing entrypoints, got %v", err)
	}
	assertNoAuthoritativeAnalysis153(t, root, "r153")
}

func Test153CertRejectsExactChangeSetMismatch(t *testing.T) {
	root := t.TempDir()
	copyAnalysisContract153(t, root, "change-analysis.schema.json")
	copyAnalysisContract153(t, root, "entrypoint-inventory.schema.json")

	snapshot := task153Snapshot([]changeset.File{
		{Path: "src/main/java/acme/AController.java", Status: "A", Sources: []changeset.Source{changeset.SourceStaged}},
		{Path: "src/main/java/acme/BController.java", Status: "A", Sources: []changeset.Source{changeset.SourceUntracked}},
		{Path: "src/main/java/acme/CController.java", Status: "M", Sources: []changeset.Source{changeset.SourceUnstaged}},
	})
	inventory := EntrypointInventory{RunID: "r153", Status: "COMPLETE", ChangeSetSHA256: snapshot.SHA256}
	draft := validCertificationDraft153([]string{
		"src/main/java/acme/AController.java",
		"src/main/java/acme/BController.java",
	})
	writeCertificationDraft153(t, root, "r153", draft)

	_, err := certifyWithRuntime153(root, CertifyRequest{
		RunID: "r153", DraftPath: ".code-harness/runs/r153/requests/change-analysis-draft.json",
		BaseRef: "develop", IncludeWorkingTree: true, Intent: Intent{Mode: "FULL"},
	}, fakeCertificationRuntime153{snapshot: snapshot, inventory: inventory})
	if err == nil || !strings.Contains(err.Error(), "CHANGE_SET_MISMATCH") {
		t.Fatalf("exact Change Set mismatch must fail closed, got %v", err)
	}
	assertNoAuthoritativeAnalysis153(t, root, "r153")
}

func Test153EvidenceRejectsMissingConfirmedControllerFact(t *testing.T) {
	a, inventory := validEvidence153()
	a.SymbolLocations = []SymbolLocation{{Symbol: "AService.create", Path: "src/main/java/acme/AService.java", Role: "Service", Source: "FIND_SYMBOL"}}
	if err := validateEvidence153(a, inventory); err == nil || !strings.Contains(err.Error(), "ENTRYPOINT_EVIDENCE_MISSING") {
		t.Fatalf("confirmed entrypoint without exact Controller fact must fail, got %v", err)
	}
}

func Test153EvidenceRejectsCallChainNodeWithoutLocation(t *testing.T) {
	a, inventory := validEvidence153()
	a.SymbolLocations = a.SymbolLocations[:1]
	if err := validateEvidence153(a, inventory); err == nil || !strings.Contains(err.Error(), "CALL_CHAIN_EVIDENCE_MISSING") {
		t.Fatalf("callChain node without symbolLocation must fail, got %v", err)
	}
}

func Test153EvidenceRejectsConflictingSameSymbolFacts(t *testing.T) {
	a, inventory := validEvidence153()
	a.SymbolLocations = append(a.SymbolLocations, SymbolLocation{Symbol: "AService.create", Path: "src/main/java/other/AService.java", Role: "Service", Source: "FIND_SYMBOL"})
	if err := validateEvidence153(a, inventory); err == nil || !strings.Contains(err.Error(), "SYMBOL_LOCATION_CONFLICT") {
		t.Fatalf("conflicting symbol facts must fail, got %v", err)
	}
}

func Test153EvidenceRejectsDependencyWorkspaceInChangedOrReviewedFiles(t *testing.T) {
	a, inventory := validEvidence153()
	dep := "src/main/java/com/company/framework/AbstractTemplate.java"
	a.SymbolLocations = append(a.SymbolLocations, SymbolLocation{Workspace: "company-framework", Symbol: "AbstractTemplate.execute", Path: dep, Role: "Service", Source: "WORKSPACE_INHERITANCE"})
	a.ChangedFiles = append(a.ChangedFiles, ChangedFile{Path: dep, Role: "Service"})
	a.ReviewCoverage.ReviewedFiles = append(a.ReviewCoverage.ReviewedFiles, ChangedFile{Path: dep, Role: "Service"})
	if err := validateEvidence153(a, inventory); err == nil || !strings.Contains(err.Error(), "WORKSPACE_DEPENDENCY_SCOPE_VIOLATION") {
		t.Fatalf("dependency workspace cannot become changed/reviewed scope, got %v", err)
	}
}

func Test153EvidenceRejectsInvalidMapperRelation(t *testing.T) {
	a, inventory := validEvidence153()
	a.ResourceRelations = []ResourceRelation{{
		Path: "src/main/resources/mapper/NotMapper.yml", Role: "MapperXml", Resource: "AMapper.xml#update",
		FromSymbol: "AService.create", FromKind: "METHOD", Source: "CONFIG_REFERENCE", Evidence: "bad",
	}}
	if err := validateEvidence153(a, inventory); err == nil || !strings.Contains(err.Error(), "RESOURCE_RELATION_INVALID") {
		t.Fatalf("invalid Mapper relation must fail, got %v", err)
	}
}

func Test153EvidenceRejectsUnresolvedEntrypointWithoutLimitationCode(t *testing.T) {
	a, inventory := validEvidence153()
	inventory.ExpectedEntrypoints = append(inventory.ExpectedEntrypoints, ExpectedEntrypoint{Symbol: "BController.submit", Path: "src/main/java/acme/BController.java"})
	a.ReviewCoverage.UnresolvedSymbols = []UnresolvedSymbol{{Symbol: "BController.submit", From: "BController.submit", Reason: ""}}
	if err := validateEvidence153(a, inventory); err == nil || !strings.Contains(err.Error(), "UNRESOLVED_LIMITATION_REQUIRED") {
		t.Fatalf("unresolved entrypoint requires explicit limitation code, got %v", err)
	}
}

func task153Snapshot(files []changeset.File) changeset.Snapshot {
	return changeset.Snapshot{BaseRef: "develop", Head: "head153", Files: files, SHA256: strings.Repeat("a", 64)}
}

func validEvidence153() (ChangeAnalysis, EntrypointInventory) {
	a := ChangeAnalysis{
		ChangedFiles: []ChangedFile{{Path: "src/main/java/acme/AController.java", Role: "Controller"}},
		AffectedControllers: []AffectedController{{Controller: "AController", Endpoints: []string{"AController.create"}, ImpactType: "DIRECT_CHANGE", SourceSymbols: []string{"AController.create"}}},
		CallChains: []CallChain{{EntryPoint: "AController.create", Chain: []string{"AController.create", "AService.create"}}},
		SymbolLocations: []SymbolLocation{
			{Symbol: "AController.create", Path: "src/main/java/acme/AController.java", Role: "Controller", Source: "FIND_SYMBOL"},
			{Symbol: "AService.create", Path: "src/main/java/acme/AService.java", Role: "Service", Source: "FIND_SYMBOL"},
		},
		ReviewCoverage: ReviewCoverage{Status: "COMPLETE", ReviewedFiles: []ChangedFile{{Path: "src/main/java/acme/AController.java", Role: "Controller"}}},
	}
	inventory := EntrypointInventory{RunID: "r153", Status: "COMPLETE", ChangeSetSHA256: strings.Repeat("a", 64), ExpectedEntrypoints: []ExpectedEntrypoint{{Symbol: "AController.create", Path: "src/main/java/acme/AController.java"}}}
	return a, inventory
}

func validCertificationDraft153(paths []string) map[string]any {
	changed := make([]map[string]any, 0, len(paths))
	reviewed := make([]map[string]any, 0, len(paths))
	for _, p := range paths {
		source := "STAGED"
		switch {
		case strings.Contains(p, "BController"):
			source = "UNTRACKED"
		case strings.Contains(p, "CController"):
			source = "UNSTAGED"
		}
		changed = append(changed, map[string]any{"path": p, "role": "Controller", "sources": []string{source}})
		reviewed = append(reviewed, map[string]any{"path": p, "role": "Controller", "reason": "CHANGED"})
	}
	return map[string]any{
		"reviewScope": map[string]any{
			"currentBranch": "feature", "baseRef": "develop", "baseCommit": "base153", "mergeBase": "base153", "headCommit": "head153", "includeWorkingTree": true,
		},
		"changedFiles": changed,
		"affectedControllers": []map[string]any{{"controller": "AController", "endpoints": []string{"AController.create"}, "impactType": "DIRECT_CHANGE", "sourceSymbols": []string{"AController.create"}}},
		"callChains": []map[string]any{{"entryPoint": "AController.create", "chain": []string{"AController.create", "AService.create"}}},
		"symbolLocations": []map[string]any{
			{"symbol": "AController.create", "path": "src/main/java/acme/AController.java", "role": "Controller", "source": "FIND_SYMBOL"},
			{"symbol": "AService.create", "path": "src/main/java/acme/AService.java", "role": "Service", "source": "FIND_SYMBOL"},
		},
		"resourceRelations": []any{}, "externalDependencies": []any{}, "riskAreas": []any{},
		"reviewCoverage": map[string]any{"status": "COMPLETE", "reviewedFiles": reviewed, "unresolvedSymbols": []any{}},
	}
}

func writeCertificationDraft153(t *testing.T, root, runID string, document map[string]any) {
	t.Helper()
	b, err := json.MarshalIndent(document, "", "  ")
	if err != nil { t.Fatal(err) }
	path := filepath.Join(root, ".code-harness", "runs", runID, "requests", "change-analysis-draft.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil { t.Fatal(err) }
}

func copyAnalysisContract153(t *testing.T, root, name string) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok { t.Fatal("runtime.Caller failed") }
	source := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "contracts", name))
	b, err := os.ReadFile(source)
	if err != nil { t.Fatalf("read contract %s: %v", name, err) }
	dst := filepath.Join(root, ".code-harness", "contracts", name)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(dst, b, 0o644); err != nil { t.Fatal(err) }
}

func assertNoAuthoritativeAnalysis153(t *testing.T, root, runID string) {
	t.Helper()
	for _, name := range []string{"change-analysis.json", "entrypoint-inventory.json", "change-analysis.cert.json"} {
		p := filepath.Join(root, ".code-harness", "runs", runID, "analysis", name)
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("authoritative artifact %s must not exist after failed certification; stat=%v", name, err)
		}
	}
}
