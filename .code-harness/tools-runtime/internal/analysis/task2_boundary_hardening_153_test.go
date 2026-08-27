package analysis

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/changeset"
)

func Test153CertRejectsConfirmedEntrypointMissingAffectedController(t *testing.T) {
	a, inventory := validEvidence153()
	a.AffectedControllers = nil
	if err := validateEvidence153(a, inventory); err == nil || !strings.Contains(err.Error(), "ENTRYPOINT_ANALYSIS_INCONSISTENT") {
		t.Fatalf("confirmed expected entrypoint without affectedController must fail closed, got %v", err)
	}
}

func Test153CertRejectsConfirmedEntrypointMissingAffectedEndpoint(t *testing.T) {
	a, inventory := validEvidence153()
	a.AffectedControllers[0].Endpoints = []string{"AController.other"}
	if err := validateEvidence153(a, inventory); err == nil || !strings.Contains(err.Error(), "ENTRYPOINT_ANALYSIS_INCONSISTENT") {
		t.Fatalf("confirmed expected entrypoint missing from affectedController endpoints must fail closed, got %v", err)
	}
}

func Test153CertRejectsABCChainsWhenAffectedControllersOnlyContainsA(t *testing.T) {
	root := t.TempDir()
	installAnalysisCertificationContracts153(t, root)
	snapshot, inventory, draft := abcEntrypointConsistencyFixture153()
	draft["affectedControllers"] = []map[string]any{
		{"controller": "AController", "endpoints": []string{"AController.create"}, "impactType": "DIRECT_CHANGE", "sourceSymbols": []string{"AController.create"}},
	}
	writeCertificationDraft153(t, root, "r153", draft)

	_, err := certifyWithRuntime153(root, CertifyRequest{
		RunID: "r153", DraftPath: ".code-harness/runs/r153/requests/change-analysis-draft.json",
		BaseRef: "develop", IncludeWorkingTree: true, Intent: Intent{Mode: "FULL"},
	}, fakeCertificationRuntime153{snapshot: snapshot, inventory: inventory})
	if err == nil || !strings.Contains(err.Error(), "ENTRYPOINT_ANALYSIS_INCONSISTENT") {
		t.Fatalf("Inventory/callChains A/B/C with affectedControllers only A must fail closed, got %v", err)
	}
	assertNoAuthoritativeAnalysis153(t, root, "r153")
}

func Test153CertRejectsABCWhenBControllerEndpointMissingSubmit(t *testing.T) {
	root := t.TempDir()
	installAnalysisCertificationContracts153(t, root)
	snapshot, inventory, draft := abcEntrypointConsistencyFixture153()
	draft["affectedControllers"] = []map[string]any{
		{"controller": "AController", "endpoints": []string{"AController.create"}, "impactType": "DIRECT_CHANGE", "sourceSymbols": []string{"AController.create"}},
		{"controller": "BController", "endpoints": []string{"BController.other"}, "impactType": "DIRECT_CHANGE", "sourceSymbols": []string{"BController.submit"}},
		{"controller": "CController", "endpoints": []string{"CController.update"}, "impactType": "DIRECT_CHANGE", "sourceSymbols": []string{"CController.update"}},
	}
	writeCertificationDraft153(t, root, "r153", draft)

	_, err := certifyWithRuntime153(root, CertifyRequest{
		RunID: "r153", DraftPath: ".code-harness/runs/r153/requests/change-analysis-draft.json",
		BaseRef: "develop", IncludeWorkingTree: true, Intent: Intent{Mode: "FULL"},
	}, fakeCertificationRuntime153{snapshot: snapshot, inventory: inventory})
	if err == nil || !strings.Contains(err.Error(), "ENTRYPOINT_ANALYSIS_INCONSISTENT") || !strings.Contains(err.Error(), "BController.submit") {
		t.Fatalf("BController without BController.submit endpoint must fail closed with exact entrypoint, got %v", err)
	}
	assertNoAuthoritativeAnalysis153(t, root, "r153")
}

