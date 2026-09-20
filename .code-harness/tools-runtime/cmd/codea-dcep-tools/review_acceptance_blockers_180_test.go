package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/reviewrun"
)

func Test180FinishToolUsesInvocationUniqueExclusiveRequestFiles(t *testing.T) {
	root := repoRoot180(t)
	data, err := os.ReadFile(filepath.Join(root, ".code-harness", "tools", "codea-review.ts"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, `path.resolve(requestsRoot, "finish.json")`) {
		t.Fatal("all finish invocations still share requests/finish.json")
	}
	for _, want := range []string{"randomUUID", `flag: "wx"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("finish tool must create an invocation-unique non-overwritable request; missing %q", want)
		}
	}
}

func Test180PrimaryToolMakesEmptyFindingFinishContinuationExplicit(t *testing.T) {
	root := repoRoot180(t)
	data, err := os.ReadFile(filepath.Join(root, ".code-harness", "tools", "codea-review.ts"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"READ_SCOPE_AND_FINISH_THIS_TURN",
		"finishRequiredEvenWhenFindingsEmpty",
		"WAIT_FOR_REAL_USER_SELECTION",
		"KEEP_REPORT_INCOMPLETE",
		"nextAction: nextAction(runtime, scope)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("primary review tool must expose mandatory continuation contract; missing %q", want)
		}
	}
}

func Test180ScopeReadyReviewIsNonTerminalAcrossActiveInstructions(t *testing.T) {
	root := repoRoot180(t)
	files := []string{
		filepath.Join(".code-harness", "commands", "harness-review.md"),
		filepath.Join(".code-harness", "agents", "orchestrator.md"),
	}
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, want := range []string{
			"READ_SCOPE_AND_FINISH_THIS_TURN",
			"non-terminal",
			"finish",
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s must keep a scope-ready review in the same assistant turn; missing %q", rel, want)
			}
		}
	}
	toolData, err := os.ReadFile(filepath.Join(root, ".code-harness", "tools", "codea-review.ts"))
	if err != nil {
		t.Fatal(err)
	}
	toolText := string(toolData)
	for _, want := range []string{
		"assistantTurnTerminal: false",
		"mustContinueToolExecution: true",
		"finishRequiredEvenWhenFindingsEmpty: true",
	} {
		if !strings.Contains(toolText, want) {
			t.Fatalf("scope-ready structured tool state must be non-terminal; missing %q", want)
		}
	}
}

func Test180FinalMatrixFixturesRespectNavigationAndSelectionContract(t *testing.T) {
	// Clean controls must be production-like and preserve the complete Controller -> ServiceImpl -> Mapper -> Mapper XML chain required by 1.8 navigation.
	root := repoRoot180(t)
	data, err := os.ReadFile(filepath.Join(root, ".github", "scripts", "review180-real-model-e2e.py"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`CLEAN_READINESS_METHOD = """    public boolean databaseReady()`,
		`int probe = mapper.ping();`,
		`CLEAN_READINESS_METHOD_CHANGED = """    public boolean databaseReady()`,
		`int result = mapper.ping();`,
		`CLEAN_READINESS_SQL = "SELECT 1"`,
		`PublicDatabaseReadinessController.java`,
		`DatabaseReadinessService.java`,
		`DatabaseReadinessServiceImpl.java`,
		`DatabaseReadinessMapper.java`,
		`DatabaseReadinessMapper.xml`,
		`public class DatabaseReadinessServiceImpl implements DatabaseReadinessService`,
		`private final DatabaseReadinessMapper mapper;`,
		`@Mapper`,
		`src" / "main" / "resources" / "com" / "example"`,
		`<mapper namespace="com.example.DatabaseReadinessMapper">`,
		`<select id="ping" resultType="int">`,
		`@GetMapping("/public/readiness/database")`,
		`return service.databaseReady();`,
		`int ping();`,
		`write_fixture(project, multi, clean_read=(scenario == "single-clean"))`,
		`command_args.append("PublicDatabaseReadinessController.databaseReady")`,
		`readiness_impl = project / "src" / "main" / "java" / "com" / "example" / "DatabaseReadinessServiceImpl.java"`,
		`not result.get("pendingRisks", [])`,
		`not result.get("gaps", [])`,
		`result.get("coverage") == "COMPLETE"`,
		`if scenario in {"early-stop", "timeout"}:`,
		`mutate_for_scenario(project, "single-issue")`,
		`command_args.append("OrderController")`,
		`host.require(scope.get("selectedIds") == ["C1"]`,
		`safe_method = """    @PreAuthorize`,
		`boolean updated = service.updateStatus(tenantId, id, status);`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("final real-model fixture lost matrix contract %q", want)
		}
	}
	if strings.Contains(text, "service.cancel(") {
		t.Fatal("multi-chain fixture must keep updateStatus lexically first so seeded issue remains C1")
	}
	for _, forbidden := range []string{
		`/orders/review-probe`,
		`reviewProbe`,
		`CLEAN_PROBE_SQL`,
		`CLEAN_COUNT_SQL`,
		`CLEAN_CATALOG_SQL`,
		`active = TRUE`,
		`reference.iso_country_codes`,
		`iso_country_codes`,
		`numericCode`,
		`countActiveCountries`,
		`.matches(`,
		`findCategoryNames`,
		`categoryNames`,
		`WHERE id = #{categoryId}`,
		`java.util.List<String>`,
		`REFERENCE_READ`,
		`@GetMapping("/reference/categories/{parentCategoryId}/child-count")`,
		`ProductCategoryReferenceController`,
		`ProductCategoryReferenceService`,
		`ProductCategoryReferenceMapper`,
		`product_category`,
		`/public/reference/categories/`,
		`CLEAN_ENUM_METHOD`,
		`OrderStatusCatalogController`,
		`OrderStatusCatalogServiceImpl`,
		`order_status_catalog`,
		`/admin/orders/count`,
		`OrderController.countOrders`,
		`else "single-clean"`,
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("final real-model fixture must not regress to debug-like or invalid clean setup %q", forbidden)
		}
	}
}


