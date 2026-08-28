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

type task6FixtureExpected160 struct {
	RuleDispatched bool `json:"ruleDispatched"`
}

type task6Fixture160 struct {
	ID       string                  `json:"id"`
	Class    string                  `json:"class"`
	RuleID   string                  `json:"ruleId"`
	Files    []task6FixtureFile160   `json:"files"`
	Expected task6FixtureExpected160 `json:"expected"`
}

func TestTask6BenchmarkRunnerUsesRealReviewUnitBuild(t *testing.T) {
	text := string(task6Read160(t, "benchmark_test.go"))
	if !strings.Contains(text, "reviewunit.Build(") {
		t.Fatal("24-case runner must call real reviewunit.Build")
	}
	if strings.Contains(text, `ID: "RU-BENCH"`) || strings.Contains(text, `ReviewUnitID: "RU-BENCH"`) {
		t.Fatal("24-case runner must not handcraft RU-BENCH authority")
	}
}

func TestTask6PositiveAndNegativeFixturesCarrySourceGroundTruth(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "review-benchmark")
	for _, class := range []string{"positive", "negative"} {
		entries, err := os.ReadDir(filepath.Join(root, class))
		if err != nil { t.Fatal(err) }
		for _, entry := range entries {
			if !entry.IsDir() { continue }
			var fixture task6Fixture160
			if err := json.Unmarshal(task6Read160(t, filepath.Join(root, class, entry.Name(), "case.json")), &fixture); err != nil { t.Fatal(err) }
			hasChangedCurrent := false
			hasRealDiff := false
			for _, file := range fixture.Files {
				workspace := strings.TrimSpace(file.Workspace)
				if workspace == "" { workspace = "current" }
				if workspace != "current" || !file.Changed { continue }
				hasChangedCurrent = true
				if file.BeforeContent != "" && file.BeforeContent != file.Content { hasRealDiff = true }
			}
			if class == "positive" && !hasRealDiff {
				t.Fatalf("positive/%s must encode a real before->after source diff; proposals cannot define ground truth", fixture.ID)
			}
			if class == "negative" {
				if hasChangedCurrent && !hasRealDiff {
					t.Fatalf("negative/%s changed source must encode a real before->after diff", fixture.ID)
				}
				if !hasChangedCurrent && fixture.Expected.RuleDispatched {
					t.Fatalf("negative/%s has no changed current source but still expects rule dispatch", fixture.ID)
				}
			}
		}
	}
}

func TestTask6ProblemFixturesEncodeRuleGroundTruthInSource(t *testing.T) {
	fixtures := map[string]struct{ before, after string }{
		"02-mybatis-tenant-removed": {"tenant_id", "UPDATE orders SET"},
		"03-mybatis-dollar-bind": {"ORDER BY created_at", "${sort}"},
		"05-tx-self-invocation": {"proxy.inner", "inner();"},
		"06-tx-checked-rollback": {"rollbackFor = Exception.class", "throws Exception"},
		"07-tx-readonly-write": {"readOnly = false", "readOnly = true"},
		"08-auth-weakened": {"@ProjectAuthorize", "service.submit();"},
		"09-validation-omitted": {"if (amount <= 0)", "repository.save"},
	}
	root := filepath.Join("..", "..", "testdata", "review-benchmark", "positive")
	for id, want := range fixtures {
		var fixture task6Fixture160
		if err := json.Unmarshal(task6Read160(t, filepath.Join(root, id, "case.json")), &fixture); err != nil { t.Fatal(err) }
		before, after := "", ""
		for _, file := range fixture.Files {
			if file.Changed { before += file.BeforeContent; after += file.Content }
		}
		if !strings.Contains(before, want.before) || !strings.Contains(after, want.after) {
			t.Fatalf("%s source ground truth missing: before must contain %q and after %q", id, want.before, want.after)
		}
	}
}

func TestTask6TestValidityUsesDedicatedAuthorityWithoutChangingSpringTen(t *testing.T) {
	var fixture task6Fixture160
	path := filepath.Join("..", "..", "testdata", "review-benchmark", "positive", "11-test-validity", "case.json")
	if err := json.Unmarshal(task6Read160(t, path), &fixture); err != nil { t.Fatal(err) }
	if fixture.RuleID != "TEST-VALIDITY-001" { t.Fatalf("11-test-validity ruleId=%q want TEST-VALIDITY-001", fixture.RuleID) }
	catalog := string(task6Read160(t, filepath.Join("..", "..", "..", "review-rules", "spring-v1.yaml")))
	if strings.Count(catalog, "  - id: ") != 10 { t.Fatalf("Spring Rule Pack must remain exactly 10 rules") }
	if strings.Contains(catalog, "TEST-VALIDITY-001") { t.Fatal("Test Validity authority must not be added to spring-v1.yaml") }
	dispatch := string(task6Read160(t, filepath.Join("..", "reviewrules", "dispatch.go")))
	if !strings.Contains(dispatch, "TEST-VALIDITY-001") || !strings.Contains(dispatch, `changedRoles["Test"]`) {
		t.Fatal("Runtime must establish dedicated Test Validity dispatch authority for changed Test ReviewUnits")
	}
	analysisSchema := string(task6Read160(t, filepath.Join("..", "..", "..", "contracts", "change-analysis.schema.json")))
	reviewUnitSchema := string(task6Read160(t, filepath.Join("..", "..", "..", "contracts", "review-unit.schema.json")))
	if !strings.Contains(analysisSchema, `"Test"`) || !strings.Contains(reviewUnitSchema, `"Test"`) {
		t.Fatal("Test Validity authority requires Test to be a legal fileRole in analysis and ReviewUnit contracts")
	}
}

func TestTask6WindowsDependencySentinelRequiresExactMachineCodeAndEvidence(t *testing.T) {
	script := string(task6Read160(t, filepath.Join("..", "..", "..", "..", ".github", "scripts", "task160-real-review-precision-regression.ps1")))
	for _, want := range []string{"workspaceDependencies", "company-framework", "FINDING_DEPENDENCY_SCOPE_FORBIDDEN", "P-DEPENDENCY"} {
		if !strings.Contains(script, want) { t.Fatalf("Windows P-DEPENDENCY missing real authority/assertion %q", want) }
	}
}

func TestTask6WorkflowGoVersionMatchesGoMod(t *testing.T) {
	goMod := string(task6Read160(t, filepath.Join("..", "..", "go.mod")))
	match := regexp.MustCompile(`(?m)^go\s+([0-9]+\.[0-9]+(?:\.[0-9]+)?)\s*$`).FindStringSubmatch(goMod)
	if len(match) != 2 { t.Fatalf("cannot read go version from go.mod") }
	workflow := string(task6Read160(t, filepath.Join("..", "..", "..", "..", ".github", "workflows", "task160-review-precision.yml")))
	if !strings.Contains(workflow, "go-version: '"+match[1]+"'") { t.Fatalf("task160-review-precision.yml must use go.mod version %s", match[1]) }
}

func task6Read160(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil { t.Fatalf("read %s: %v", path, err) }
	return data
}
