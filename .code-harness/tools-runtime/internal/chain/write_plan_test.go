package chain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func installTask153WritePlanContracts(t *testing.T, root string) {
	t.Helper()
	installTask153ChainAuthorityContract(t, root, "chain-candidate-cert.schema.json")
	installTask153ChainAuthorityContract(t, root, "chain-write-plan.schema.json")
}

func Test153WritePlanRejectsFakeCandidateBeforeAnalysisLookup(t *testing.T) {
	root := t.TempDir()
	installTask153WritePlanContracts(t, root)
	candidatePath := ".code-harness/runs/r153/analysis/discovered-chains/order-approve.yaml"
	writeTask153AuthorityCandidate(t, root, candidatePath, task153AuthorityCandidate())

	_, err := SealWritePlan(root, "r153", candidatePath, "")
	if err == nil || !strings.Contains(err.Error(), "CHAIN_ARTIFACT_NOT_RUNTIME_OWNED") {
		t.Fatalf("fake Runtime-path candidate must fail provenance before analysis lookup, got %v", err)
	}
}

func Test153WritePlanRejectsMutatedCandidateBeforeAnalysisLookup(t *testing.T) {
	root := t.TempDir()
	installTask153WritePlanContracts(t, root)
	candidatePath := ".code-harness/runs/r153/analysis/discovered-chains/order-approve.yaml"
	candidate := task153AuthorityCandidate()
	writeTask153AuthorityCandidate(t, root, candidatePath, candidate)
	if _, err := CertifyCandidate(root, candidate, candidatePath, "DISCOVERED", task153AnalysisCert()); err != nil {
		t.Fatal(err)
	}
	abs := filepath.Join(root, filepath.FromSlash(candidatePath))
	if err := os.WriteFile(abs, append(mustReadTask153(t, abs), []byte("# changed\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := SealWritePlan(root, "r153", candidatePath, "")
	if err == nil || !strings.Contains(err.Error(), "CHAIN_CANDIDATE_HASH_MISMATCH") {
		t.Fatalf("mutated candidate must fail hash binding before analysis lookup, got %v", err)
	}
}

func Test153WritePlanIDBindsCandidateAnalysisExistingAndPreviewHashes(t *testing.T) {
	base := WritePlan{
		RunID: "r153",
		ChainID: "order-approve",
		CandidatePath: ".code-harness/runs/r153/analysis/discovered-chains/order-approve.yaml",
		CandidateHash: strings.Repeat("a", 64),
		AnalysisHash: strings.Repeat("b", 64),
		ExpectedExistingHash: strings.Repeat("c", 64),
		PreviewSHA256: strings.Repeat("d", 64),
	}
	id := writePlanID153(base)
	if id != writePlanID153(base) {
		t.Fatal("identical sealed facts must produce deterministic planId")
	}
	variants := []WritePlan{base, base, base, base}
	variants[0].CandidateHash = strings.Repeat("e", 64)
	variants[1].AnalysisHash = strings.Repeat("e", 64)
	variants[2].ExpectedExistingHash = strings.Repeat("e", 64)
	variants[3].PreviewSHA256 = strings.Repeat("e", 64)
	for i, variant := range variants {
		if got := writePlanID153(variant); got == id {
			t.Fatalf("planId must change when sealed fact %d changes", i)
		}
	}
}

func mustReadTask153(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil { t.Fatal(err) }
	return b
}
