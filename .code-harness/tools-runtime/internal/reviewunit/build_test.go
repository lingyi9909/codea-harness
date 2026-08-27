package reviewunit

import (
	"strings"
	"testing"

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/changeset"
	"codea-harness-tools/internal/reviewscope"
)

const (
	controllerPath160 = "src/main/java/acme/OrderController.java"
	servicePath160    = "src/main/java/acme/OrderServiceImpl.java"
	configPath160     = "src/main/resources/application.yml"
)

func TestBuildFullCreatesBranchUnitsAndStandaloneChangedFile(t *testing.T) {
	facts := baseFacts160()
	manifest, err := buildFromFacts160(facts)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Mode != ModeFull || len(manifest.Units) != 2 {
		t.Fatalf("FULL must create one branch unit plus one standalone changed file: %+v", manifest)
	}
	covered := map[string]bool{}
	var branch, standalone *Unit
	for i := range manifest.Units {
		u := &manifest.Units[i]
		for _, file := range u.Files {
			covered[file.Path] = true
		}
		if u.EntryPoint == "OrderController.approve" {
			branch = u
		}
		if strings.HasPrefix(u.ID, "RU-FILE-") {
			standalone = u
		}
	}
	if branch == nil || standalone == nil {
		t.Fatalf("expected branch + RU-FILE standalone units: %+v", manifest.Units)
	}
	if len(branch.ChangedHunks) == 0 {
		t.Fatal("branch unit must carry real Change Set hunks for changed files")
	}
	for _, path := range []string{controllerPath160, servicePath160, configPath160} {
		if !covered[path] {
			t.Fatalf("FULL Finding-scope file %s was silently omitted", path)
		}
	}
	if len(standalone.Files) != 1 || standalone.Files[0].Path != configPath160 || !standalone.Files[0].Changed {
		t.Fatalf("unbound changed config must become deterministic standalone unit: %+v", standalone)
	}
}

func TestBuildTargetedUsesOnlyVerifiedScopedFiles(t *testing.T) {
	facts := baseFacts160()
	adminController := "src/main/java/acme/AdminController.java"
	adminService := "src/main/java/acme/AdminService.java"
	facts.analysis.CallChains = append(facts.analysis.CallChains, analysisruntime.CallChain{
		EntryPoint: "AdminController.update",
		Chain:      []string{"AdminController.update", "AdminService.update"},
	})
	facts.analysis.SymbolLocations = append(facts.analysis.SymbolLocations,
		analysisruntime.SymbolLocation{Symbol: "AdminController.update", Path: adminController, Role: "Controller", Source: "FIND_SYMBOL"},
		analysisruntime.SymbolLocation{Symbol: "AdminService.update", Path: adminService, Role: "Service", Source: "FIND_SYMBOL"},
	)
	facts.analysis.ReviewCoverage.ReviewedFiles = append(facts.analysis.ReviewCoverage.ReviewedFiles,
		analysisruntime.ChangedFile{Path: adminController, Role: "Controller"},
		analysisruntime.ChangedFile{Path: adminService, Role: "Service"},
	)
	facts.scope = reviewscope.Selection{
		Mode: "TARGETED",
		Target: &reviewscope.Target{Symbol: "OrderController.approve", Kind: "METHOD"},
		SelectedCallChains: []reviewscope.CallChain{{
			EntryPoint: "OrderController.approve",
			Chain:      []string{"OrderController.approve", "OrderServiceImpl.approve"},
		}},
		ScopedFiles: []string{controllerPath160, servicePath160},
	}

	manifest, err := buildFromFacts160(facts)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Mode != ModeTargeted || len(manifest.Units) != 1 {
		t.Fatalf("TARGETED must contain only selected branch, got %+v", manifest)
	}
	for _, file := range manifest.Units[0].Files {
		if file.Path != controllerPath160 && file.Path != servicePath160 {
			t.Fatalf("TARGETED leaked scope-out file %s", file.Path)
		}
	}
	if manifest.Units[0].EntryPoint != "OrderController.approve" {
		t.Fatalf("wrong targeted branch: %+v", manifest.Units[0])
	}
}

