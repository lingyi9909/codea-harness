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
	if err != nil { t.Fatal(err) }
	return string(b)
}

func TestFixFlowRequiresRuntimeApplyEvidence(t *testing.T) {
	for _, target := range []struct{parts []string; forbidden string}{
		{[]string{"agents","fix-agent.md"}, "apply_approved_patch(fixPlanId, changes)"},
		{[]string{"skills","fix-bug","SKILL.md"}, "apply_approved_patch(fixPlanId, changes)"},
	} {
		text := readHarnessContract(t,target.parts...)
		for _, want := range []string{"apply-request.schema.json", "codea-harness-tools apply --input", "evidence/apply", "diffSha256", "baseSha256"} {
			if !strings.Contains(text,want) { t.Fatalf("%s missing %q",filepath.Join(target.parts...),want) }
		}
		if strings.Contains(text,target.forbidden) { t.Fatalf("%s still treats direct host patch tool as formal apply",filepath.Join(target.parts...)) }
	}
}

func TestTestGenerationRequiresRuntimeApplyEvidence(t *testing.T) {
	for _, parts := range [][]string{{"agents","integration-test-agent.md"},{"skills","generate-integration-tests","SKILL.md"},{"skills","design-integration-tests","SKILL.md"}} {
		text:=readHarnessContract(t,parts...)
		for _,want:=range []string{"unifiedDiff","diffSha256","baseSha256","codea-harness-tools apply --input","planType=TEST"} {
			if !strings.Contains(text,want) { t.Fatalf("%s missing %q",filepath.Join(parts...),want) }
		}
	}
}

func TestOrchestratorDefinesRuntimeApplySafetyGate(t *testing.T) {
	text:=readHarnessContract(t,"agents","orchestrator.md")
	for _,want:=range []string{"Runtime Apply Safety Gate","apply-request.schema.json","apply-result.schema.json","BASE_CHANGED","PLAN_ALREADY_APPLIED","rollbackPerformed","direct host write"} {
		if !strings.Contains(text,want) { t.Fatalf("orchestrator apply gate missing %q",want) }
	}
}
