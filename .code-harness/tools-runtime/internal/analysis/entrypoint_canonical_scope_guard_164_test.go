package analysis

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/changeset"
)

type canonicalScopeWideningRuntime164 struct {
	snapshot changeset.Snapshot
}

func (r canonicalScopeWideningRuntime164) Compute(_ string, _ string, _ bool) (changeset.Snapshot, error) {
	return r.snapshot, nil
}

func (canonicalScopeWideningRuntime164) Inventory(_ string, _ string, _ changeset.Snapshot, _ Intent) (EntrypointInventory, error) {
	return EntrypointInventory{}, errors.New("ENTRYPOINT_SCAN_SCOPE_WIDENED: injected canonical runner widening")
}

func Test164CertifyEntrypointScopeWideningCanonicalZeroCertifiedWrites(t *testing.T) {
	root := t.TempDir()
	gitTask164(t, root, "init", "-b", "feature")
	gitTask164(t, root, "config", "user.email", "task164@example.invalid")
	gitTask164(t, root, "config", "user.name", "Task 164 Gate")
	gitTask164(t, root, "config", "core.autocrlf", "false")
	if err := os.WriteFile(filepath.Join(root, ".git", "info", "exclude"), []byte(".code-harness/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := "src/main/java/acme/AController.java"
	writeTask164Java(t, root, path, `package acme;
@RestController
public class AController {
    @GetMapping
    public String get() { return "base"; }
}
`)
	gitTask164(t, root, "add", "src/main/java")
	gitTask164(t, root, "commit", "-m", "base")
	baseSHA := strings.TrimSpace(gitTask164(t, root, "rev-parse", "HEAD"))
	writeTask164Java(t, root, path, `package acme;
@RestController
public class AController {
    @GetMapping
    public String get() { return "current"; }
}
`)

	snapshot, err := changeset.Compute(root, baseSHA, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Files) != 1 || snapshot.Files[0].Path != path {
		t.Fatalf("unexpected canonical fixture snapshot: %+v", snapshot.Files)
	}
	for _, name := range []string{"change-set.schema.json", "change-analysis-proposal.schema.json", "change-analysis.schema.json"} {
		copyAnalysisContract153(t, root, name)
	}
	snapshotBytes, err := changeset.CanonicalBytes(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	snapshotPath := filepath.Join(root, ".code-harness", "runs", "run164-canonical", "analysis", "change-set.json")
	if err := os.MkdirAll(filepath.Dir(snapshotPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(snapshotPath, snapshotBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	proposal := map[string]any{
		"changedFileRoles": []map[string]any{{"path": path, "role": "Controller"}},
		"affectedControllers": []map[string]any{{"controller": "AController", "endpoints": []string{"AController.get"}, "impactType": "DIRECT_CHANGE", "sourceSymbols": []string{"AController.get"}}},
		"callChains": []map[string]any{{"entryPoint": "AController.get", "chain": []string{"AController.get"}}},
		"symbolLocations": []map[string]any{{"symbol": "AController.get", "path": path, "role": "Controller", "source": "FIND_SYMBOL"}},
		"resourceRelations": []any{},
		"externalDependencies": []any{},
		"riskAreas": []any{},
		"reviewCoverage": map[string]any{
			"status": "COMPLETE",
			"reviewedFiles": []map[string]any{{"path": path, "role": "Controller", "reason": "CHANGED"}},
			"unresolvedSymbols": []any{},
		},
	}
	proposalBytes, err := json.MarshalIndent(proposal, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	proposalPath := filepath.Join(root, ".code-harness", "runs", "run164-canonical", "requests", "change-analysis-proposal.json")
	if err := os.MkdirAll(filepath.Dir(proposalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(proposalPath, append(proposalBytes, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = certifyWithRuntime153(root, CertifyRequest{
		RunID:          "run164-canonical",
		SnapshotPath:   ".code-harness/runs/run164-canonical/analysis/change-set.json",
		SnapshotSHA256: snapshot.SnapshotSHA256,
		ProposalPath:   ".code-harness/runs/run164-canonical/requests/change-analysis-proposal.json",
		Intent:         Intent{Mode: "FULL"},
	}, canonicalScopeWideningRuntime164{snapshot: snapshot})
	if err == nil || !strings.Contains(err.Error(), "ENTRYPOINT_SCAN_SCOPE_WIDENED") {
		t.Fatalf("canonical scope widening must fail closed, got %v", err)
	}
	assertNoAuthoritativeAnalysis153(t, root, "run164-canonical")
}
