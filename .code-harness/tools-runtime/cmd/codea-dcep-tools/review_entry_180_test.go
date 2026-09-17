package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test180PrimaryReviewEntryStartsReportBeforeAgent(t *testing.T) {
	root := repoRoot180(t)
	data, err := os.ReadFile(filepath.Join(root, ".code-harness", "commands", "harness-review.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "review begin") {
		t.Fatal("formal 1.8 review entry still uses legacy review begin")
	}
	if !strings.Contains(text, "!`./.code-harness/bin/codea-dcep-tools.exe review start`") {
		t.Fatal("formal review command must synchronously execute fixed review start before the model turn")
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "!`") && strings.Contains(line, "$ARGUMENTS") {
			t.Fatalf("user target must not be interpolated into shell command: %q", line)
		}
	}
	if !strings.Contains(text, "Review target: $ARGUMENTS") {
		t.Fatal("user target must remain ordinary prompt text")
	}
}

func Test180PrimaryReviewUsesSingleStructuredTool(t *testing.T) {
	root := repoRoot180(t)
	data, err := os.ReadFile(filepath.Join(root, ".code-harness", "tools", "codea-review.ts"))
	if err != nil {
		t.Fatalf("1.8 structured review tool missing: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"prepare", "select", "finish", "runId", "intent", "selection", "result",
		"context.sessionID", "context.messageID", "execFile",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("codea-review tool missing %q", want)
		}
	}
	argsStart := strings.Index(text, "args:")
	execStart := strings.Index(text, "async execute")
	if argsStart < 0 || execStart < 0 || execStart <= argsStart {
		t.Fatal("cannot locate tool args/execute boundary")
	}
	argsText := text[argsStart:execStart]
	if strings.Contains(argsText, "sessionID") || strings.Contains(argsText, "sessionId") || strings.Contains(argsText, "messageID") || strings.Contains(argsText, "messageId") || strings.Contains(argsText, "userConfirmed") {
		t.Fatal("HostTurn/user confirmation must not be model-fillable tool args")
	}
	for _, forbidden := range []string{"cmd /c", "powershell -Command", "bash -c", "shell: true"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("structured tool must use native argument arrays, found %q", forbidden)
		}
	}
}

func Test180PrimaryReviewFinishPathDoesNotUseOpaqueHostID(t *testing.T) {
	root := repoRoot180(t)
	data, err := os.ReadFile(filepath.Join(root, ".code-harness", "tools", "codea-review.ts"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "finish-${context.messageID") || strings.Contains(text, "finish-${context.sessionID") {
		t.Fatal("opaque Host ids must never become Windows path components")
	}
	if !strings.Contains(text, `path.resolve(requestsRoot, "finish.json")`) {
		t.Fatal("finish request must use a deterministic Windows-safe path")
	}
}

func Test180PrimaryReviewInstructionsDoNotRequireLegacyReviewerAuthority(t *testing.T) {
	root := repoRoot180(t)
	files := []string{
		filepath.Join(".code-harness", "commands", "harness-review.md"),
		filepath.Join(".code-harness", "AGENTS.md"),
		filepath.Join(".code-harness", "agents", "orchestrator.md"),
		filepath.Join(".code-harness", "skills", "review-code", "SKILL.md"),
	}
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if !strings.Contains(text, "1.8") || !strings.Contains(text, "codea-review") {
			t.Fatalf("%s does not expose the 1.8 primary review path", rel)
		}
		prefix := text
		if marker := strings.Index(text, "## 历史"); marker >= 0 {
			prefix = text[:marker]
		}
		for _, old := range []string{"codea-reviewer-submit", "analysis certify", "ReviewUnit", "RuleDispatch", "REPORT SUCCEEDED"} {
			if strings.Contains(prefix, old) {
				t.Fatalf("%s still requires legacy ordinary-review step %q before historical isolation", rel, old)
			}
	}
}

func Test180SmokeDriversUseNativeCommandPathAndDoNotScriptFinish(t *testing.T) {
	root := repoRoot180(t)
	files := []string{
		filepath.Join(".github", "scripts", "review180-host-smoke.py"),
		filepath.Join(".github", "scripts", "review180-real-model-smoke.py"),
	}
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("T3 smoke driver missing %s: %v", rel, err)
		}
		text := string(data)
		for _, want := range []string{
			"harness-review",
			"OrderController",
			"finish",
			"export",
			`"--command", "harness-review", "OrderController"`,
			`args.command_source.parent.parent / "agents" / "orchestrator.md"`,
			`project / ".opencode" / "agents"`,
			`project / ".opencode" / "agents" / "orchestrator.md"`,
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing %q", rel, want)
			}
		}
		if strings.Contains(text, `run + ["/harness-review", "OrderController"]`) {
			t.Fatalf("%s sends /harness-review as ordinary prompt instead of native OpenCode command", rel)
		}
		for _, forbidden := range []string{"analysis certify", "report review", "review finish --", "rt(certify", "rt(report"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s must observe model behavior, not execute Runtime business step %q", rel, forbidden)
			}
		}
	}
}

func repoRoot180(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}
