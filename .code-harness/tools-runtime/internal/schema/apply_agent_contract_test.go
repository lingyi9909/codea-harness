package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readHarnessContract(t *testing.T, parts ...string) string {
	t.Helper()
	p := filepath.Join(append([]string{"..", "..", ".."}, parts...)...)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestFixFlowRequiresRuntimeApplyEvidence(t *testing.T) {
	for _, parts := range [][]string{{"agents", "fix-agent.md"}, {"skills", "fix-bug", "SKILL.md"}} {
		text := readHarnessContract(t, parts...)
		for _, want := range []string{"apply-request.schema.json", "codea-harness-tools seal-apply --input", "sealed-plans", "codea-harness-tools apply --input", "evidence/apply", "diffSha256", "baseSha256", "审批前"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing %q", filepath.Join(parts...), want)
			}
		}
		for _, forbidden := range []string{"\n  - apply_approved_patch\n", "\n  - write_file\n", "\n  - write_code\n"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s still declares direct host write tool %q", filepath.Join(parts...), strings.TrimSpace(forbidden))
			}
		}
		if !strings.Contains(text, "不得") || !strings.Contains(text, "apply_approved_patch") {
			t.Fatalf("%s must explicitly prohibit legacy direct patch completion", filepath.Join(parts...))
		}
	}
}

func TestTestGenerationRequiresRuntimeApplyEvidence(t *testing.T) {
	for _, parts := range [][]string{{"agents", "integration-test-agent.md"}, {"skills", "generate-integration-tests", "SKILL.md"}, {"skills", "design-integration-tests", "SKILL.md"}} {
		text := readHarnessContract(t, parts...)
		for _, want := range []string{"unifiedDiff", "diffSha256", "baseSha256", "codea-harness-tools seal-apply --input", "sealed-plans", "codea-harness-tools apply --input", "planType=TEST", "审批前"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing %q", filepath.Join(parts...), want)
			}
		}
		for _, forbidden := range []string{"\n  - write_test\n", "\n  - write_file\n", "\n  - write_code\n"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s still declares direct host test write tool %q", filepath.Join(parts...), strings.TrimSpace(forbidden))
			}
		}
	}
}

func TestOrchestratorDefinesRuntimeApplySafetyGate(t *testing.T) {
	text := readHarnessContract(t, "agents", "orchestrator.md")
	for _, want := range []string{"Runtime Apply Safety Gate", "apply-request.schema.json", "apply-result.schema.json", "codea-harness-tools seal-apply --input", "sealed-plans", "APPROVAL_IDENTITY_MISMATCH", "BASE_CHANGED", "PLAN_ALREADY_APPLIED", "rollbackPerformed", "direct host write", ".git/**", ".code-harness/**"} {
		if !strings.Contains(text, want) {
			t.Fatalf("orchestrator apply gate missing %q", want)
		}
	}
}