func TestBuildDistinctBranchesHaveDistinctDeterministicIDs(t *testing.T) {
	facts := baseFacts160()
	facts.analysis.CallChains = append(facts.analysis.CallChains, analysisruntime.CallChain{
		EntryPoint: "OrderController.approve",
		Chain:      []string{"OrderController.approve", "OrderServiceImpl.approveAlternative"},
	})
	facts.analysis.SymbolLocations = append(facts.analysis.SymbolLocations,
		analysisruntime.SymbolLocation{Symbol: "OrderServiceImpl.approveAlternative", Path: servicePath160, Role: "Service", Source: "FIND_SYMBOL"},
	)

	first, err := buildFromFacts160(facts)
	if err != nil {
		t.Fatal(err)
	}
	second, err := buildFromFacts160(facts)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, unit := range first.Units {
		if unit.EntryPoint == "OrderController.approve" {
			ids = append(ids, unit.ID)
		}
	}
	if len(ids) != 2 || ids[0] == ids[1] {
		t.Fatalf("distinct verified core branches need distinct IDs: %v", ids)
	}
	if first.SHA256 != second.SHA256 {
		t.Fatalf("same facts must reproduce deterministic manifest SHA: %s != %s", first.SHA256, second.SHA256)
	}
}

func TestBuildCanonicalizesDuplicateCoreBranch(t *testing.T) {
	facts := baseFacts160()
	facts.analysis.CallChains = append(facts.analysis.CallChains, facts.analysis.CallChains[0])
	manifest, err := buildFromFacts160(facts)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, unit := range manifest.Units {
		if unit.EntryPoint == "OrderController.approve" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("duplicate exact core branch must canonicalize to one unit, count=%d units=%+v", count, manifest.Units)
	}
}

func TestBuildRejectsDependencyPathInFiles(t *testing.T) {
	facts := baseFacts160()
	dependencyPath := "src/main/java/com/company/framework/AbstractTemplate.java"
	chain := []string{"OrderController.approve", "AbstractTemplate.execute"}
	facts.analysis.CallChains = []analysisruntime.CallChain{{EntryPoint: "OrderController.approve", Chain: chain}}
	facts.analysis.SymbolLocations = []analysisruntime.SymbolLocation{
		{Symbol: "OrderController.approve", Path: controllerPath160, Role: "Controller", Source: "FIND_SYMBOL"},
		{Workspace: "company-framework", Symbol: "AbstractTemplate.execute", Path: dependencyPath, Role: "Service", Source: "WORKSPACE_INHERITANCE"},
	}
	facts.scope = reviewscope.Selection{
		Mode:               "TARGETED",
		Target:             &reviewscope.Target{Symbol: "OrderController.approve", Kind: "METHOD"},
		SelectedCallChains: []reviewscope.CallChain{{EntryPoint: "OrderController.approve", Chain: chain}},
		ScopedFiles:        []string{controllerPath160, dependencyPath},
	}
	if _, err := buildFromFacts160(facts); err == nil || !strings.Contains(err.Error(), "REVIEW_UNIT_SCOPE_VIOLATION") {
		t.Fatalf("dependency workspace path in files must fail closed, got %v", err)
	}
}

func TestCanonicalBytesStableAcrossInputOrder(t *testing.T) {
	left := baseFacts160()
	right := baseFacts160()
	reverseAnalysis160(&right.analysis)
	for i := range right.snapshot.Files {
		reverseHunks160(right.snapshot.Files[i].Hunks)
	}
	reverseFiles160(right.snapshot.Files)

	leftManifest, err := buildFromFacts160(left)
	if err != nil {
		t.Fatal(err)
	}
	rightManifest, err := buildFromFacts160(right)
	if err != nil {
		t.Fatal(err)
	}
	leftBytes, err := CanonicalBytes(leftManifest)
	if err != nil {
		t.Fatal(err)
	}
	rightBytes, err := CanonicalBytes(rightManifest)
	if err != nil {
		t.Fatal(err)
	}
	if string(leftBytes) != string(rightBytes) {
		t.Fatalf("canonical review units changed with input ordering\nLEFT:\n%s\nRIGHT:\n%s", leftBytes, rightBytes)
	}
}

