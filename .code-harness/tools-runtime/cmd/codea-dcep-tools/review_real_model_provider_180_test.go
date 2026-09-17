package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test180RealModelSmokeUsesUnauthenticatedOpenCodeFreeModel(t *testing.T) {
	root := repoRoot180(t)

	scriptPath := filepath.Join(root, ".github", "scripts", "review180-real-model-smoke.py")
	scriptBytes, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	script := string(scriptBytes)

	for _, want := range []string{
		"https://opencode.ai/inference/openai/v1",
		"mimo-v2.5-free",
		"opencode-free/mimo-v2.5-free",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("real-model smoke must use the unauthenticated OpenCode free-model endpoint; missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"models.github.ai",
		"githubmodels/openai/gpt-4.1",
		"GITHUB_TOKEN",
	} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("real-model smoke must not depend on retired GitHub Models; found %q", forbidden)
		}
	}

	workflowPath := filepath.Join(root, ".github", "workflows", "runtime-regression-windows-x64.yml")
	workflowBytes, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(workflowBytes)
	for _, forbidden := range []string{"models: read", "GITHUB_TOKEN:"} {
		if strings.Contains(workflow, forbidden) {
			t.Fatalf("runtime regression must not request retired GitHub Models auth; found %q", forbidden)
		}
	}
}
