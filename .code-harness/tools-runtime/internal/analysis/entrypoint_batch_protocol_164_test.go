package analysis

import (
	"strings"
	"testing"

	"codea-harness-tools/internal/changeset"
)

type batchProtocolCertificationRuntime164 struct {
	snapshot       changeset.Snapshot
	metrics        entrypointExecutionMetrics164
	inventoryCalls int
}

func (r *batchProtocolCertificationRuntime164) Compute(_ string, _ string, _ bool) (changeset.Snapshot, error) {
	return r.snapshot, nil
}

func (r *batchProtocolCertificationRuntime164) Inventory(root, runID string, snapshot changeset.Snapshot, intent Intent) (EntrypointInventory, error) {
	r.inventoryCalls++
	inventory, metrics, err := buildEntrypointInventoryWithMetrics164(root, runID, snapshot, intent)
	r.metrics = metrics
	return inventory, err
}

func Test164EntrypointBatchProtocolRejectsBeforeAnyProcess(t *testing.T) {
	for _, tc := range []struct {
		name      string
		delimiter string
	}{
		{name: "NUL", delimiter: "\x00"},
		{name: "CR", delimiter: "\r"},
		{name: "LF", delimiter: "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := "src/main/java/acme/Bad" + tc.delimiter + "Injected.java"
			snapshot := task153Snapshot([]changeset.File{{
				Path: path, Status: "M", Sources: []changeset.Source{changeset.SourceStaged},
			}})
			snapshot.MergeBase = strings.Repeat("b", 40)
			_, metrics, err := buildEntrypointInventoryWithMetrics164(t.TempDir(), "run164-protocol", snapshot, Intent{Mode: "FULL"})
			if err == nil || !strings.Contains(err.Error(), "ENTRYPOINT_SCAN_SCOPE_WIDENED") {
				t.Fatalf("batch protocol delimiter %s must fail closed as scope widening, got %v", tc.name, err)
			}
			if metrics.AstGrepProcessCount != 0 || metrics.BaseGitBatchProcessCount != 0 {
				t.Fatalf("batch protocol delimiter %s launched processes: astGrep=%d baseGitBatch=%d; want 0/0", tc.name, metrics.AstGrepProcessCount, metrics.BaseGitBatchProcessCount)
			}
			t.Logf("TASK164_BATCH_PROTOCOL_ZERO_PROCESS PASS delimiter=%s astGrepProcessCount=0 baseGitBatchProcessCount=0", tc.name)
		})
	}
}

func Test164CertifyBatchProtocolRejectsZeroProcessesZeroCertifiedWrites(t *testing.T) {
	for _, tc := range []struct {
		name      string
		delimiter string
	}{
		{name: "NUL", delimiter: "\x00"},
		{name: "CR", delimiter: "\r"},
		{name: "LF", delimiter: "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			copyAnalysisContract153(t, root, "change-analysis.schema.json")
			path := "src/main/java/acme/Bad" + tc.delimiter + "Injected.java"
			snapshot := task153Snapshot([]changeset.File{{
				Path: path, Status: "A", Sources: []changeset.Source{changeset.SourceStaged},
			}})
			snapshot.MergeBase = strings.Repeat("b", 40)
			draft := validCertificationDraft153([]string{path})
			runID := "run164-protocol-" + strings.ToLower(tc.name)
			writeCertificationDraft153(t, root, runID, draft)
			runtime := &batchProtocolCertificationRuntime164{snapshot: snapshot}

			_, err := certifyWithRuntime153(root, CertifyRequest{
				RunID: runID, DraftPath: ".code-harness/runs/" + runID + "/requests/change-analysis-draft.json",
				BaseRef: "develop", IncludeWorkingTree: true, Intent: Intent{Mode: "FULL"},
			}, runtime)
			if err == nil || !strings.Contains(err.Error(), "ENTRYPOINT_SCAN_SCOPE_WIDENED") {
				t.Fatalf("batch protocol delimiter %s must fail closed during certify, got %v", tc.name, err)
			}
			if runtime.inventoryCalls != 1 {
				t.Fatalf("batch protocol delimiter %s must be rejected by Inventory, inventoryCalls=%d", tc.name, runtime.inventoryCalls)
			}
			if runtime.metrics.AstGrepProcessCount != 0 || runtime.metrics.BaseGitBatchProcessCount != 0 {
				t.Fatalf("batch protocol delimiter %s launched processes during certify: astGrep=%d baseGitBatch=%d; want 0/0", tc.name, runtime.metrics.AstGrepProcessCount, runtime.metrics.BaseGitBatchProcessCount)
			}
			assertNoAuthoritativeAnalysis153(t, root, runID)
			t.Logf("TASK164_BATCH_PROTOCOL_CERTIFY_FAIL_CLOSED PASS delimiter=%s astGrepProcessCount=0 baseGitBatchProcessCount=0 certifiedWrites=0", tc.name)
		})
	}
}