func Test153CertRejectsEmptyHeadCommitWithoutAuthoritativeWrite(t *testing.T) {
	root := t.TempDir()
	installAnalysisCertificationContracts153(t, root)

	snapshot := task153Snapshot([]changeset.File{{
		Path: "src/main/java/acme/AController.java", Status: "A", Sources: []changeset.Source{changeset.SourceStaged},
	}})
	inventory := EntrypointInventory{
		RunID: "r153", Status: "COMPLETE", ChangeSetSHA256: snapshot.SHA256,
		ExpectedEntrypoints: []ExpectedEntrypoint{{Symbol: "AController.create", Path: "src/main/java/acme/AController.java"}},
	}
	draft := validCertificationDraft153([]string{"src/main/java/acme/AController.java"})
	draft["reviewScope"].(map[string]any)["headCommit"] = ""
	writeCertificationDraft153(t, root, "r153", draft)

	_, err := certifyWithRuntime153(root, CertifyRequest{
		RunID: "r153", DraftPath: ".code-harness/runs/r153/requests/change-analysis-draft.json",
		BaseRef: "develop", IncludeWorkingTree: true, Intent: Intent{Mode: "FULL"},
	}, fakeCertificationRuntime153{snapshot: snapshot, inventory: inventory})
	if err == nil {
		t.Fatal("empty reviewScope.headCommit must fail closed")
	}
	assertNoAuthoritativeAnalysis153(t, root, "r153")
}

func Test153CertReverifiesWorkspaceEvidenceAndWritesNothingOnFailure(t *testing.T) {
	cases := []struct {
		name string
		setup workspaceCertificationSetup153
		want string
	}{
		{name: "NOT_CONFIGURED", setup: workspaceCertificationSetup153{configured: false, currentVersion: "1.0.0", sourceVersion: "1.0.0"}, want: "WORKSPACE_DEPENDENCY_NOT_CONFIGURED"},
		{name: "VERSION_UNRESOLVED", setup: workspaceCertificationSetup153{configured: true, currentVersion: "${missing.version}", sourceVersion: "1.0.0"}, want: "WORKSPACE_DEPENDENCY_VERSION_UNRESOLVED"},
		{name: "COORDINATE_MISMATCH", setup: workspaceCertificationSetup153{configured: true, currentArtifact: "other-framework", currentVersion: "1.0.0", sourceVersion: "1.0.0"}, want: "WORKSPACE_DEPENDENCY_COORDINATE_MISMATCH"},
		{name: "VERSION_MISMATCH", setup: workspaceCertificationSetup153{configured: true, currentVersion: "1.0.0", sourceVersion: "2.0.0"}, want: "WORKSPACE_DEPENDENCY_VERSION_MISMATCH"},
		{name: "SOURCE_NOT_FOUND", setup: workspaceCertificationSetup153{configured: true, currentVersion: "1.0.0", sourceVersion: ""}, want: "WORKSPACE_DEPENDENCY_SOURCE_NOT_FOUND"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := newWorkspaceCertificationFixture153(t, tc.setup)
			snapshot, inventory, draft := workspaceCertificationDraft153()
			writeCertificationDraft153(t, root, "r153", draft)
			_, err := certifyWithRuntime153(root, CertifyRequest{
				RunID: "r153", DraftPath: ".code-harness/runs/r153/requests/change-analysis-draft.json",
				BaseRef: "develop", IncludeWorkingTree: true, Intent: Intent{Mode: "FULL"},
			}, fakeCertificationRuntime153{snapshot: snapshot, inventory: inventory})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("workspace evidence must fail closed with %s, got %v", tc.want, err)
			}
			assertNoAuthoritativeAnalysis153(t, root, "r153")
		})
	}
}

