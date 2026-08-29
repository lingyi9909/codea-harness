package finding

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestTask6MyBatisDollarBindReviewUnitCarriesControllabilityEvidence(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "review-benchmark", "positive", "03-mybatis-dollar-bind", "case.json")
	var c benchmarkCase160
	if err := json.Unmarshal(task6Read160(t, path), &c); err != nil { t.Fatal(err) }
	if len(c.Chain) < 2 { t.Fatal("03 must encode verified Controller->Mapper chain") }
	if len(c.Symbols) < 2 { t.Fatal("03 must encode verified Controller and Mapper symbol locations") }
	if len(c.Relations) == 0 { t.Fatal("03 must encode verified Mapper resource relation") }

	got := runBenchmarkCase160(t, c)
	var targetUnitID string
	for _, d := range got.Dispatch.Dispatches {
		if d.RuleID == "MYBATIS-BIND-001" { targetUnitID = d.ReviewUnitID; break }
	}
	if targetUnitID == "" { t.Fatal("MYBATIS-BIND-001 must be dispatched") }
	for _, u := range got.Units.Units {
		if u.ID != targetUnitID { continue }
		hasController, hasMapperXML := false, false
		for _, f := range u.Files {
			if f.Path == "src/main/java/com/acme/OrderQueryController.java" { hasController = true }
			if f.Path == "src/main/resources/OrderMapper.xml" { hasMapperXML = true }
		}
		if !hasController || !hasMapperXML { t.Fatalf("MYBATIS-BIND-001 ReviewUnit must contain Controller controllability evidence and Mapper XML together: controller=%v mapperXml=%v", hasController, hasMapperXML) }
		return
	}
	t.Fatalf("ReviewUnit %s not found", targetUnitID)
}

func TestTask6AnalyzeChangeSkillDirectlyDefinesTestPathRoleInvariant(t *testing.T) {
	skill := string(task6Read160(t, filepath.Join("..", "..", "..", "skills", "analyze-change", "SKILL.md")))
	for _, want := range []string{"src/test/java/**/*.java", "-> Test", "src/test/**", "Test"} {
		if !strings.Contains(skill, want) { t.Fatalf("analyze-change/SKILL.md missing Test path-role authority %q", want) }
	}
	if !strings.Contains(skill, "非 `src/test/**`") && !strings.Contains(skill, "非 src/test/**") {
		t.Fatal("analyze-change/SKILL.md must state inverse constraint: non-src/test/** cannot be Test")
	}
}
