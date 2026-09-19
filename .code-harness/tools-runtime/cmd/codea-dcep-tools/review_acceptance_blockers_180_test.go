package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	root := repoRoot180(t)
	data, err := os.ReadFile(filepath.Join(root, ".github", "scripts", "review180-real-model-e2e.py"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`@PreAuthorize("hasAuthority('ORDER_WRITE')")`,
		`@AuthenticationPrincipal(expression = "tenantId") String tenantId`,
		`if (tenantId == null || tenantId.isBlank())`,
		`if (id <= 0)`,
		`if (!"PAID".equals(status) && !"CANCELLED".equals(status))`,
		`int updated = mapper.updateStatus(tenantId, id, status);`,
		`public void voidOrder`,
		`command_args.append("OrderController")`,
		`host.require(scope.get("selectedIds") == ["C1"]`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("final real-model fixture lost navigation/selection contract %q", want)
		}
	}
	if strings.Contains(text, "service.cancel(") {
		t.Fatal("multi-chain fixture must keep updateStatus lexically first so seeded issue remains C1")
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