func Test153WorkspaceIdentityAllowsSameRelativePathAcrossWorkspaces(t *testing.T) {
	root := newWorkspaceCertificationFixture153(t, workspaceCertificationSetup153{
		configured: true, currentVersion: "1.0.0", sourceVersion: "1.0.0", sameRelativePath: true,
	})
	snapshot, inventory, draft := workspaceCertificationDraft153()
	locations := draft["symbolLocations"].([]map[string]any)
	locations = append(locations, map[string]any{
		"workspace": "company-framework", "symbol": "FrameworkController.create",
		"path": "src/main/java/acme/AController.java", "role": "Service", "source": "WORKSPACE_INHERITANCE",
	})
	draft["symbolLocations"] = locations
	writeCertificationDraft153(t, root, "r153", draft)

	if _, err := certifyWithRuntime153(root, CertifyRequest{
		RunID: "r153", DraftPath: ".code-harness/runs/r153/requests/change-analysis-draft.json",
		BaseRef: "develop", IncludeWorkingTree: true, Intent: Intent{Mode: "FULL"},
	}, fakeCertificationRuntime153{snapshot: snapshot, inventory: inventory}); err != nil {
		t.Fatalf("same relative path in current and VERIFIED dependency workspace must not be treated as dependency scope leakage: %v", err)
	}
}

func abcEntrypointConsistencyFixture153() (changeset.Snapshot, EntrypointInventory, map[string]any) {
	paths := []string{
		"src/main/java/acme/AController.java",
		"src/main/java/acme/BController.java",
		"src/main/java/acme/CController.java",
	}
	snapshot := task153Snapshot([]changeset.File{
		{Path: paths[0], Status: "A", Sources: []changeset.Source{changeset.SourceStaged}},
		{Path: paths[1], Status: "A", Sources: []changeset.Source{changeset.SourceUntracked}},
		{Path: paths[2], Status: "M", Sources: []changeset.Source{changeset.SourceUnstaged}},
	})
	inventory := EntrypointInventory{
		RunID: "r153", Status: "COMPLETE", ChangeSetSHA256: snapshot.SHA256,
		ExpectedEntrypoints: []ExpectedEntrypoint{
			{Symbol: "AController.create", Path: paths[0]},
			{Symbol: "BController.submit", Path: paths[1]},
			{Symbol: "CController.update", Path: paths[2]},
		},
	}
	draft := validCertificationDraft153(paths)
	draft["callChains"] = []map[string]any{
		{"entryPoint": "AController.create", "chain": []string{"AController.create", "AService.create"}},
		{"entryPoint": "BController.submit", "chain": []string{"BController.submit", "BService.submit"}},
		{"entryPoint": "CController.update", "chain": []string{"CController.update", "CService.update"}},
	}
	draft["symbolLocations"] = []map[string]any{
		{"symbol": "AController.create", "path": paths[0], "role": "Controller", "source": "FIND_SYMBOL"},
		{"symbol": "AService.create", "path": "src/main/java/acme/AService.java", "role": "Service", "source": "FIND_SYMBOL"},
		{"symbol": "BController.submit", "path": paths[1], "role": "Controller", "source": "FIND_SYMBOL"},
		{"symbol": "BService.submit", "path": "src/main/java/acme/BService.java", "role": "Service", "source": "FIND_SYMBOL"},
		{"symbol": "CController.update", "path": paths[2], "role": "Controller", "source": "FIND_SYMBOL"},
		{"symbol": "CService.update", "path": "src/main/java/acme/CService.java", "role": "Service", "source": "FIND_SYMBOL"},
	}
	draft["affectedControllers"] = []map[string]any{
		{"controller": "AController", "endpoints": []string{"AController.create"}, "impactType": "DIRECT_CHANGE", "sourceSymbols": []string{"AController.create"}},
		{"controller": "BController", "endpoints": []string{"BController.submit"}, "impactType": "DIRECT_CHANGE", "sourceSymbols": []string{"BController.submit"}},
		{"controller": "CController", "endpoints": []string{"CController.update"}, "impactType": "DIRECT_CHANGE", "sourceSymbols": []string{"CController.update"}},
	}
	return snapshot, inventory, draft
}

type workspaceCertificationSetup153 struct {
	configured       bool
	currentArtifact  string
	currentVersion   string
	sourceVersion    string
	sameRelativePath bool
}

