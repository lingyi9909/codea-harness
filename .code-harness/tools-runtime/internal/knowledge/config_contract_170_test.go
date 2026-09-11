package knowledge

import (
	"os"
	"testing"

	"codea-harness-tools/internal/schema"
)

func readContract170(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("../../../contracts/" + name)
	if err != nil { t.Fatal(err) }
	return data
}

func Test170ContextConfigContract(t *testing.T) {
	s := readContract170(t, "context-config.schema.json")
	valid := []byte("version: 1\nprojectId: order-service\nteamRoot: D:/team/total-repo\nsources:\n  - id: order-state-rule\n    root: PROJECT\n    path: docs/business/order-state.md\n    kind: RULES\n    required: true\n")
	if err := schema.ValidateYAML(s, valid); err != nil { t.Fatalf("valid context config rejected: %v", err) }
	cases := [][]byte{
		[]byte("version: 1\nprojectId: order-service\nsources: []\nextra: true\n"),
		[]byte("version: 1\nprojectId: order-service\nprojectId: duplicate\nsources: []\n"),
		[]byte("version: 1\nsources: []\n"),
		[]byte("version: 1\nprojectId: order-service\nsources:\n  - id: rule-a\n    root: PROJECT\n    path: ../private.md\n    kind: RULES\n    required: true\n"),
	}
	for _, raw := range cases { if err := schema.ValidateYAML(s, raw); err == nil { t.Fatalf("invalid context config accepted: %s", raw) } }
}

func Test170BusinessRuleContract(t *testing.T) {
	s := readContract170(t, "business-rule.schema.json")
	valid := []byte("ruleId: ORDER-STATE-001\nprojectId: order-service\nstatus: ACTIVE\nversion: '1'\nowner: order-team\nsource: requirements/order-state.md\napprovalRef: reviews/order-state-approved.md\nappliesTo:\n  paths: [src/main/java/example/order/]\n  entryPoints: []\nsupersedes: []\nexceptions: []\n")
	if err := schema.ValidateYAML(s, valid); err != nil { t.Fatalf("valid business rule rejected: %v", err) }
	missingApproval := []byte("ruleId: ORDER-STATE-001\nprojectId: order-service\nstatus: ACTIVE\nversion: '1'\nowner: order-team\nsource: requirements/order-state.md\nappliesTo:\n  paths: [src/main/java/example/order/]\n  entryPoints: []\nsupersedes: []\nexceptions: []\n")
	if err := schema.ValidateYAML(s, missingApproval); err == nil { t.Fatal("rule without approvalRef accepted") }
}

func Test170KnowledgeAndChecksJSONContracts(t *testing.T) {
	request := readContract170(t, "review-knowledge-request.schema.json")
	if err := schema.ValidateJSON(request, []byte(`{"runId":"review-case"}`)); err != nil { t.Fatal(err) }
	if err := schema.ValidateJSON(request, []byte(`{"runId":"review-case","path":"docs"}`)); err == nil { t.Fatal("knowledge request accepted extra field") }

	manifest := readContract170(t, "review-knowledge.schema.json")
	validManifest := []byte(`{"runId":"review-case","projectId":"order-service","status":"READY","bindingSha256":"abc","sources":[],"checks":[],"issues":[]}`)
	if err := schema.ValidateJSON(manifest, validManifest); err != nil { t.Fatalf("valid knowledge manifest rejected: %v", err) }
	if err := schema.ValidateJSON(manifest, []byte(`[]`)); err == nil { t.Fatal("knowledge manifest accepted array envelope") }

	checks := readContract170(t, "review-checks.schema.json")
	validChecks := []byte(`[{"reviewUnitId":"unit-a","ruleId":"BUSINESS:rules:ORDER-1","status":"COMPLETED","reason":"","sourceIds":["rules"]}]`)
	if err := schema.ValidateJSON(checks, validChecks); err != nil { t.Fatalf("valid review checks rejected: %v", err) }
	if err := schema.ValidateJSON(checks, []byte(`{"checks":[]}`)); err == nil { t.Fatal("review checks accepted object envelope") }
}
