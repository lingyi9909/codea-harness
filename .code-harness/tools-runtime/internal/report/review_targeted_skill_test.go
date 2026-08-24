package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewCodeSkillRequiresModeSpecificMachineGate(t *testing.T) {
	path := filepath.Join("..", "..", "..", "skills", "review-code", "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"version: 4",
		"FULL",
		"reviewCoverage.status == COMPLETE",
		"TARGETED",
		"Runtime verified ReviewScopeSelection",
		"Scoped Coverage",
		"Finding.file",
		"verified scopedFiles",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("review-code targeted gate missing %q", want)
		}
	}
}

func TestReviewCodeSkillDoesNotAllowAgentDeclaredCompleteToReplaceScopedGate(t *testing.T) {
	path := filepath.Join("..", "..", "..", "skills", "review-code", "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "TARGETED 不得仅凭 Agent 声明的 reviewCoverage.status == COMPLETE 放行") {
		t.Fatal("review-code must explicitly reject agent-declared COMPLETE as targeted gate")
	}
}
