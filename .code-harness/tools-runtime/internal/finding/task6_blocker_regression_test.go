package finding

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type task6FixtureFile160 struct {
	Path          string `json:"path"`
	Workspace     string `json:"workspace"`
	Changed       bool   `json:"changed"`
	BeforeContent string `json:"beforeContent"`
	Content       string `json:"content"`
}

type task6FixtureExpected160 struct { RuleDispatched bool `json:"ruleDispatched"` }
type task6Fixture160 struct {
	ID string `json:"id"`; Class string `json:"class"`; RuleID string `json:"ruleId"`
	Files []task6FixtureFile160 `json:"files"`; Expected task6FixtureExpected160 `json:"expected"`
}

func TestTask6BenchmarkRunnerUsesRealReviewUnitBuild(t *testing.T) {
	text := string(task6Read160(t, "benchmark_test.go"))
	if !strings.Contains(text, "reviewunit.Build(") { t.Fatal("24-case runner must call real reviewunit.Build") }
	if strings.Contains(text, `ID: "RU-BENCH"`) || strings.Contains(text, `ReviewUnitID: "RU-BENCH"`) { t.Fatal("24-case runner must not handcraft RU-BENCH authority") }
}

func TestTask6PositiveAndNegativeFixturesCarrySourceGroundTruth(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "review-benchmark")
	for _, class := range []string{"positive", "negative"} {
		entries, err := os.ReadDir(filepath.Join(root, class)); if err != nil { t.Fatal(err) }
		for _, entry := range entries {
			if !entry.IsDir() { continue }
			var fixture task6Fixture160
			if err := json.Unmarshal(task6Read160(t, filepath.Join(root, class, entry.Name(), "case.json")), &fixture); err != nil { t.Fatal(err) }
			hasChangedCurrent, hasRealDiff := false, false
			for _, file := range fixture.Files {
				workspace := strings.TrimSpace(file.Workspace); if workspace == "" { workspace = "current" }
				if workspace != "current" || !file.Changed { continue }
				hasChangedCurrent = true
				if file.BeforeContent != "" && file.BeforeContent != file.Content { hasRealDiff = true }
			}
			if class == "positive" && !hasRealDiff { t.Fatalf("positive/%s must encode a real before->after source diff", fixture.ID) }
			if class == "negative" && hasChangedCurrent && !hasRealDiff { t.Fatalf("negative/%s changed source must encode a real before->after diff", fixture.ID) }
		}
	}
}

func TestTask6ProblemFixturesEncodeRuleGroundTruthInSource(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "review-benchmark", "positive")
	checks := map[string]func(string,string) bool{
		"03-mybatis-dollar-bind": func(before, after string) bool { return strings.Contains(after, "${sort}") && (strings.Contains(after, "#{sort}") || strings.Contains(after, "sort")) && (strings.Contains(after, "@RequestParam") || strings.Contains(after, "request.get") || strings.Contains(after, "String sort")) },
		"05-tx-self-invocation": func(before, after string) bool { return strings.Contains(after, "@Transactional") && strings.Contains(after, "inner();") && (strings.Contains(before, "proxy.inner();") || strings.Contains(before, "self.inner();")) },
		"11-test-validity": func(before, after string) bool { return (strings.Contains(before, "assertEquals(") || strings.Contains(before, "assertThat(") || strings.Contains(before, "assertTrue(")) && !strings.Contains(after, "assertEquals(") && !strings.Contains(after, "assertThat(") && !strings.Contains(after, "assertTrue(") },
	}
	for id, check := range checks {
		var fixture task6Fixture160
		if err := json.Unmarshal(task6Read160(t, filepath.Join(root, id, "case.json")), &fixture); err != nil { t.Fatal(err) }
		before, after := "", ""; for _, f := range fixture.Files { if f.Changed { before += f.BeforeContent; after += f.Content } }
		if !check(before, after) { t.Fatalf("%s source ground truth is not semantically sufficient", id) }
	}
}

