package analysis

import (
	"context"
	"strings"
	"testing"

	"codea-harness-tools/internal/changeset"
)

type fake153EntrypointScanner struct {
	current map[string][]ControllerEndpoint
	base    map[string][]ControllerEndpoint
}

func (f fake153EntrypointScanner) Current(_ context.Context, path string) ([]ControllerEndpoint, error) {
	return append([]ControllerEndpoint(nil), f.current[path]...), nil
}

func (f fake153EntrypointScanner) Base(_ context.Context, _ changeset.Snapshot, path string) ([]ControllerEndpoint, error) {
	return append([]ControllerEndpoint(nil), f.base[path]...), nil
}

func Test153EntrypointInventoryFindsAllThreeChangedControllersWithoutNameGuessing(t *testing.T) {
	snap := changeset.Snapshot{
		BaseRef: "develop",
		Head:    "abc",
		SHA256:  strings.Repeat("a", 64),
		Files: []changeset.File{
			{Path: "src/main/java/acme/AController.java", Status: "A", Sources: []changeset.Source{changeset.SourceStaged}, Hunks: []changeset.Hunk{{NewStart: 1, NewLines: 30}}},
			{Path: "src/main/java/acme/BController.java", Status: "A", Sources: []changeset.Source{changeset.SourceUntracked}},
			{Path: "src/main/java/acme/CController.java", Status: "M", Sources: []changeset.Source{changeset.SourceUnstaged}, Hunks: []changeset.Hunk{{OldStart: 12, OldLines: 1, NewStart: 12, NewLines: 1}}},
			{Path: "src/main/java/acme/PlainService.java", Status: "M", Sources: []changeset.Source{changeset.SourceUnstaged}, Hunks: []changeset.Hunk{{NewStart: 2, NewLines: 1}}},
			{Path: "src/main/java/acme/FakeController.java", Status: "M", Sources: []changeset.Source{changeset.SourceUnstaged}, Hunks: []changeset.Hunk{{NewStart: 2, NewLines: 1}}},
		},
	}
	scanner := fake153EntrypointScanner{current: map[string][]ControllerEndpoint{
		"src/main/java/acme/AController.java": {{Controller: "AController", Symbol: "AController.create", Path: "src/main/java/acme/AController.java", ControllerStartLine: 1, ControllerEndLine: 30, StartLine: 10, EndLine: 15}},
		"src/main/java/acme/BController.java": {{Controller: "BController", Symbol: "BController.submit", Path: "src/main/java/acme/BController.java", ControllerStartLine: 1, ControllerEndLine: 30, StartLine: 10, EndLine: 15}},
		"src/main/java/acme/CController.java": {
			{Controller: "CController", Symbol: "CController.update", Path: "src/main/java/acme/CController.java", ControllerStartLine: 1, ControllerEndLine: 40, StartLine: 10, EndLine: 16},
			{Controller: "CController", Symbol: "CController.cancel", Path: "src/main/java/acme/CController.java", ControllerStartLine: 1, ControllerEndLine: 40, StartLine: 25, EndLine: 31},
		},
		// PlainService and FakeController intentionally have no annotation-backed controller evidence.
	}}

	got, err := buildEntrypointInventoryWithScanner(context.Background(), "r153", snap, Intent{Mode: "FULL"}, scanner)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"AController.create", "BController.submit", "CController.update"}
	assert153EntrypointSymbols(t, got, want)
	if got.Status != "COMPLETE" || got.ChangeSetSHA256 != snap.SHA256 {
		t.Fatalf("inventory identity=%+v", got)
	}
}

func Test153EntrypointInventoryClassLevelChangeRequiresAllControllerEndpoints(t *testing.T) {
	path := "src/main/java/acme/CController.java"
	snap := changeset.Snapshot{BaseRef: "develop", Head: "abc", SHA256: strings.Repeat("b", 64), Files: []changeset.File{{Path: path, Status: "M", Sources: []changeset.Source{changeset.SourceUnstaged}, Hunks: []changeset.Hunk{{OldStart: 5, OldLines: 1, NewStart: 5, NewLines: 1}}}}}
	scanner := fake153EntrypointScanner{current: map[string][]ControllerEndpoint{path: {
		{Controller: "CController", Symbol: "CController.update", Path: path, ControllerStartLine: 2, ControllerEndLine: 45, StartLine: 10, EndLine: 16},
		{Controller: "CController", Symbol: "CController.cancel", Path: path, ControllerStartLine: 2, ControllerEndLine: 45, StartLine: 25, EndLine: 31},
	}}}
	got, err := buildEntrypointInventoryWithScanner(context.Background(), "r153", snap, Intent{Mode: "FULL"}, scanner)
	if err != nil { t.Fatal(err) }
	assert153EntrypointSymbols(t, got, []string{"CController.cancel", "CController.update"})
}

