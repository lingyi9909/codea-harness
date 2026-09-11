package upgrade

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func addReviewerHost164Source(t *testing.T, source string) {
	t.Helper()
	write(t, source, "host/.opencode/agents/reviewer.md", "reviewer-host-1.6.4\n")
	write(t, source, "host/.opencode/commands/harness-review-reviewer.md", "reviewer-command-1.6.4\n")
	write(t, source, "host/.opencode/tools/codea-reviewer-submit.ts", "reviewer-submit-tool-1.6.4\n")
}

func Test164OfficialUpgradeInstallsReviewerHostTransactionally(t *testing.T) {
	source, target := make163To164Pair(t, task164Real163Config)
	addReviewerHost164Source(t, source)
	projectRoot := filepath.Dir(target)
	write(t, projectRoot, ".opencode/user-settings.json", "keep-user-host-config\n")

	result := Run(Options{SourceDir: source, TargetDir: target})
	if result.Status != StatusUpgraded {
		t.Fatalf("result=%+v", result)
	}
	for rel, want := range map[string]string{
		".opencode/agents/reviewer.md":                  "reviewer-host-1.6.4\n",
		".opencode/commands/harness-review-reviewer.md": "reviewer-command-1.6.4\n",
		".opencode/tools/codea-reviewer-submit.ts":      "reviewer-submit-tool-1.6.4\n",
		".opencode/user-settings.json":                  "keep-user-host-config\n",
	} {
		got, err := os.ReadFile(filepath.Join(projectRoot, filepath.FromSlash(rel)))
		if err != nil || string(got) != want {
			t.Fatalf("%s got=%q err=%v", rel, got, err)
		}
	}
	for _, rel := range reviewerHostFiles164 {
		if !contains(result.UpdatedFiles, rel) {
			t.Fatalf("Reviewer Host transaction evidence missing %s: %v", rel, result.UpdatedFiles)
		}
	}
}

func Test164OfficialUpgradeRejectsUnknownReviewerHostConflictBeforeFrameworkWrite(t *testing.T) {
	source, target := make163To164Pair(t, task164Real163Config)
	addReviewerHost164Source(t, source)
	projectRoot := filepath.Dir(target)
	write(t, projectRoot, ".opencode/agents/reviewer.md", "user-owned-reviewer\n")
	beforeVersion, _ := os.ReadFile(filepath.Join(target, "VERSION"))
	beforeAgents, _ := os.ReadFile(filepath.Join(target, "AGENTS.md"))

	result := Run(Options{SourceDir: source, TargetDir: target})
	if result.Status != StatusManualActionRequired || result.RollbackPerformed {
		t.Fatalf("result=%+v", result)
	}
	if len(result.Errors) == 0 || !strings.Contains(result.Errors[0], "Reviewer Host target conflict") {
		t.Fatalf("unexpected conflict error: %v", result.Errors)
	}
	afterVersion, _ := os.ReadFile(filepath.Join(target, "VERSION"))
	afterAgents, _ := os.ReadFile(filepath.Join(target, "AGENTS.md"))
	if string(afterVersion) != string(beforeVersion) || string(afterAgents) != string(beforeAgents) {
		t.Fatal("Host conflict modified framework before fail-closed result")
	}
	got, _ := os.ReadFile(filepath.Join(projectRoot, ".opencode", "agents", "reviewer.md"))
	if string(got) != "user-owned-reviewer\n" {
		t.Fatalf("user Reviewer Host was changed: %q", got)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("failed upgrade consumed source: %v", err)
	}
}

func Test164OfficialUpgradeRollsBackFrameworkAndPartialReviewerHostCommit(t *testing.T) {
	source, target := make163To164Pair(t, task164Real163Config)
	addReviewerHost164Source(t, source)
	projectRoot := filepath.Dir(target)
	write(t, projectRoot, ".opencode/user-settings.json", "keep-user-host-config\n")
	beforeVersion, _ := os.ReadFile(filepath.Join(target, "VERSION"))
	beforeAgents, _ := os.ReadFile(filepath.Join(target, "AGENTS.md"))

	previousHook := reviewerHostInstallHook
	reviewerHostInstallHook = func(index int, rel string) error {
		if index == 2 {
			return errors.New("injected Reviewer Host submission-tool commit failure")
		}
		return nil
	}
	defer func() { reviewerHostInstallHook = previousHook }()

	result := Run(Options{SourceDir: source, TargetDir: target})
	if result.Status != StatusUpgradeFailed || !result.RollbackPerformed {
		t.Fatalf("result=%+v", result)
	}
	afterVersion, _ := os.ReadFile(filepath.Join(target, "VERSION"))
	afterAgents, _ := os.ReadFile(filepath.Join(target, "AGENTS.md"))
	if string(afterVersion) != string(beforeVersion) || string(afterAgents) != string(beforeAgents) {
		t.Fatal("framework was not restored after Host commit failure")
	}
	for _, rel := range reviewerHostFiles164 {
		if _, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Fatalf("partial Reviewer Host file survived rollback: %s err=%v", rel, err)
		}
	}
	userConfig, err := os.ReadFile(filepath.Join(projectRoot, ".opencode", "user-settings.json"))
	if err != nil || string(userConfig) != "keep-user-host-config\n" {
		t.Fatalf("unrelated OpenCode config changed: %q err=%v", userConfig, err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("failed upgrade consumed source: %v", err)
	}
}