func newWorkspaceCertificationFixture153(t *testing.T, setup workspaceCertificationSetup153) string {
	t.Helper()
	parent := t.TempDir()
	root := filepath.Join(parent, "order-service")
	dep := filepath.Join(parent, "company-framework")
	if err := os.MkdirAll(root, 0o755); err != nil { t.Fatal(err) }
	if err := os.MkdirAll(dep, 0o755); err != nil { t.Fatal(err) }
	installAnalysisCertificationContracts153(t, root)

	artifact := setup.currentArtifact
	if artifact == "" { artifact = "company-framework" }
	currentPOM := fmt.Sprintf(`<project><modelVersion>4.0.0</modelVersion><groupId>com.company</groupId><artifactId>order-service</artifactId><version>1.0.0</version><dependencies><dependency><groupId>com.company</groupId><artifactId>%s</artifactId><version>%s</version></dependency></dependencies></project>`, artifact, setup.currentVersion)
	writeAnalysisFixtureFile153(t, filepath.Join(root, "pom.xml"), currentPOM)
	if setup.sourceVersion != "" {
		depPOM := fmt.Sprintf(`<project><modelVersion>4.0.0</modelVersion><groupId>com.company</groupId><artifactId>company-framework</artifactId><version>%s</version></project>`, setup.sourceVersion)
		writeAnalysisFixtureFile153(t, filepath.Join(dep, "pom.xml"), depPOM)
	}
	if setup.sameRelativePath {
		writeAnalysisFixtureFile153(t, filepath.Join(root, "src/main/java/acme/AController.java"), "class AController {}\n")
		writeAnalysisFixtureFile153(t, filepath.Join(dep, "src/main/java/acme/AController.java"), "class FrameworkController {}\n")
	}
	config := "version: 2\n"
	if setup.configured {
		config += "workspaceDependencies:\n  - id: company-framework\n    root: ../company-framework\n    maven:\n      groupId: com.company\n      artifactId: company-framework\n    mode: READ_ONLY\n"
	}
	writeAnalysisFixtureFile153(t, filepath.Join(root, ".code-harness", "harness.yaml"), config)
	return root
}

func workspaceCertificationDraft153() (changeset.Snapshot, EntrypointInventory, map[string]any) {
	snapshot := task153Snapshot([]changeset.File{{
		Path: "src/main/java/acme/AController.java", Status: "A", Sources: []changeset.Source{changeset.SourceStaged},
	}})
	inventory := EntrypointInventory{
		RunID: "r153", Status: "COMPLETE", ChangeSetSHA256: snapshot.SHA256,
		ExpectedEntrypoints: []ExpectedEntrypoint{{Symbol: "AController.create", Path: "src/main/java/acme/AController.java"}},
	}
	draft := validCertificationDraft153([]string{"src/main/java/acme/AController.java"})
	locations := draft["symbolLocations"].([]map[string]any)
	locations = append(locations, map[string]any{
		"workspace": "company-framework", "symbol": "AbstractTemplate.execute",
		"path": "src/main/java/com/company/framework/AbstractTemplate.java", "role": "Service", "source": "WORKSPACE_INHERITANCE",
	})
	draft["symbolLocations"] = locations
	return snapshot, inventory, draft
}

func installAnalysisCertificationContracts153(t *testing.T, root string) {
	t.Helper()
	copyAnalysisContract153(t, root, "change-analysis.schema.json")
	copyAnalysisContract153(t, root, "entrypoint-inventory.schema.json")
	copyAnalysisContract153(t, root, "change-analysis-cert.schema.json")
	writeAnalysisVersion153(t, root, "1.5.2")
}

func writeAnalysisVersion153(t *testing.T, root, version string) {
	t.Helper()
	p := filepath.Join(root, ".code-harness", "VERSION")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(p, []byte(version+"\n"), 0o644); err != nil { t.Fatal(err) }
}

func writeAnalysisFixtureFile153(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil { t.Fatal(err) }
}
