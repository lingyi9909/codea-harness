package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func Test151ChainDiscoverBootstrapContractIsSelfContained(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", "..", "..", ".."))
	paths := []string{
		filepath.Join(repoRoot, ".code-harness", "agents", "orchestrator.md"),
		filepath.Join(repoRoot, ".code-harness", "agents", "reviewer.md"),
		filepath.Join(repoRoot, ".code-harness", "skills", "analyze-change", "SKILL.md"),
		filepath.Join(repoRoot, ".code-harness", "skills", "discover-chain", "SKILL.md"),
	}
	texts := map[string]string{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		texts[path] = string(data)
	}

	orchestrator := texts[paths[0]]
	if !strings.Contains(orchestrator, "| `harness chain discover [target]` | Reviewer → discover-chain |") {
		t.Fatal("orchestrator must continue routing direct chain discover to Reviewer → discover-chain")
	}
	reviewer := texts[paths[1]]
	if !strings.Contains(reviewer, "analyze-change → discover-chain → Controlled Runtime") {
		t.Fatal("Reviewer must keep coordinating analyze-change → discover-chain → Controlled Runtime")
	}
	analyzeChange := texts[paths[2]]
	for _, want := range []string{"committed", "staged", "unstaged", "untracked", "生产 Controller Method"} {
		if !strings.Contains(analyzeChange, want) {
			t.Fatalf("analyze-change must preserve Change Set/new entrypoint semantics %q", want)
		}
	}

	discoverSkill := texts[paths[3]]
	for _, want := range []string{
		"Chain Discover Bootstrap（1.5.1）",
		"harness chain discover [target] 是自包含流程",
		"current Change Set",
		"analyze-change",
		"ChangeAnalysis Schema validate",
		"Runtime machine coverage verify",
		"chain discover",
		"DISCOVERED Chain",
		"不得要求用户先执行 harness review",
		"source revision / Change Set",
		"不存在或已过期时自动重新 analyze-change",
		"COMMITTED / STAGED / UNSTAGED / UNTRACKED",
		"新增 production Controller Method",
		"PARTIAL",
		"未解析",
		"原因",
		"IMPLEMENTATION_NOT_FOUND",
		"reviewCoverage.unresolvedSymbols",
	} {
		if !strings.Contains(discoverSkill, want) {
			t.Fatalf("discover-chain missing 1.5.1 bootstrap contract %q", want)
		}
	}

	for path, text := range texts {
		for _, forbidden := range []string{
			"Chain 通常依赖历史沉淀",
			"新增 Controller 首次可能无法自动形成 Chain",
			"需要先经过 harness review 正式 Coverage",
			"可能需要额外 Code Navigation",
		} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s contains forbidden discover guidance %q", path, forbidden)
			}
		}
	}
}

func Test151DirectDiscoverSupportsFreshAddedControllerServiceMapperStack(t *testing.T) {
	withTempProject(t)
	installChangeAnalysisSchema(t)
	setup151FreshGitFixture(t)

	runID := "run-151-bootstrap"
	analysisPath := filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json")
	if _, err := os.Stat(filepath.Join(".code-harness", "runs")); !os.IsNotExist(err) {
		t.Fatalf("precondition: no historical run allowed, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(".code-harness", "chains")); !os.IsNotExist(err) {
		t.Fatalf("precondition: no historical chains/** allowed, err=%v", err)
	}
	if _, err := os.Stat(analysisPath); !os.IsNotExist(err) {
		t.Fatalf("precondition: test must not prewrite change-analysis.json, err=%v", err)
	}

	if err := executeHarnessDiscoverIntent151("harness chain discover NewController", runID); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(analysisPath); err != nil {
		t.Fatalf("bootstrap must generate change-analysis.json: %v", err)
	}
	if _, err := os.Stat(filepath.Join(".code-harness", "chains")); !os.IsNotExist(err) {
		t.Fatalf("direct discovery must not create chains/**, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(".code-harness", "runs", runID, "review.md")); !os.IsNotExist(err) {
		t.Fatalf("direct discovery must not generate review.md, err=%v", err)
	}
}

func setup151FreshGitFixture(t *testing.T) {
	t.Helper()
	git151(t, "init", "-b", "develop")
	git151(t, "config", "user.email", "codea@example.invalid")
	git151(t, "config", "user.name", "Codea Test")
	git151(t, "add", ".code-harness/contracts/change-analysis.schema.json")
	git151(t, "commit", "-m", "baseline")
	git151(t, "checkout", "-b", "feature/new-approval")

	writeFile(t, "src/main/java/com/example/approval/NewController.java", `package com.example.approval;
@RestController
public class NewController {
    private final ApprovalService approvalService;
    public NewController(ApprovalService approvalService) { this.approvalService = approvalService; }
    @PostMapping("/approve")
    public void approve() { approvalService.approve(); }
}
`)
	writeFile(t, "src/main/java/com/example/approval/ApprovalService.java", `package com.example.approval;
public interface ApprovalService { void approve(); }
`)
	writeFile(t, "src/main/java/com/example/approval/ApprovalServiceImpl.java", `package com.example.approval;
@Service
public class ApprovalServiceImpl implements ApprovalService {
    private final ApprovalMapper mapper;
    public ApprovalServiceImpl(ApprovalMapper mapper) { this.mapper = mapper; }
    public void approve() { mapper.updateStatus(); }
}
`)
	writeFile(t, "src/main/java/com/example/approval/ApprovalMapper.java", `package com.example.approval;
@Mapper
public interface ApprovalMapper { void updateStatus(); }
`)
	writeFile(t, "src/main/resources/mapper/ApprovalMapper.xml", `<mapper namespace="com.example.approval.ApprovalMapper"><update id="updateStatus">update approval set status='DONE'</update></mapper>`)

	git151(t, "add",
		"src/main/java/com/example/approval/NewController.java",
		"src/main/java/com/example/approval/ApprovalService.java",
		"src/main/java/com/example/approval/ApprovalServiceImpl.java",
		"src/main/java/com/example/approval/ApprovalMapper.java",
	)
	f, err := os.OpenFile("src/main/java/com/example/approval/ApprovalServiceImpl.java", os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("// unstaged follow-up\n"); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func git151(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func executeHarnessDiscoverIntent151(intent, runID string) error {
	return errors.New("CHAIN_DISCOVER_BOOTSTRAP_NOT_EXECUTED: " + intent + " run=" + runID)
}
