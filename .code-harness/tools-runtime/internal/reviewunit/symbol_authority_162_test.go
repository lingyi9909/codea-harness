package reviewunit

import (
	"strings"
	"testing"

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/changeset"
)

func Test162FullDuplicateModuleChainsBindExactFiles(t *testing.T) {
	facts := baseFacts160()
	moduleAController := "module-a/src/main/java/com/acme/UserController.java"
	moduleAService := "module-a/src/main/java/com/acme/UserService.java"
	moduleBController := "module-b/src/main/java/com/acme/UserController.java"
	moduleBService := "module-b/src/main/java/com/acme/UserService.java"
	paths := []string{moduleAController, moduleAService, moduleBController, moduleBService}

	facts.snapshot.Files = nil
	facts.analysis.ChangedFiles = nil
	facts.analysis.ReviewCoverage.ReviewedFiles = nil
	for _, p := range paths {
		facts.snapshot.Files = append(facts.snapshot.Files, changeset.File{Path: p, Status: "M", Sources: []changeset.Source{changeset.SourceCommitted}, Hunks: []changeset.Hunk{{OldStart: 1, OldLines: 1, NewStart: 1, NewLines: 8}}})
		role := "Service"
		if strings.Contains(p, "Controller") { role = "Controller" }
		facts.analysis.ChangedFiles = append(facts.analysis.ChangedFiles, analysisruntime.ChangedFile{Path: p, Role: role})
		facts.analysis.ReviewCoverage.ReviewedFiles = append(facts.analysis.ReviewCoverage.ReviewedFiles, analysisruntime.ChangedFile{Path: p, Role: role})
	}

	makeRef := func(path, symbol string) analysisruntime.SymbolRef {
		return analysisruntime.SymbolRef{Workspace: "current", Path: path, Symbol: symbol}
	}
	facts.analysis.CallChains = []analysisruntime.CallChain{
		{
			EntryPoint: "UserController.create",
			Chain: []string{"UserController.create", "UserService.create"},
			EntryPointRef: func() *analysisruntime.SymbolRef { r := makeRef(moduleAController, "UserController.create"); return &r }(),
			ChainRefs: []analysisruntime.SymbolRef{makeRef(moduleAController, "UserController.create"), makeRef(moduleAService, "UserService.create")},
		},
		{
			EntryPoint: "UserController.create",
			Chain: []string{"UserController.create", "UserService.create"},
			EntryPointRef: func() *analysisruntime.SymbolRef { r := makeRef(moduleBController, "UserController.create"); return &r }(),
			ChainRefs: []analysisruntime.SymbolRef{makeRef(moduleBController, "UserController.create"), makeRef(moduleBService, "UserService.create")},
		},
	}
	facts.analysis.SymbolLocations = []analysisruntime.SymbolLocation{
		{Workspace:"current", Symbol:"UserController.create", Path:moduleAController, Role:"Controller", Source:"FIND_SYMBOL"},
		{Workspace:"current", Symbol:"UserService.create", Path:moduleAService, Role:"Service", Source:"FIND_SYMBOL"},
		{Workspace:"current", Symbol:"UserController.create", Path:moduleBController, Role:"Controller", Source:"FIND_SYMBOL"},
		{Workspace:"current", Symbol:"UserService.create", Path:moduleBService, Role:"Service", Source:"FIND_SYMBOL"},
	}

	manifest, err := buildFromFacts160(facts)
	if err != nil { t.Fatalf("path-qualified duplicate chains must build: %v", err) }
	branches := make([]Unit, 0)
	for _, unit := range manifest.Units {
		if unit.EntryPoint == "UserController.create" { branches = append(branches, unit) }
	}
	if len(branches) != 2 { t.Fatalf("expected two distinct duplicate-symbol branch units, got %d: %+v", len(branches), manifest.Units) }
	for _, unit := range branches {
		hasA, hasB := false, false
		for _, file := range unit.Files {
			if strings.HasPrefix(file.Path, "module-a/") { hasA = true }
			if strings.HasPrefix(file.Path, "module-b/") { hasB = true }
		}
		if hasA == hasB { t.Fatalf("ReviewUnit crossed module authority boundary: %+v", unit.Files) }
	}
}
