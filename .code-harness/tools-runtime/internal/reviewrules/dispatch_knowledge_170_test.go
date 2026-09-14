package reviewrules

import (
	"reflect"
	"strings"
	"testing"

	"codea-harness-tools/internal/knowledge"
	"codea-harness-tools/internal/reviewunit"
)

func Test170KnowledgeBusinessDispatchBindsManifestDigest(t *testing.T) {
	rules, catalogSHA := springRules160(t)
	units := sealedUnits160(t, []reviewunit.Unit{{
		ID: "RU-ORDER",
		Files: []reviewunit.FileRef{{Path: "src/main/java/OrderService.java", Role: "Service", Changed: true, Workspace: "current"}},
	}})
	km := knowledge.Manifest170{
		RunID: "run-rule-dispatch",
		ProjectID: "order-service",
		Status: "READY",
		BindingSHA256: strings.Repeat("c", 64),
		Sources: []knowledge.SourceRecord170{{SourceID: "order-rules", Root: "PROJECT", Path: "docs/order.md", Kind: "RULES", Required: true, SHA256: strings.Repeat("d", 64), Status: "READY"}},
		Checks: []knowledge.BusinessCheck170{{ReviewUnitID: "RU-ORDER", RuleKey: "BUSINESS:order-rules:ORDER-STATE-001", SourceID: "order-rules", RuleID: "ORDER-STATE-001", Status: "READY", Reasons: []string{}}},
		Issues: []string{},
	}
	knowledgeSHA, err := knowledge.Digest170(km)
	if err != nil { t.Fatal(err) }

	manifest, err := BuildDispatch170(units, rules, catalogSHA, km)
	if err != nil { t.Fatal(err) }
	if manifest.KnowledgeSHA256 != knowledgeSHA { t.Fatalf("knowledge digest mismatch got=%s want=%s", manifest.KnowledgeSHA256, knowledgeSHA) }
	if manifest.RuleCatalogSHA256 == catalogSHA || len(manifest.RuleCatalogSHA256) != 64 { t.Fatalf("effective catalog must bind built-in + knowledge digest: %s", manifest.RuleCatalogSHA256) }

	var business *Dispatch
	for i := range manifest.Dispatches {
		if manifest.Dispatches[i].RuleID == "BUSINESS:order-rules:ORDER-STATE-001" { business = &manifest.Dispatches[i]; break }
	}
	if business == nil { t.Fatalf("READY business check not dispatched: %+v", manifest.Dispatches) }
	if business.Kind != KindAgent || business.RuleVersion != 1 || business.SeverityDefault != "medium" { t.Fatalf("unexpected BUSINESS dispatch: %+v", *business) }
	if !reflect.DeepEqual(business.RequiredEvidence, []string{"BUSINESS_RULE", "CHANGED_RANGE"}) { t.Fatalf("business evidence=%v", business.RequiredEvidence) }
	if !reflect.DeepEqual(business.DispatchReason, []string{"BUSINESS_RULE:order-rules:ORDER-STATE-001"}) { t.Fatalf("business reason=%v", business.DispatchReason) }
}

func Test170KnowledgeBlockedChecksDoNotBecomeEffectiveDispatch(t *testing.T) {
	rules, catalogSHA := springRules160(t)
	units := sealedUnits160(t, []reviewunit.Unit{{ID: "RU-A", Files: []reviewunit.FileRef{{Path: "src/main/java/A.java", Role: "Code", Changed: true, Workspace: "current"}}}})
	km := knowledge.Manifest170{RunID: "run-rule-dispatch", ProjectID: "p", Status: "PARTIAL", BindingSHA256: strings.Repeat("e", 64), Sources: []knowledge.SourceRecord170{}, Checks: []knowledge.BusinessCheck170{{ReviewUnitID: "RU-A", RuleKey: "BUSINESS:x:R-1", SourceID: "x", RuleID: "R-1", Status: "BLOCKED", Reasons: []string{"KNOWLEDGE_SOURCE_MISSING"}}}, Issues: []string{"KNOWLEDGE_SOURCE_MISSING"}}
	manifest, err := BuildDispatch170(units, rules, catalogSHA, km)
	if err != nil { t.Fatal(err) }
	for _, d := range manifest.Dispatches { if strings.HasPrefix(d.RuleID, "BUSINESS:") { t.Fatalf("blocked knowledge became effective dispatch: %+v", d) } }
}

func Test170KnowledgeLegacyDispatchRemainsKnowledgeFree(t *testing.T) {
	rules, catalogSHA := springRules160(t)
	units := sealedUnits160(t, []reviewunit.Unit{{ID: "RU-A", Files: []reviewunit.FileRef{{Path: "src/main/java/A.java", Role: "Code", Changed: true, Workspace: "current"}}}})
	legacy, err := BuildDispatch(units, rules, catalogSHA)
	if err != nil { t.Fatal(err) }
	if legacy.KnowledgeSHA256 != "" || legacy.RuleCatalogSHA256 != catalogSHA { t.Fatalf("legacy dispatch semantics changed: %+v", legacy) }
}
