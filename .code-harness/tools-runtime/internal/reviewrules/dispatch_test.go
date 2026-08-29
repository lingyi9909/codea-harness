package reviewrules

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"codea-harness-tools/internal/reviewunit"
	"codea-harness-tools/internal/schema"
)

func TestDispatchMapperXmlGetsOnlyRelevantMyBatisRules(t *testing.T) {
	rules, catalogSHA := springRules160(t)
	units := sealedUnits160(t, []reviewunit.Unit{{
		ID: "RU-MAPPER-XML",
		Files: []reviewunit.FileRef{{Path: "src/main/resources/OrderMapper.xml", Role: "MapperXml", Changed: true, Workspace: "current"}},
		ChangedHunks: []reviewunit.HunkRef{{Path: "src/main/resources/OrderMapper.xml", NewStart: 40, NewLines: 8}},
	}})
	manifest, err := BuildDispatch(units, rules, catalogSHA)
	if err != nil {
		t.Fatalf("build dispatch: %v", err)
	}
	got := ruleIDs160(manifest, "RU-MAPPER-XML")
	want := []string{"MYBATIS-BIND-001", "MYBATIS-CONTRACT-001", "MYBATIS-ISOLATION-001", "MYBATIS-SQL-001"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MapperXml dispatch mismatch: got %v want %v", got, want)
	}
}

func TestDispatchTransactionalServiceGetsTxRules(t *testing.T) {
	rules, catalogSHA := springRules160(t)
	units := sealedUnits160(t, []reviewunit.Unit{{
		ID: "RU-SERVICE",
		EntryPoint: "OrderController.approve",
		Chain: []string{"OrderController.approve", "OrderService.approve", "OrderServiceImpl.approve"},
		Files: []reviewunit.FileRef{{Path: "src/main/java/OrderServiceImpl.java", Role: "Service", Changed: true, Workspace: "current"}},
	}})
	manifest, err := BuildDispatch(units, rules, catalogSHA)
	if err != nil {
		t.Fatalf("build dispatch: %v", err)
	}
	got := ruleIDs160(manifest, "RU-SERVICE")
	want := []string{"SPRING-TX-001", "SPRING-TX-002", "SPRING-TX-003"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Service dispatch mismatch: got %v want %v", got, want)
	}
}

func TestDispatchYamlGetsConfigRule(t *testing.T) {
	rules, catalogSHA := springRules160(t)
	units := sealedUnits160(t, []reviewunit.Unit{{
		ID: "RU-CONFIG",
		Files: []reviewunit.FileRef{{Path: "src/main/resources/application.yml", Role: "YamlConfig", Changed: true, Workspace: "current"}},
		ChangedHunks: []reviewunit.HunkRef{{Path: "src/main/resources/application.yml", NewStart: 10, NewLines: 2}},
	}})
	manifest, err := BuildDispatch(units, rules, catalogSHA)
	if err != nil {
		t.Fatalf("build dispatch: %v", err)
	}
	got := ruleIDs160(manifest, "RU-CONFIG")
	want := []string{"SPRING-CONFIG-001"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("YamlConfig dispatch mismatch: got %v want %v", got, want)
	}
}

func TestYamlConfigReviewUnitDispatchesSpringConfigRuleContract(t *testing.T) {
	rules, catalogSHA := springRules160(t)
	units := sealedUnits160(t, []reviewunit.Unit{{
		ID: "RU-YAML-CONTRACT",
		Files: []reviewunit.FileRef{{Path: "src/main/resources/application-prod.yml", Role: "YamlConfig", Changed: true, Workspace: "current"}},
		ChangedHunks: []reviewunit.HunkRef{{Path: "src/main/resources/application-prod.yml", NewStart: 3, NewLines: 4}},
	}})
	manifest, err := BuildDispatch(units, rules, catalogSHA)
	if err != nil {
		t.Fatalf("build dispatch: %v", err)
	}
	if got, want := ruleIDs160(manifest, "RU-YAML-CONTRACT"), []string{"SPRING-CONFIG-001"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("YamlConfig ReviewUnit must dispatch SPRING-CONFIG-001: got %v want %v", got, want)
	}
	encoded, err := CanonicalBytes(manifest)
	if err != nil {
		t.Fatalf("canonicalize rule dispatch: %v", err)
	}
	contract, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "rule-dispatch.schema.json"))
	if err != nil {
		t.Fatalf("read rule-dispatch contract: %v", err)
	}
	if err := schema.ValidateJSON(contract, encoded); err != nil {
		t.Fatalf("YamlConfig RuleDispatch must satisfy runtime contract: %v", err)
	}
}