func baseFacts160() buildFacts160 {
	snapshot := changeset.Snapshot{
		BaseRef: "main",
		Head:    "head160",
		SHA256:  strings.Repeat("c", 64),
		Files: []changeset.File{
			{Path: controllerPath160, Status: "M", Sources: []changeset.Source{changeset.SourceCommitted}, Hunks: []changeset.Hunk{{OldStart: 10, OldLines: 1, NewStart: 10, NewLines: 2}}},
			{Path: servicePath160, Status: "M", Sources: []changeset.Source{changeset.SourceCommitted}, Hunks: []changeset.Hunk{{OldStart: 20, OldLines: 1, NewStart: 20, NewLines: 3}}},
			{Path: configPath160, Status: "M", Sources: []changeset.Source{changeset.SourceCommitted}, Hunks: []changeset.Hunk{{OldStart: 3, OldLines: 1, NewStart: 3, NewLines: 1}}},
		},
	}
	analysis := analysisruntime.ChangeAnalysis{
		ChangedFiles: []analysisruntime.ChangedFile{
			{Path: controllerPath160, Role: "Controller"},
			{Path: servicePath160, Role: "Service"},
			{Path: configPath160, Role: "YamlConfig"},
		},
		CallChains: []analysisruntime.CallChain{{
			EntryPoint: "OrderController.approve",
			Chain:      []string{"OrderController.approve", "OrderServiceImpl.approve"},
		}},
		SymbolLocations: []analysisruntime.SymbolLocation{
			{Symbol: "OrderController.approve", Path: controllerPath160, Role: "Controller", Source: "FIND_SYMBOL"},
			{Symbol: "OrderServiceImpl.approve", Path: servicePath160, Role: "Service", Source: "FIND_SYMBOL"},
		},
		ReviewCoverage: analysisruntime.ReviewCoverage{
			Status: "COMPLETE",
			ReviewedFiles: []analysisruntime.ChangedFile{
				{Path: controllerPath160, Role: "Controller"},
				{Path: servicePath160, Role: "Service"},
				{Path: configPath160, Role: "YamlConfig"},
			},
		},
	}
	return buildFacts160{
		runID:          "run160",
		harnessVersion: "1.5.3",
		analysis:       analysis,
		cert: analysisruntime.Certificate{
			RunID:           "run160",
			RuntimeVersion:  "1.5.3",
			AnalysisSHA256:  strings.Repeat("a", 64),
			ChangeSetSHA256: snapshot.SHA256,
			BaseRef:         snapshot.BaseRef,
			Head:            snapshot.Head,
		},
		scope: reviewscope.Selection{Mode: "FULL", SelectedCallChains: []reviewscope.CallChain{}, ScopedFiles: []string{}},
		scopeSHA256: strings.Repeat("b", 64),
		snapshot:    snapshot,
	}
}

func reverseAnalysis160(a *analysisruntime.ChangeAnalysis) {
	for i, j := 0, len(a.ChangedFiles)-1; i < j; i, j = i+1, j-1 { a.ChangedFiles[i], a.ChangedFiles[j] = a.ChangedFiles[j], a.ChangedFiles[i] }
	for i, j := 0, len(a.CallChains)-1; i < j; i, j = i+1, j-1 { a.CallChains[i], a.CallChains[j] = a.CallChains[j], a.CallChains[i] }
	for i, j := 0, len(a.SymbolLocations)-1; i < j; i, j = i+1, j-1 { a.SymbolLocations[i], a.SymbolLocations[j] = a.SymbolLocations[j], a.SymbolLocations[i] }
	for i, j := 0, len(a.ResourceRelations)-1; i < j; i, j = i+1, j-1 { a.ResourceRelations[i], a.ResourceRelations[j] = a.ResourceRelations[j], a.ResourceRelations[i] }
	for i, j := 0, len(a.ReviewCoverage.ReviewedFiles)-1; i < j; i, j = i+1, j-1 { a.ReviewCoverage.ReviewedFiles[i], a.ReviewCoverage.ReviewedFiles[j] = a.ReviewCoverage.ReviewedFiles[j], a.ReviewCoverage.ReviewedFiles[i] }
}

func reverseFiles160(files []changeset.File) {
	for i, j := 0, len(files)-1; i < j; i, j = i+1, j-1 { files[i], files[j] = files[j], files[i] }
}

func reverseHunks160(hunks []changeset.Hunk) {
	for i, j := 0, len(hunks)-1; i < j; i, j = i+1, j-1 { hunks[i], hunks[j] = hunks[j], hunks[i] }
}
