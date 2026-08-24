package chain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readHarnessContractFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestDiscoverChainSkillLocksLazyRuntimeContract(t *testing.T) {
	text := readHarnessContractFile(t, "skills/discover-chain/SKILL.md")
	for _, needle := range []string{
		"harness chain discover",
		"harness chain discover OrderController",
		"harness chain discover OrderController.approve",
		"codea-harness-tools chain discover --input",
		".code-harness/runs/<runId>/analysis/discovered-chains/",
		"不得写入 `.code-harness/chains/**`",
		"PARTIAL",
		"verified core path 完全一致",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("discover-chain contract missing %q", needle)
		}
	}
}

func TestAnalyzeChangeAndReviewerKeepDiscoveryEvidenceBounded(t *testing.T) {
	for _, rel := range []string{"skills/analyze-change/SKILL.md", "agents/reviewer.md"} {
		text := readHarnessContractFile(t, rel)
		for _, needle := range []string{
			"Lazy Chain Discovery",
			"ChangeAnalysis.symbolLocations[]",
			"ChangeAnalysis.resourceRelations[]",
			"生产 Controller Method",
			"不得根据类名后缀",
			"PARTIAL",
			"runs/<runId>/analysis/discovered-chains/",
		} {
			if !strings.Contains(text, needle) {
				t.Fatalf("%s discovery contract missing %q", rel, needle)
			}
		}
	}
}
