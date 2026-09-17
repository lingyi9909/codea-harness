package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test180RealModelSmokeUsesConfiguredSecretModel(t *testing.T) {
	root := repoRoot180(t)

	scriptPath := filepath.Join(root, ".github", "scripts", "review180-real-model-smoke.py")
	scriptBytes, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	script := string(scriptBytes)
	for _, want := range []string{
		"TASK15_OPENAI_BASE_URL",
		"TASK15_OPENAI_API_KEY",
		"deepseek-v4-pro",
		"task15secret/deepseek-v4-pro",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("real-model smoke must use the configured secret model; missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"models.github.ai",
		"githubmodels/openai/gpt-4.1",
		"GITHUB_TOKEN",
		"opencode.ai/inference/openai/v1",
		"mimo-v2.5-free",
	} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("real-model smoke must not depend on retired or anonymous providers; found %q", forbidden)
		}
	}

	workflowPath := filepath.Join(root, ".github", "workflows", "runtime-regression-windows-x64.yml")
	workflowBytes, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(workflowBytes)
	for _, want := range []string{
		"TASK15_OPENAI_BASE_URL: ${{ secrets.TASK15_OPENAI_BASE_URL }}",
		"TASK15_OPENAI_API_KEY: ${{ secrets.TASK15_OPENAI_API_KEY }}",
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("runtime regression must inject configured secret model credentials; missing %q", want)
		}
	}
	for _, forbidden := range []string{"models: read", "GITHUB_TOKEN:"} {
		if strings.Contains(workflow, forbidden) {
			t.Fatalf("runtime regression must not request retired GitHub Models auth; found %q", forbidden)
		}
	}
}
