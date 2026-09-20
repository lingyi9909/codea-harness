package reviewrules

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"codea-harness-tools/internal/reviewunit"
)

const TestValidityRuleID = "TEST-VALIDITY-001"

var testValidityRule160 = Rule{
	ID:               TestValidityRuleID,
	Version:          1,
	Kind:             KindAgent,
	SeverityDefault:  "high",
	Roles:            []string{"Test"},
	RequiredEvidence: []string{"CHANGED_RANGE"},
	Prompt:           "只检查本次测试变更是否使关键断言、失败路径或有效性校验失效；必须引用 changed test range 证明 Test Validity 问题由本次变更引入，不审普通测试风格。",
}

func BuildDispatch(units reviewunit.Manifest, rules []Rule, catalogSHA string) (Manifest, error) {
	if err := verifyReviewUnits160(units); err != nil {
		return Manifest{}, err
	}
	normalizedRules, err := normalizeRules160(rules)
	if err != nil {
		return Manifest{}, err
	}
	actualCatalogSHA, err := catalogDigest160(normalizedRules)
	if err != nil {
		return Manifest{}, err
	}
	if strings.TrimSpace(catalogSHA) == "" || catalogSHA != actualCatalogSHA {
		return Manifest{}, fmt.Errorf("RULE_DISPATCH_CATALOG_STALE: catalog sha256 mismatch")
	}

	orderedUnits := append([]reviewunit.Unit(nil), units.Units...)
	sort.Slice(orderedUnits, func(i, j int) bool { return orderedUnits[i].ID < orderedUnits[j].ID })
	seenUnits := map[string]bool{}
	dispatches := []Dispatch{}
	for _, unit := range orderedUnits {
		unitID := strings.TrimSpace(unit.ID)
		if unitID == "" {
			return Manifest{}, fmt.Errorf("RULE_DISPATCH_INVALID: empty ReviewUnit id")
		}
		if seenUnits[unitID] {
			return Manifest{}, fmt.Errorf("RULE_DISPATCH_INVALID: duplicate ReviewUnit id %s", unitID)
		}
		seenUnits[unitID] = true
		changedRoles, err := changedCurrentRoles160(unit)
		if err != nil {
			return Manifest{}, err
		}
		for _, rule := range normalizedRules {
			reasons := matchReasons160(rule, changedRoles)
			if len(reasons) == 0 {
				continue
			}
			dispatches = append(dispatches, Dispatch{
				ReviewUnitID:     unitID,
				RuleID:           rule.ID,
				RuleVersion:      rule.Version,
				Kind:             rule.Kind,
				SeverityDefault:  rule.SeverityDefault,
				RequiredEvidence: append([]string(nil), rule.RequiredEvidence...),
				DispatchReason:   reasons,
			})
		}
		// Test Validity is Runtime-owned authority, deliberately outside the
		// locked Spring Rule Pack catalog. Test scope is derived from the path,
		// and src/test/** <-> Test is machine-enforced below; Agent role claims
		// cannot create or suppress this dispatch authority.
		if changedRoles["Test"] {
			dispatches = append(dispatches, Dispatch{
				ReviewUnitID:     unitID,
				RuleID:           testValidityRule160.ID,
				RuleVersion:      testValidityRule160.Version,
				Kind:             testValidityRule160.Kind,
				SeverityDefault:  testValidityRule160.SeverityDefault,
				RequiredEvidence: append([]string(nil), testValidityRule160.RequiredEvidence...),
				DispatchReason:   []string{"CHANGED_ROLE:Test"},
			})
		}
	}

	manifest := Manifest{
		RunID:             units.RunID,
		ReviewUnitsSHA256: units.SHA256,
		RuleCatalogSHA256: actualCatalogSHA,
		Dispatches:        dispatches,
	}
	return sealManifest160(manifest)
}

