package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test180HostSmokeFixtureDoesNotStageOpenCodeDependencies(t *testing.T) {
	root := repoRoot180(t)
	data, err := os.ReadFile(filepath.Join(root, ".github", "scripts", "review180-host-smoke.py"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, `.opencode/node_modules/`) {
		t.Fatal("T3 Host smoke fixture must ignore .opencode/node_modules before git baseline staging")
	}
	if !strings.Contains(text, `.gitignore`) {
		t.Fatal("T3 Host smoke fixture must persist the dependency exclusion in the fixture .gitignore")
	}
}