func Test153EntrypointInventoryPureDeletionInsideExistingEndpointUsesOldSide(t *testing.T) {
	path := "src/main/java/acme/CController.java"
	snap := changeset.Snapshot{BaseRef: "develop", Head: "abc", SHA256: strings.Repeat("f", 64), Files: []changeset.File{{Path: path, Status: "M", Sources: []changeset.Source{changeset.SourceUnstaged}, Hunks: []changeset.Hunk{{OldStart: 13, OldLines: 1, NewStart: 13, NewLines: 0}}}}}
	scanner := fake153EntrypointScanner{
		current: map[string][]ControllerEndpoint{path: {{Controller: "CController", Symbol: "CController.update", Path: path, ControllerStartLine: 2, ControllerEndLine: 40, StartLine: 10, EndLine: 15}}},
		base: map[string][]ControllerEndpoint{path: {{Controller: "CController", Symbol: "CController.update", Path: path, ControllerStartLine: 2, ControllerEndLine: 41, StartLine: 10, EndLine: 16}}},
	}
	got, err := buildEntrypointInventoryWithScanner(context.Background(), "r153", snap, Intent{Mode: "FULL"}, scanner)
	if err != nil { t.Fatal(err) }
	assert153EntrypointSymbols(t, got, []string{"CController.update"})
	if got.ExpectedEntrypoints[0].Disposition != "" { t.Fatalf("existing endpoint must not be REMOVED: %+v", got.ExpectedEntrypoints[0]) }
}

func Test153EntrypointInventoryPureClassLevelDeletionRequiresAllCurrentEndpoints(t *testing.T) {
	path := "src/main/java/acme/CController.java"
	snap := changeset.Snapshot{BaseRef: "develop", Head: "abc", SHA256: strings.Repeat("9", 64), Files: []changeset.File{{Path: path, Status: "M", Sources: []changeset.Source{changeset.SourceUnstaged}, Hunks: []changeset.Hunk{{OldStart: 5, OldLines: 1, NewStart: 5, NewLines: 0}}}}}
	current := []ControllerEndpoint{
		{Controller: "CController", Symbol: "CController.update", Path: path, ControllerStartLine: 2, ControllerEndLine: 44, StartLine: 10, EndLine: 16},
		{Controller: "CController", Symbol: "CController.cancel", Path: path, ControllerStartLine: 2, ControllerEndLine: 44, StartLine: 25, EndLine: 31},
	}
	base := []ControllerEndpoint{
		{Controller: "CController", Symbol: "CController.update", Path: path, ControllerStartLine: 2, ControllerEndLine: 45, StartLine: 10, EndLine: 16},
		{Controller: "CController", Symbol: "CController.cancel", Path: path, ControllerStartLine: 2, ControllerEndLine: 45, StartLine: 25, EndLine: 31},
	}
	scanner := fake153EntrypointScanner{current: map[string][]ControllerEndpoint{path: current}, base: map[string][]ControllerEndpoint{path: base}}
	got, err := buildEntrypointInventoryWithScanner(context.Background(), "r153", snap, Intent{Mode: "FULL"}, scanner)
	if err != nil { t.Fatal(err) }
	assert153EntrypointSymbols(t, got, []string{"CController.cancel", "CController.update"})
}

func Test153EntrypointInventoryRepresentsDeletedEndpointAsRemoved(t *testing.T) {
	path := "src/main/java/acme/CController.java"
	snap := changeset.Snapshot{BaseRef: "develop", Head: "abc", SHA256: strings.Repeat("c", 64), Files: []changeset.File{{Path: path, Status: "M", Sources: []changeset.Source{changeset.SourceUnstaged}, Hunks: []changeset.Hunk{{OldStart: 20, OldLines: 6, NewStart: 20, NewLines: 0}}}}}
	scanner := fake153EntrypointScanner{
		current: map[string][]ControllerEndpoint{path: {{Controller: "CController", Symbol: "CController.update", Path: path, ControllerStartLine: 2, ControllerEndLine: 35, StartLine: 10, EndLine: 16}}},
		base: map[string][]ControllerEndpoint{path: {
			{Controller: "CController", Symbol: "CController.update", Path: path, ControllerStartLine: 2, ControllerEndLine: 42, StartLine: 10, EndLine: 16},
			{Controller: "CController", Symbol: "CController.remove", Path: path, ControllerStartLine: 2, ControllerEndLine: 42, StartLine: 20, EndLine: 25},
		}},
	}
	got, err := buildEntrypointInventoryWithScanner(context.Background(), "r153", snap, Intent{Mode: "FULL"}, scanner)
	if err != nil { t.Fatal(err) }
	if len(got.ExpectedEntrypoints) != 1 || got.ExpectedEntrypoints[0].Symbol != "CController.remove" || got.ExpectedEntrypoints[0].Disposition != DispositionRemoved {
		t.Fatalf("deleted endpoint disposition=%+v", got.ExpectedEntrypoints)
	}
}

