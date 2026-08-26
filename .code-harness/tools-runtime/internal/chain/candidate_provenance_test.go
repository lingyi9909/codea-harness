package chain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	analysisruntime "codea-harness-tools/internal/analysis"
)

func task153AuthorityCandidate() Chain {
	return Chain{
		Version: 1,
		ID: "order-approve",
		Name: "OrderController.approve",
		Status: StatusDiscovered,
		EntryPoints: []EntryPoint{{Workspace: CurrentWorkspace, Symbol: "OrderController.approve", Path: "src/main/java/com/example/order/OrderController.java"}},
		Nodes: []Node{{Workspace: CurrentWorkspace, Symbol: "OrderService.approve", Path: "src/main/java/com/example/order/OrderService.java", Role: "SERVICE"}},
	}
}

func installTask153ChainAuthorityContract(t *testing.T, root, name string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", name))
	if err != nil { t.Fatal(err) }
	dst := filepath.Join(root, ".code-harness", "contracts", name)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(dst, data, 0o644); err != nil { t.Fatal(err) }
}

func writeTask153AuthorityCandidate(t *testing.T, root, candidatePath string, c Chain) {
	t.Helper()
	b, err := MarshalYAML(c)
	if err != nil { t.Fatal(err) }
	abs := filepath.Join(root, filepath.FromSlash(candidatePath))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(abs, b, 0o644); err != nil { t.Fatal(err) }
}

func task153AnalysisCert() analysisruntime.Certificate {
	return analysisruntime.Certificate{RunID: "r153", AnalysisSHA256: strings.Repeat("a", 64)}
}

func Test153CandidateAuthorityRejectsFakeAgentCandidate(t *testing.T) {
	root := t.TempDir()
	installTask153ChainAuthorityContract(t, root, "chain-candidate-cert.schema.json")
	candidatePath := ".code-harness/runs/r153/analysis/discovered-chains/order-approve.yaml"
	writeTask153AuthorityCandidate(t, root, candidatePath, task153AuthorityCandidate())

	_, _, err := LoadRuntimeCandidate(root, candidatePath, task153AnalysisCert())
	if err == nil || !strings.Contains(err.Error(), "CHAIN_ARTIFACT_NOT_RUNTIME_OWNED") {
		t.Fatalf("hand-created candidate under Runtime path must be rejected, got %v", err)
	}
}

func Test153CandidateAuthorityRejectsMutatedRuntimeCandidate(t *testing.T) {
	root := t.TempDir()
	installTask153ChainAuthorityContract(t, root, "chain-candidate-cert.schema.json")
	candidatePath := ".code-harness/runs/r153/analysis/discovered-chains/order-approve.yaml"
	candidate := task153AuthorityCandidate()
	writeTask153AuthorityCandidate(t, root, candidatePath, candidate)
	if _, err := CertifyCandidate(root, candidate, candidatePath, "DISCOVERED", task153AnalysisCert()); err != nil {
		t.Fatalf("certify Runtime candidate: %v", err)
	}
	abs := filepath.Join(root, filepath.FromSlash(candidatePath))
	f, err := os.OpenFile(abs, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil { t.Fatal(err) }
	if _, err := f.WriteString("# agent mutation\n"); err != nil { _ = f.Close(); t.Fatal(err) }
	if err := f.Close(); err != nil { t.Fatal(err) }

	_, _, err = LoadRuntimeCandidate(root, candidatePath, task153AnalysisCert())
	if err == nil || !strings.Contains(err.Error(), "CHAIN_CANDIDATE_HASH_MISMATCH") {
		t.Fatalf("mutated Runtime candidate must fail hash binding, got %v", err)
	}
}

func Test153CandidateAuthorityRejectsMutationBeforeRuntimeCertification(t *testing.T) {
	root := t.TempDir()
	installTask153ChainAuthorityContract(t, root, "chain-candidate-cert.schema.json")
	candidatePath := ".code-harness/runs/r153/analysis/discovered-chains/order-approve.yaml"
	runtimeCandidate := task153AuthorityCandidate()
	writeTask153AuthorityCandidate(t, root, candidatePath, runtimeCandidate)

	mutated := runtimeCandidate
	mutated.Nodes = []Node{{Workspace: CurrentWorkspace, Symbol: "InjectedService.approve", Path: "src/main/java/com/example/order/InjectedService.java", Role: "SERVICE"}}
	writeTask153AuthorityCandidate(t, root, candidatePath, mutated)

	_, err := CertifyCandidate(root, runtimeCandidate, candidatePath, "DISCOVERED", task153AnalysisCert())
	if err == nil || !strings.Contains(err.Error(), "CHAIN_CANDIDATE_RUNTIME_BYTES_MISMATCH") {
		t.Fatalf("candidate changed after Runtime generation but before provenance certification must be rejected, got %v", err)
	}
	if _, statErr := os.Stat(candidateCertPath153(root, candidatePath)); !os.IsNotExist(statErr) {
		t.Fatalf("rejected pre-certification mutation must not produce provenance certificate, stat=%v", statErr)
	}
}
