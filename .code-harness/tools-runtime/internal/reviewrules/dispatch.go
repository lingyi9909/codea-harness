package reviewrules

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"codea-harness-tools/internal/knowledge"
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

// BuildDispatch170 extends the immutable technical dispatch with READY business
// checks produced by Runtime-owned knowledge loading. The knowledge document
// version remains metadata; dispatch version is the stable Runtime contract v1.
func BuildDispatch170(units reviewunit.Manifest, rules []Rule, catalogSHA string, km knowledge.Manifest170) (Manifest, error) {
	base, err := BuildDispatch(units, rules, catalogSHA)
	if err != nil {
		return Manifest{}, err
	}
	if strings.TrimSpace(km.RunID) != strings.TrimSpace(units.RunID) {
		return Manifest{}, fmt.Errorf("RULE_DISPATCH_KNOWLEDGE_STALE: runId mismatch")
	}
	knowledgeSHA, err := knowledge.Digest170(km)
	if err != nil {
		return Manifest{}, fmt.Errorf("RULE_DISPATCH_KNOWLEDGE_INVALID: %w", err)
	}
	unitIDs := map[string]bool{}
	for _, unit := range units.Units {
		unitIDs[strings.TrimSpace(unit.ID)] = true
	}
	sources := map[string]knowledge.SourceRecord170{}
	for _, source := range km.Sources {
		id := strings.TrimSpace(source.SourceID)
		if id == "" {
			return Manifest{}, fmt.Errorf("RULE_DISPATCH_KNOWLEDGE_INVALID: empty sourceId")
		}
		if _, exists := sources[id]; exists {
			return Manifest{}, fmt.Errorf("RULE_DISPATCH_KNOWLEDGE_INVALID: duplicate sourceId %s", id)
		}
		sources[id] = source
	}
	seenBusiness := map[string]bool{}
	for _, check := range km.Checks {
		if check.Status != "READY" {
			continue
		}
		unitID := strings.TrimSpace(check.ReviewUnitID)
		sourceID := strings.TrimSpace(check.SourceID)
		ruleID := strings.TrimSpace(check.RuleID)
		ruleKey := "BUSINESS:" + sourceID + ":" + ruleID
		if !unitIDs[unitID] || strings.TrimSpace(check.RuleKey) != ruleKey || sourceID == "" || ruleID == "" {
			return Manifest{}, fmt.Errorf("RULE_DISPATCH_KNOWLEDGE_STALE: invalid READY business check %s", check.RuleKey)
		}
		source, ok := sources[sourceID]
		if !ok || source.Kind != "RULES" || source.Status != "READY" || source.Rule == nil || source.Rule.RuleID != ruleID || source.Rule.Status != "ACTIVE" {
			return Manifest{}, fmt.Errorf("RULE_DISPATCH_KNOWLEDGE_STALE: READY business check has no matching active rule %s", ruleKey)
		}
		key := unitID + "\x00" + ruleKey
		if seenBusiness[key] {
			return Manifest{}, fmt.Errorf("RULE_DISPATCH_KNOWLEDGE_INVALID: duplicate business dispatch %s", ruleKey)
		}
		seenBusiness[key] = true
		base.Dispatches = append(base.Dispatches, Dispatch{
			ReviewUnitID:     unitID,
			RuleID:           ruleKey,
			RuleVersion:      1,
			Kind:             KindAgent,
			SeverityDefault:  "medium",
			RequiredEvidence: []string{"BUSINESS_RULE", "CHANGED_RANGE"},
			DispatchReason:   []string{"BUSINESS_RULE:" + sourceID + ":" + ruleID},
		})
	}
	base.KnowledgeSHA256 = knowledgeSHA
	base.RuleCatalogSHA256 = effectiveCatalogDigest170(catalogSHA, knowledgeSHA)
	base.SHA256 = ""
	return sealManifest160(base)
}

func effectiveCatalogDigest170(catalogSHA, knowledgeSHA string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(catalogSHA) + "\x00" + strings.TrimSpace(knowledgeSHA)))
	return fmt.Sprintf("%x", sum[:])
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
	m.KnowledgeSHA256 = strings.TrimSpace(m.KnowledgeSHA256)
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