func TestDispatchDoesNotUseClassNameSuffixAsRoleFact(t *testing.T) {
	rules, catalogSHA := springRules160(t)
	units := sealedUnits160(t, []reviewunit.Unit{{
		ID: "RU-NAME-ONLY",
		Chain: []string{"FakeController.call", "LooksLikeServiceImpl.execute", "LooksLikeMapper.select"},
		Files: []reviewunit.FileRef{
			{Path: "src/main/java/LooksLikeServiceImpl.java", Role: "Code", Changed: true, Workspace: "current"},
			{Path: "src/main/java/LooksLikeMapper.java", Role: "Code", Changed: true, Workspace: "current"},
		},
	}})
	manifest, err := BuildDispatch(units, rules, catalogSHA)
	if err != nil {
		t.Fatalf("build dispatch: %v", err)
	}
	if got := ruleIDs160(manifest, "RU-NAME-ONLY"); len(got) != 0 {
		t.Fatalf("class/symbol suffixes must not act as role evidence, got %v", got)
	}
}

func TestDispatchStableAcrossUnitOrder(t *testing.T) {
	rules, catalogSHA := springRules160(t)
	mapper := reviewunit.Unit{ID: "RU-B", Files: []reviewunit.FileRef{{Path: "src/main/resources/BMapper.xml", Role: "MapperXml", Changed: true, Workspace: "current"}}}
	service := reviewunit.Unit{ID: "RU-A", Files: []reviewunit.FileRef{{Path: "src/main/java/AService.java", Role: "Service", Changed: true, Workspace: "current"}}}
	left, err := BuildDispatch(sealedUnits160(t, []reviewunit.Unit{mapper, service}), rules, catalogSHA)
	if err != nil { t.Fatal(err) }
	right, err := BuildDispatch(sealedUnits160(t, []reviewunit.Unit{service, mapper}), rules, catalogSHA)
	if err != nil { t.Fatal(err) }
	leftBytes, err := json.MarshalIndent(left, "", "  ")
	if err != nil { t.Fatal(err) }
	rightBytes, err := json.MarshalIndent(right, "", "  ")
	if err != nil { t.Fatal(err) }
	if string(leftBytes) != string(rightBytes) {
		t.Fatalf("dispatch must be byte-stable across ReviewUnit ordering:\nLEFT=%s\nRIGHT=%s", leftBytes, rightBytes)
	}
}

func TestDispatchRejectsStaleReviewUnits(t *testing.T) {
	rules, catalogSHA := springRules160(t)
	units := sealedUnits160(t, []reviewunit.Unit{{ID: "RU-STALE", Files: []reviewunit.FileRef{{Path: "src/main/java/OrderService.java", Role: "Service", Changed: true, Workspace: "current"}}}})
	units.Units[0].Files[0].Role = "Controller"
	if _, err := BuildDispatch(units, rules, catalogSHA); err == nil || !strings.Contains(err.Error(), "RULE_DISPATCH_STALE") {
		t.Fatalf("tampered/stale ReviewUnit manifest must fail closed, got %v", err)
	}
}

func springRules160(t *testing.T) ([]Rule, string) {
	t.Helper()
	rules, sha, err := LoadCatalog(filepath.Join("..", "..", "..", "review-rules", "spring-v1.yaml"))
	if err != nil { t.Fatalf("load spring rules: %v", err) }
	return rules, sha
}

func sealedUnits160(t *testing.T, units []reviewunit.Unit) reviewunit.Manifest {
	t.Helper()
	m := reviewunit.Manifest{
		RunID: "run-rule-dispatch",
		HarnessVersion: "1.6.0",
		Mode: reviewunit.ModeFull,
		ChangeSetSHA256: strings.Repeat("a", 64),
		ChangeAnalysisSHA256: strings.Repeat("b", 64),
		Units: units,
	}
	unsigned, err := reviewunit.CanonicalBytes(m)
	if err != nil { t.Fatal(err) }
	m.SHA256 = fmt.Sprintf("%x", sha256.Sum256(unsigned))
	return m
}

func ruleIDs160(m Manifest, unitID string) []string {
	out := []string{}
	for _, dispatch := range m.Dispatches {
		if dispatch.ReviewUnitID == unitID {
			out = append(out, dispatch.RuleID)
		}
	}
	return out
}
