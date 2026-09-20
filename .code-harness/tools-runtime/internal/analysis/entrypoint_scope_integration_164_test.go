package analysis

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"codea-harness-tools/internal/changeset"
)

type recordingEntrypointScanner164 struct {
	currentCalls []string
	baseCalls    []string
}

func (s *recordingEntrypointScanner164) Current(_ context.Context, path string) ([]ControllerEndpoint, error) {
	s.currentCalls = append(s.currentCalls, path)
	return nil, nil
}

func (s *recordingEntrypointScanner164) Base(_ context.Context, _ changeset.Snapshot, path string) ([]ControllerEndpoint, error) {
	s.baseCalls = append(s.baseCalls, path)
	return nil, nil
}

func Test164EntrypointInventoryRequestsOnlySnapshotSideExactFiles(t *testing.T) {
	snapshot := changeset.Snapshot{
		SHA256: strings.Repeat("a", 64),
		Files: []changeset.File{
			{Path: "src/main/java/acme/A.java", Status: "M"},
			{Path: "src/main/java/acme/B.java", Status: "M"},
			{Path: "src/main/java/acme/C.java", Status: "D"},
			{Path: "src/main/java/acme/D.java", Status: "A"},
			{Path: "src/test/java/acme/TestOnly.java", Status: "M"},
		},
	}
	scanner := &recordingEntrypointScanner164{}
	if _, err := buildEntrypointInventoryWithScanner(context.Background(), "run164", snapshot, Intent{Mode: "FULL"}, scanner); err != nil {
		t.Fatal(err)
	}
	wantCurrent := []string{"src/main/java/acme/A.java", "src/main/java/acme/B.java", "src/main/java/acme/D.java"}
	wantBase := []string{"src/main/java/acme/A.java", "src/main/java/acme/B.java", "src/main/java/acme/C.java"}
	if !reflect.DeepEqual(scanner.currentCalls, wantCurrent) {
		t.Fatalf("currentRequestedFiles=%v want=%v", scanner.currentCalls, wantCurrent)
	}
	if !reflect.DeepEqual(scanner.baseCalls, wantBase) {
		t.Fatalf("baseRequestedFiles=%v want=%v", scanner.baseCalls, wantBase)
	}
}

type scopeWideningCertificationRuntime164 struct {
	snapshot changeset.Snapshot
}

func (r scopeWideningCertificationRuntime164) Compute(_ string, _ string, _ bool) (changeset.Snapshot, error) {
	return r.snapshot, nil
}

func (scopeWideningCertificationRuntime164) Inventory(_ string, _ string, _ changeset.Snapshot, _ Intent) (EntrypointInventory, error) {
	return EntrypointInventory{}, errors.New("ENTRYPOINT_SCAN_SCOPE_WIDENED: injected runner widening")
}

func Test164CertifyEntrypointScopeWideningZeroCertifiedWrites(t *testing.T) {
	root := t.TempDir()
	copyAnalysisContract153(t, root, "change-analysis.schema.json")

	path := "src/main/java/acme/AController.java"
	snapshot := task153Snapshot([]changeset.File{{Path: path, Status: "A", Sources: []changeset.Source{changeset.SourceStaged}}})
	draft := validCertificationDraft153([]string{path})
	writeCertificationDraft153(t, root, "r164", draft)

	_, err := certifyWithRuntime153(root, CertifyRequest{
		RunID: "r164", DraftPath: ".code-harness/runs/r164/requests/change-analysis-draft.json",
		BaseRef: "develop", IncludeWorkingTree: true, Intent: Intent{Mode: "FULL"},
	}, scopeWideningCertificationRuntime164{snapshot: snapshot})
	if err == nil || !strings.Contains(err.Error(), "ENTRYPOINT_SCAN_SCOPE_WIDENED") {
		t.Fatalf("scope widening must fail closed, got %v", err)
	}
	assertNoAuthoritativeAnalysis153(t, root, "r164")
}
