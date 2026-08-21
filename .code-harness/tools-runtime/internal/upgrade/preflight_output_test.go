package upgrade

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreflightJSONListsAllMissingReleaseBinariesAndSourceHint(t *testing.T) {
	source, target := make13Pair(t, validConfig("review:\n  baseRef: origin/develop\n  includeWorkingTree: true\n"))
	if err := os.Remove(filepath.Join(source, "bin", "codea-harness-tools.exe")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(source, "bin", "ast-grep.exe")); err != nil {
		t.Fatal(err)
	}

	before, err := os.ReadFile(filepath.Join(target, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(source)
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	result := Run(Options{
		SourceDir: ".code-harness-upgrade",
		TargetDir: ".code-harness",
		Refs:      StaticRefs{RemoteBranches: []string{"origin/develop"}},
	})
	if result.Status != StatusManualActionRequired {
		t.Fatalf("result=%+v", result)
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, want := range []string{
		"missing: bin/codea-harness-tools.exe",
		"missing: bin/ast-grep.exe",
		"GitHub Source Code",
		"windows-x64-upgrade.zip",
		".code-harness-upgrade/VERSION",
		".code-harness-upgrade/bin/codea-harness-tools.exe",
		".code-harness-upgrade/bin/ast-grep.exe",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("preflight output missing %q: %s", want, text)
		}
	}

	after, err := os.ReadFile(filepath.Join(target, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("preflight failure modified target files")
	}
}