func verifyReviewUnits160(units reviewunit.Manifest) error {
	want := strings.TrimSpace(units.SHA256)
	if want == "" || strings.TrimSpace(units.RunID) == "" {
		return fmt.Errorf("RULE_DISPATCH_STALE: ReviewUnit identity is incomplete")
	}
	candidate := units
	candidate.SHA256 = ""
	canonical, err := reviewunit.CanonicalBytes(candidate)
	if err != nil {
		return fmt.Errorf("RULE_DISPATCH_STALE: canonicalize ReviewUnit: %w", err)
	}
	got := fmt.Sprintf("%x", sha256.Sum256(canonical))
	if got != want {
		return fmt.Errorf("RULE_DISPATCH_STALE: ReviewUnit sha256 mismatch")
	}
	return nil
}

func changedCurrentRoles160(unit reviewunit.Unit) (map[string]bool, error) {
	roles := map[string]bool{}
	for _, file := range unit.Files {
		if !file.Changed || strings.TrimSpace(file.Workspace) != "current" {
			continue
		}
		p := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(file.Path), "\\", "/"))
		role := strings.TrimSpace(file.Role)
		isTestPath := strings.HasPrefix(p, "src/test/") || strings.Contains(p, "/src/test/")
		if isTestPath && role != "Test" {
			return nil, fmt.Errorf("RULE_DISPATCH_PATH_ROLE_INVALID: src/test path %s requires role Test", file.Path)
		}
		if role == "Test" && !isTestPath {
			return nil, fmt.Errorf("RULE_DISPATCH_PATH_ROLE_INVALID: role Test requires src/test path, got %s", file.Path)
		}
		if isTestPath {
			roles["Test"] = true
			continue
		}
		if role != "" {
			roles[role] = true
		}
	}
	return roles, nil
}

func matchReasons160(rule Rule, changedRoles map[string]bool) []string {
	reasons := []string{}
	for _, role := range rule.Roles {
		if changedRoles[role] {
			reasons = append(reasons, "CHANGED_ROLE:"+role)
		}
	}
	return uniqueSorted160(reasons)
}

func sealManifest160(m Manifest) (Manifest, error) {
	m = normalizeManifest160(m)
	m.SHA256 = ""
	data, err := canonicalManifestBytes160(m)
	if err != nil {
		return Manifest{}, err
	}
	m.SHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
	return normalizeManifest160(m), nil
}

func CanonicalBytes(m Manifest) ([]byte, error) {
	return canonicalManifestBytes160(normalizeManifest160(m))
}

func canonicalManifestBytes160(m Manifest) ([]byte, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("RULE_DISPATCH_ENCODE_FAILED: %w", err)
	}
	return append(data, '\n'), nil
}

func normalizeManifest160(m Manifest) Manifest {
	m.RunID = strings.TrimSpace(m.RunID)
	m.ReviewUnitsSHA256 = strings.TrimSpace(m.ReviewUnitsSHA256)
	m.RuleCatalogSHA256 = strings.TrimSpace(m.RuleCatalogSHA256)
	m.Dispatches = append([]Dispatch(nil), m.Dispatches...)
	for i := range m.Dispatches {
		m.Dispatches[i].ReviewUnitID = strings.TrimSpace(m.Dispatches[i].ReviewUnitID)
		m.Dispatches[i].RuleID = strings.TrimSpace(m.Dispatches[i].RuleID)
		m.Dispatches[i].SeverityDefault = strings.TrimSpace(m.Dispatches[i].SeverityDefault)
		m.Dispatches[i].RequiredEvidence = uniqueSorted160(m.Dispatches[i].RequiredEvidence)
		m.Dispatches[i].DispatchReason = uniqueSorted160(m.Dispatches[i].DispatchReason)
	}
	sort.Slice(m.Dispatches, func(i, j int) bool {
		left, right := m.Dispatches[i], m.Dispatches[j]
		if left.ReviewUnitID != right.ReviewUnitID {
			return left.ReviewUnitID < right.ReviewUnitID
		}
		if left.RuleID != right.RuleID {
			return left.RuleID < right.RuleID
		}
		return left.RuleVersion < right.RuleVersion
	})
	if m.Dispatches == nil {
		m.Dispatches = []Dispatch{}
	}
	return m
}