func Test180FinalMatrixCleanFixtureNavigatesCompleteSingleChain(t *testing.T) {
	if strings.TrimSpace(os.Getenv("CODEA_AST_GREP_TEST_PATH")) == "" {
		t.Skip("formal Windows release regression provides CODEA_AST_GREP_TEST_PATH")
	}
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	write("src/main/java/com/example/PublicDatabaseReadinessController.java", `package com.example;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class PublicDatabaseReadinessController {
    private final DatabaseReadinessService service;
    public PublicDatabaseReadinessController(DatabaseReadinessService service) { this.service = service; }

    @GetMapping("/public/readiness/database")
    public boolean databaseReady() {
        return service.databaseReady();
    }
}
`)
	write("src/main/java/com/example/DatabaseReadinessService.java", `package com.example;
public interface DatabaseReadinessService {
    boolean databaseReady();
}
`)
	write("src/main/java/com/example/DatabaseReadinessServiceImpl.java", `package com.example;
import org.springframework.stereotype.Service;

@Service
public class DatabaseReadinessServiceImpl implements DatabaseReadinessService {
    private final DatabaseReadinessMapper mapper;
    public DatabaseReadinessServiceImpl(DatabaseReadinessMapper mapper) { this.mapper = mapper; }

    public boolean databaseReady() {
        int result = mapper.ping();
        return result == 1;
    }
}
`)
	write("src/main/java/com/example/DatabaseReadinessMapper.java", `package com.example;
import org.apache.ibatis.annotations.Mapper;

@Mapper
public interface DatabaseReadinessMapper {
    int ping();
}
`)
	write("src/main/resources/com/example/DatabaseReadinessMapper.xml", `<?xml version="1.0" encoding="UTF-8"?>
<mapper namespace="com.example.DatabaseReadinessMapper">
  <select id="ping" resultType="int">SELECT 1</select>
</mapper>
`)

	started, err := reviewrun.Start(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reviewrun.Prepare(
		context.Background(),
		root,
		started.RunID,
		reviewrun.Intent{Mode: "CURRENT_IMPLEMENTATION", Target: "PublicDatabaseReadinessController.databaseReady"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !got.DiscoveryComplete || got.SelectionRequired || len(got.Chains) != 1 || len(got.Gaps) != 0 {
		t.Fatalf("clean matrix fixture must produce one complete auto-selected chain: %+v", got)
	}
	if len(got.Chains[0].Nodes) < 4 {
		t.Fatalf("clean matrix fixture must preserve Controller -> ServiceImpl -> Mapper -> Mapper XML nodes: %+v", got.Chains[0])
	}
	wantSQL := "src/main/resources/com/example/DatabaseReadinessMapper.xml"
	foundSQL := false
	for _, node := range got.Chains[0].Nodes {
		if node.Role == "SQL" && node.Path == wantSQL && node.Symbol == "DatabaseReadinessMapper.ping" {
			foundSQL = true
		}
	}
	if !foundSQL {
		t.Fatalf("clean matrix fixture must use co-located mapper XML without external mapper-locations config: %+v", got.Chains[0])
	}
}

func Test180HumanSelectionMenuIsMachineVerifiable(t *testing.T) {
	root := repoRoot180(t)
	toolData, err := os.ReadFile(filepath.Join(root, ".code-harness", "tools", "codea-review.ts"))
	if err != nil {
		t.Fatal(err)
	}
	toolText := string(toolData)
	for _, want := range []string{
		"function selectionMenuText",
		"requiredMenuText",
		" options=",
		"copy requiredMenuText verbatim",
		"Do not convert it to a Markdown table",
	} {
		if !strings.Contains(toolText, want) {
			t.Fatalf("primary review tool lost exact human-selection menu contract; missing %q", want)
		}
	}

	commandData, err := os.ReadFile(filepath.Join(root, ".code-harness", "commands", "harness-review.md"))
	if err != nil {
		t.Fatal(err)
	}
	commandText := string(commandData)
	for _, want := range []string{
		"nextAction.requiredMenuText",
		"<runId> options=<optionsHash>",
		"Do not turn this block into a Markdown table",
	} {
		if !strings.Contains(commandText, want) {
			t.Fatalf("ordinary review command lost exact menu rendering contract; missing %q", want)
		}
	}
}

func Test180HostSmokeRunsRealConcurrentFinishRegression(t *testing.T) {
	root := repoRoot180(t)
	data, err := os.ReadFile(filepath.Join(root, ".github", "scripts", "review180-host-smoke.py"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"run_concurrent_finish_scenario",
		"concurrent.futures",
		"REVIEW180_HOST_CONCURRENT_FINISH PASS",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("native Host smoke must exercise concurrent finish calls through the real TypeScript tool/Runtime; missing %q", want)
		}
	}
}

func Test180ActiveOrdinaryReviewInstructionsContainNoExecutableLegacyFlow(t *testing.T) {
	root := repoRoot180(t)
	files := []string{
		filepath.Join(".code-harness", "AGENTS.md"),
		filepath.Join(".code-harness", "bootstrap.md"),
		filepath.Join(".code-harness", "agents", "orchestrator.md"),
		filepath.Join(".code-harness", "skills", "review-code", "SKILL.md"),
	}
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if !strings.Contains(text, "Codea Harness 1.8") || !strings.Contains(text, "codea-review") {
			t.Fatalf("%s does not expose the 1.8 ordinary Review protocol", rel)
		}
		for _, forbidden := range []string{
			"## 1.6.7 单类 Review 入口",
			"review begin",
			"REPORT SUCCEEDED",
			"ReviewUnit",
			"RuleDispatch",
		} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s still embeds executable legacy ordinary-Review flow %q; move legacy Review history out of active instructions", rel, forbidden)
			}
		}
	}
}

func Test180RealModelSmokeProvesSeededIssueAndDurableFinishIdentity(t *testing.T) {
	root := repoRoot180(t)
	scriptBytes, err := os.ReadFile(filepath.Join(root, ".github", "scripts", "review180-real-model-smoke.py"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(scriptBytes)
	for _, want := range []string{
		`--evidence-dir`,
		`seeded_issue_evidence`,
		`finish_runtime`,
		`reportSha256`,
		`hashlib.sha256`,
		`trajectory.json`,
		`manifest.json`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("real-model smoke lacks self-verifying retained evidence contract %q", want)
		}
	}
	workflowBytes, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "runtime-regression-windows-x64.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(workflowBytes)
	for _, want := range []string{
		"review180-real-model-acceptance",
		"review180-real-model-evidence",
		"Upload Codea 1.8 real-model acceptance evidence",
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("Runtime Regression must retain real-model acceptance evidence; missing %q", want)
		}
	}
}
