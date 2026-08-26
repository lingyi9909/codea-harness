package analysis

import (
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

func Test153CertRejectsEmptyHeadCommitWithoutAuthoritativeWrite(t *testing.T) {
	root := t.TempDir()
	copyAnalysisContract153(t, root, "change-analysis.schema.json")
	copyAnalysisContract153(t, root, "entrypoint-inventory.schema.json")
	copyAnalysisContract153(t, root, "change-analysis-cert.schema.json")
	writeAnalysisVersion153(t, root, "1.5.2")

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

func writeAnalysisVersion153(t *testing.T, root, version string) {
	t.Helper()
	p := filepath.Join(root, ".code-harness", "VERSION")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(p, []byte(version+"\n"), 0o644); err != nil { t.Fatal(err) }
}