func Test153EntrypointInventoryTargetedRequiresOnlyExactTarget(t *testing.T) {
	aPath := "src/main/java/acme/AController.java"
	bPath := "src/main/java/acme/BController.java"
	snap := changeset.Snapshot{BaseRef: "develop", Head: "abc", SHA256: strings.Repeat("d", 64), Files: []changeset.File{
		{Path: aPath, Status: "A", Sources: []changeset.Source{changeset.SourceStaged}},
		{Path: bPath, Status: "A", Sources: []changeset.Source{changeset.SourceStaged}},
	}}
	scanner := fake153EntrypointScanner{current: map[string][]ControllerEndpoint{
		aPath: {{Controller: "AController", Symbol: "AController.create", Path: aPath, ControllerStartLine: 1, ControllerEndLine: 20, StartLine: 5, EndLine: 10}},
		bPath: {{Controller: "BController", Symbol: "BController.submit", Path: bPath, ControllerStartLine: 1, ControllerEndLine: 20, StartLine: 5, EndLine: 10}},
	}}
	got, err := buildEntrypointInventoryWithScanner(context.Background(), "r153", snap, Intent{Mode: "TARGETED", Target: "BController.submit"}, scanner)
	if err != nil { t.Fatal(err) }
	assert153EntrypointSymbols(t, got, []string{"BController.submit"})
}

func Test153VerifyEntrypointDispositionsRejectsSilentMissing(t *testing.T) {
	inventory := EntrypointInventory{RunID: "r153", Status: "COMPLETE", ChangeSetSHA256: strings.Repeat("e", 64), ExpectedEntrypoints: []ExpectedEntrypoint{
		{Symbol: "AController.create", Path: "src/main/java/acme/AController.java"},
		{Symbol: "BController.submit", Path: "src/main/java/acme/BController.java"},
		{Symbol: "CController.update", Path: "src/main/java/acme/CController.java"},
	}}
	proposal := ChangeAnalysis{
		CallChains: []CallChain{{EntryPoint: "AController.create", Chain: []string{"AController.create", "AService.create"}}},
		ReviewCoverage: ReviewCoverage{Status: "PARTIAL", UnresolvedSymbols: []UnresolvedSymbol{{Symbol: "BService.submit", From: "BController.submit", Reason: "IMPLEMENTATION_NOT_FOUND"}}},
	}
	err := VerifyEntrypointDispositions(inventory, proposal)
	if err == nil || !strings.Contains(err.Error(), "ENTRYPOINT_COMPLETENESS_INCOMPLETE") || !strings.Contains(err.Error(), "CController.update") {
		t.Fatalf("silent missing entrypoint must fail with exact symbol, got %v", err)
	}

	proposal.CallChains = append(proposal.CallChains, CallChain{EntryPoint: "CController.update", Chain: []string{"CController.update", "CService.update"}})
	if err := VerifyEntrypointDispositions(inventory, proposal); err != nil {
		t.Fatalf("CONFIRMED/PARTIAL coverage should pass: %v", err)
	}
}

func assert153EntrypointSymbols(t *testing.T, got EntrypointInventory, want []string) {
	t.Helper()
	if len(got.ExpectedEntrypoints) != len(want) {
		t.Fatalf("entrypoints=%+v want=%v", got.ExpectedEntrypoints, want)
	}
	for i, symbol := range want {
		if got.ExpectedEntrypoints[i].Symbol != symbol {
			t.Fatalf("entrypoints[%d]=%s want %s; all=%+v", i, got.ExpectedEntrypoints[i].Symbol, symbol, got.ExpectedEntrypoints)
		}
	}
}
