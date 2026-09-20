package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"codea-harness-tools/internal/chain"
)

func Test163Task2CleanProjectDiscoveryFindsExistingChain(t *testing.T) {
	astGrep := strings.TrimSpace(os.Getenv("CODEA_AST_GREP_TEST_PATH"))
	if astGrep == "" {
		t.Skip("CODEA_AST_GREP_TEST_PATH is required for Task 2 real project discovery")
	}

	withTempProject(t)
	installTask163ChainCandidateContract(t)
	writeFile(t, ".gitignore", ".code-harness/bin/\n.code-harness/runs/\n")
	writeFile(t, "src/main/java/com/example/order/OrderController.java", `package com.example.order;
@RestController
public class OrderController {
    private final OrderService service;
    public OrderController(OrderService service) { this.service = service; }
    @PostMapping("/orders")
    public void createOrder() { service.createOrder(); }
}
`)
	writeFile(t, "src/main/java/com/example/order/OrderService.java", `package com.example.order;
public interface OrderService { void createOrder(); }
`)
	writeFile(t, "src/main/java/com/example/order/OrderServiceImpl.java", `package com.example.order;
@Service
public class OrderServiceImpl implements OrderService {
    private final OrderMapper mapper;
    public OrderServiceImpl(OrderMapper mapper) { this.mapper = mapper; }
    public void createOrder() { mapper.insertOrder(); }
}
`)
	writeFile(t, "src/main/java/com/example/order/OrderMapper.java", `package com.example.order;
@Mapper
public interface OrderMapper { void insertOrder(); }
`)
	writeFile(t, "src/main/resources/mapper/OrderMapper.xml", `<mapper namespace="com.example.order.OrderMapper"><insert id="insertOrder">insert into orders(id) values (1)</insert></mapper>`)

	git163Task2(t, "init", "-b", "main")
	git163Task2(t, "config", "user.email", "codea@example.invalid")
	git163Task2(t, "config", "user.name", "Codea Test")
	git163Task2(t, "add", ".")
	git163Task2(t, "commit", "-m", "clean project baseline")

	if got := strings.TrimSpace(git163Task2(t, "status", "--porcelain")); got != "" {
		t.Fatalf("fixture working tree must be clean before discovery, status=%q", got)
	}
	if got := strings.TrimSpace(git163Task2(t, "branch", "--show-current")); got != "main" {
		t.Fatalf("fixture must discover from clean main, branch=%q", got)
	}

	copyTask163Executable(t, astGrep, filepath.Join(".code-harness", "bin", "ast-grep.exe"))
	if got := strings.TrimSpace(git163Task2(t, "status", "--porcelain")); got != "" {
		t.Fatalf("installed Runtime binary must not dirty clean fixture, status=%q", got)
	}

	runID := "run-163-project-discovery"
	requestPath := writeQueryRequest(t, runID, "chain-discover.json", `{"runId":"run-163-project-discovery","mode":"PROJECT","target":"OrderController"}`)
	if err := run([]string{"chain", "discover", "--input", requestPath}); err != nil {
		t.Fatalf("clean project discovery failed: %v", err)
	}

	analysisDir := filepath.Join(".code-harness", "runs", runID, "analysis")
	for _, forbidden := range []string{"change-set.json", "change-analysis.json"} {
		if _, err := os.Stat(filepath.Join(analysisDir, forbidden)); !os.IsNotExist(err) {
			t.Fatalf("PROJECT discovery must not synthesize %s, err=%v", forbidden, err)
		}
	}
	if _, err := os.Stat(filepath.Join(analysisDir, "project-source.json")); err != nil {
		t.Fatalf("PROJECT discovery must persist Runtime-owned project source identity: %v", err)
	}

	discoveredDir := filepath.Join(analysisDir, "discovered-chains")
	entries, err := os.ReadDir(discoveredDir)
	if err != nil {
		t.Fatalf("read discovered candidates: %v", err)
	}
	var yamlPath, certPath string
	for _, entry := range entries {
		switch {
		case strings.HasSuffix(entry.Name(), ".yaml"):
			yamlPath = filepath.Join(discoveredDir, entry.Name())
		case strings.HasSuffix(entry.Name(), ".cert.json"):
			certPath = filepath.Join(discoveredDir, entry.Name())
		}
	}
	if yamlPath == "" || certPath == "" {
		t.Fatalf("Runtime-owned candidate + provenance certificate required, entries=%v", entries)
	}
	yamlBytes, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"status: DISCOVERED",
		"OrderController.createOrder",
		"OrderService.createOrder",
		"OrderServiceImpl.createOrder",
		"OrderMapper.insertOrder",
		"MAPPER_XML",
		"src/main/resources/mapper/OrderMapper.xml",
	} {
		if !strings.Contains(string(yamlBytes), want) {
			t.Fatalf("clean PROJECT discovery missing %q:\n%s", want, yamlBytes)
		}
	}

	certBytes, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatal(err)
	}
	var cert struct {
		RunID         string `json:"runId"`
		Kind          string `json:"kind"`
		CandidatePath string `json:"candidatePath"`
		CandidateHash string `json:"candidateHash"`
		AnalysisHash  string `json:"analysisHash"`
		AuthorityKind string `json:"authorityKind"`
		SourceHash    string `json:"sourceHash"`
	}
	if err := json.Unmarshal(certBytes, &cert); err != nil {
		t.Fatalf("decode candidate certificate: %v", err)
	}
	if cert.RunID != runID || cert.Kind != "DISCOVERED" || cert.AuthorityKind != "PROJECT_SOURCE" || cert.SourceHash == "" || cert.AnalysisHash != cert.SourceHash {
		t.Fatalf("PROJECT candidate provenance identity incomplete: %+v", cert)
	}
	actualCandidateHash := fmt.Sprintf("%x", sha256.Sum256(yamlBytes))
	if cert.CandidateHash != actualCandidateHash {
		t.Fatalf("candidate certificate hash=%s want=%s", cert.CandidateHash, actualCandidateHash)
	}

	if _, err := os.Stat(filepath.Join(".code-harness", "chains")); !os.IsNotExist(err) {
		t.Fatalf("PROJECT discovery must not write .code-harness/chains/**, err=%v", err)
	}
	if hasTask163FileNamed(t, filepath.Join(".code-harness", "runs"), "review.md") {
		t.Fatal("PROJECT discovery must not generate review.md")
	}

	// Runtime provenance must fail closed before any later ChangeAnalysis authority
	// check if a discovered candidate is tampered after certification.
	if err := os.WriteFile(yamlPath, append(yamlBytes, []byte("# tampered\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	relCandidate, err := filepath.Rel(".", yamlPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = chain.SealWritePlan(".", runID, filepath.ToSlash(relCandidate), "")
	if err == nil || !strings.Contains(err.Error(), "CHAIN_CANDIDATE_HASH_MISMATCH") {
		t.Fatalf("tampered PROJECT candidate must fail Runtime provenance before persistence authority, err=%v", err)
	}
}

func installTask163ChainCandidateContract(t *testing.T) {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	source := filepath.Join(filepath.Dir(testFile), "..", "..", "..", "contracts", "chain-candidate-cert.schema.json")
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(".code-harness", "contracts", "chain-candidate-cert.schema.json"), string(data))
}

func copyTask163Executable(t *testing.T, source, destination string) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read ast-grep fixture binary: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, data, 0o755); err != nil {
		t.Fatalf("install ast-grep fixture binary: %v", err)
	}
}

func git163Task2(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func hasTask163FileNamed(t *testing.T, root, name string) bool {
	t.Helper()
	found := false
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !entry.IsDir() && entry.Name() == name {
			found = true
		}
		return nil
	})
	return found
}