func TestTask6TestValidityPathRoleIsRuntimeOwned(t *testing.T) {
	build := string(task6Read160(t, filepath.Join("..", "reviewunit", "build.go")))
	if !strings.Contains(build, "expectedPathRole160") || !strings.Contains(build, `src/test/`) || !strings.Contains(build, `"Test"`) { t.Fatal("ReviewUnit Runtime must machine-enforce src/test/** <-> Test role") }
	skill := string(task6Read160(t, filepath.Join("..", "..", "..", "skills", "analyze-change", "SKILL.md")))
	if !strings.Contains(skill, "src/test/") || !strings.Contains(skill, "-> Test") { t.Fatal("analyze-change Skill must map src/test/** to Test") }
}

func TestTask6DeterminismComparesCanonicalArtifacts(t *testing.T) {
	text := string(task6Read160(t, "benchmark_test.go"))
	for _, want := range []string{"reviewunit.CanonicalBytes(left.Units)", "reviewrules.CanonicalBytes(left.Dispatch)", "canonicalCertifiedSet160(left.Set, true)", "left.Units.SHA256", "left.Dispatch.SHA256", "left.Set.SHA256"} {
		if !strings.Contains(text, want) { t.Fatalf("determinism benchmark must compare real canonical artifact authority: missing %s", want) }
	}
}

func TestTask6BenchmarkUsesFormalPrecisionAndSemanticDedupIdentity(t *testing.T) {
	text := string(task6Read160(t, "benchmark_test.go"))
	if !strings.Contains(text, "float64(truePositive) / float64(truePositive+falsePositive)") { t.Fatal("Precision must use TP/(TP+FP)") }
	if !strings.Contains(text, "semanticIdentity160(") || !strings.Contains(text, "EvidenceDigest") { t.Fatal("DuplicateRate must reuse formal semantic identity including evidenceDigest") }
}

func TestTask6TestValidityUsesDedicatedAuthorityWithoutChangingSpringTen(t *testing.T) {
	var fixture task6Fixture160; path := filepath.Join("..", "..", "testdata", "review-benchmark", "positive", "11-test-validity", "case.json")
	if err := json.Unmarshal(task6Read160(t, path), &fixture); err != nil { t.Fatal(err) }
	if fixture.RuleID != "TEST-VALIDITY-001" { t.Fatalf("11-test-validity ruleId=%q", fixture.RuleID) }
	catalog := string(task6Read160(t, filepath.Join("..", "..", "..", "review-rules", "spring-v1.yaml")))
	if strings.Count(catalog, "  - id: ") != 10 || strings.Contains(catalog, "TEST-VALIDITY-001") { t.Fatal("Spring Rule Pack must remain exactly 10 rules") }
}

func TestTask6WindowsDependencySentinelRequiresExactMachineCodeAndEvidence(t *testing.T) {
	script := string(task6Read160(t, filepath.Join("..", "..", "..", "..", ".github", "scripts", "task160-real-review-precision-regression.ps1")))
	for _, want := range []string{"workspaceDependencies", "company-framework", "FINDING_DEPENDENCY_SCOPE_FORBIDDEN", "P-DEPENDENCY"} { if !strings.Contains(script, want) { t.Fatalf("Windows P-DEPENDENCY missing %q", want) } }
}

func TestTask6WorkflowGoVersionMatchesGoMod(t *testing.T) {
	goMod := string(task6Read160(t, filepath.Join("..", "..", "go.mod"))); match := regexp.MustCompile(`(?m)^go\s+([0-9]+\.[0-9]+(?:\.[0-9]+)?)\s*$`).FindStringSubmatch(goMod)
	if len(match) != 2 { t.Fatal("cannot read go version") }
	workflow := string(task6Read160(t, filepath.Join("..", "..", "..", "..", ".github", "workflows", "task160-review-precision.yml")))
	if !strings.Contains(workflow, "go-version: '"+match[1]+"'") { t.Fatalf("workflow must use %s", match[1]) }
}

func task6Read160(t *testing.T, path string) []byte { t.Helper(); data, err := os.ReadFile(path); if err != nil { t.Fatalf("read %s: %v", path, err) }; return data }
