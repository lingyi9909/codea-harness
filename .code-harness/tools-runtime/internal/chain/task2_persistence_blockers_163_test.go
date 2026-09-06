package chain

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/nav"
)

func setupTask163ProjectPersistenceCandidate(t *testing.T, root, runID string) (string, Chain) {
	t.Helper()
	installTask153WritePlanContracts(t, root)
	for rel, body := range map[string]string{
		"src/main/java/com/example/order/OrderController.java": "package com.example.order; class OrderController { void approve() {} }\n",
		"src/main/java/com/example/order/OrderService.java":    "package com.example.order; class OrderService { void approve() {} }\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	identity, err := SnapshotProjectSource(root, runID)
	if err != nil {
		t.Fatalf("snapshot PROJECT source: %v", err)
	}
	candidate := task153AuthorityCandidate()
	candidatePath := filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "analysis", "discovered-chains", candidate.ID+".yaml"))
	if err := persistDiscovered(root, runID, []Chain{candidate}); err != nil {
		t.Fatalf("persist Runtime PROJECT candidate: %v", err)
	}
	if _, err := CertifyProjectCandidate(root, candidate, candidatePath, identity); err != nil {
		t.Fatalf("certify Runtime PROJECT candidate: %v", err)
	}
	return candidatePath, candidate
}

func Test163Task2ProjectDiscoverSealPersist(t *testing.T) {
	root := t.TempDir()
	runID := "run-163-project-persist"
	candidatePath, _ := setupTask163ProjectPersistenceCandidate(t, root, runID)
	plan, err := SealWritePlan(root, runID, candidatePath, "")
	if err != nil {
		t.Fatalf("PROJECT candidate must seal without fabricated ChangeAnalysis: %v", err)
	}
	if err := PersistWritePlan(root, runID, plan.PlanID); err != nil {
		t.Fatalf("PROJECT sealed plan must persist with PROJECT_SOURCE authority: %v", err)
	}
	t.Log("PROJECT_DISCOVER_SEAL_PERSIST PASS")
}

func Test163Task2ProjectDiscoverPersistedChain(t *testing.T) {
	root := t.TempDir()
	runID := "run-163-project-persisted-chain"
	candidatePath, candidate := setupTask163ProjectPersistenceCandidate(t, root, runID)
	plan, err := SealWritePlan(root, runID, candidatePath, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := PersistWritePlan(root, runID, plan.PlanID); err != nil {
		t.Fatal(err)
	}
	path, err := ChainPath(root, candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := Load(path)
	if err != nil {
		t.Fatalf("load persisted PROJECT chain: %v", err)
	}
	if persisted.ID != candidate.ID || persisted.Status != StatusAccepted {
		t.Fatalf("persisted PROJECT chain=%+v", persisted)
	}
	t.Log("PROJECT_DISCOVER_PERSISTED_CHAIN PASS")
}

func Test163Task2ProjectSourceStaleBeforePersistRejects(t *testing.T) {
	root := t.TempDir()
	runID := "run-163-project-stale"
	candidatePath, candidate := setupTask163ProjectPersistenceCandidate(t, root, runID)
	plan, err := SealWritePlan(root, runID, candidatePath, "")
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "src", "main", "java", "com", "example", "order", "OrderService.java")
	if err := os.WriteFile(marker, []byte("package com.example.order; class OrderService { void approveChanged() {} }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := PersistWritePlan(root, runID, plan.PlanID); err == nil || !strings.Contains(err.Error(), "PROJECT_SOURCE_STALE") {
		t.Fatalf("stale PROJECT source must reject before Project State write, err=%v", err)
	}
	projectPath, err := ChainPath(root, candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(projectPath); !os.IsNotExist(err) {
		t.Fatalf("stale PROJECT source must produce zero Project State writes, stat=%v", err)
	}
	t.Log("PROJECT_SOURCE_STALE_BEFORE_PERSIST REJECT")
}

type task163SourceMutatingRunner struct {
	path    string
	mutated bool
}

func (r *task163SourceMutatingRunner) Run(_ context.Context, _ string, _ ...string) ([]byte, error) {
	if !r.mutated {
		r.mutated = true
		if err := os.WriteFile(r.path, []byte("package example; class Marker { int changed; }\n"), 0o644); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func Test163Task2ProjectSourceChangedDuringDiscoveryRejects(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "src", "main", "java", "example", "Marker.java")
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("package example; class Marker {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &task163SourceMutatingRunner{path: marker}
	_, _, err := DiscoverProject(context.Background(), root, ProjectDiscoverInput{
		RunID: "run-163-project-toctou",
		Navigator: nav.Navigator{
			RepoRoot:    root,
			AstGrepPath: "ast-grep",
			Runner:      runner,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "PROJECT_SOURCE_CHANGED_DURING_DISCOVERY") {
		t.Fatalf("PROJECT discovery must fail closed when source changes during navigation, err=%v", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".code-harness", "runs", "run-163-project-toctou", "analysis", "project-source.json")); !os.IsNotExist(statErr) {
		t.Fatalf("changed-during-discovery source must not become authoritative snapshot, stat=%v", statErr)
	}
	t.Log("PROJECT_SOURCE_CHANGED_DURING_DISCOVERY REJECT")
}
