package reviewrules

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type catalog160 struct {
	Version int    `yaml:"version" json:"version"`
	Rules   []Rule `yaml:"rules" json:"rules"`
}

func LoadCatalog(path string) ([]Rule, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("RULE_CATALOG_READ_FAILED: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	var catalog catalog160
	if err := decoder.Decode(&catalog); err != nil {
		return nil, "", fmt.Errorf("RULE_CATALOG_INVALID: decode: %w", err)
	}
	if catalog.Version != 1 {
		return nil, "", fmt.Errorf("RULE_CATALOG_INVALID: version=%d", catalog.Version)
	}
	normalized, err := normalizeRules160(catalog.Rules)
	if err != nil {
		return nil, "", err
	}
	sha, err := catalogDigest160(normalized)
	if err != nil {
		return nil, "", err
	}
	return normalized, sha, nil
}

func normalizeRules160(in []Rule) ([]Rule, error) {
	if len(in) == 0 {
		return nil, fmt.Errorf("RULE_CATALOG_INVALID: rules are empty")
	}
	out := make([]Rule, 0, len(in))
	seen := map[string]bool{}
	for _, raw := range in {
		rule := raw
		rule.ID = strings.TrimSpace(rule.ID)
		if rule.ID == "" {
			return nil, fmt.Errorf("RULE_CATALOG_INVALID: empty rule id")
		}
		if seen[rule.ID] {
			return nil, fmt.Errorf("RULE_CATALOG_DUPLICATE_ID: %s", rule.ID)
		}
		seen[rule.ID] = true
		if rule.Version <= 0 {
			return nil, fmt.Errorf("RULE_CATALOG_INVALID: rule %s version=%d", rule.ID, rule.Version)
		}
		if rule.Kind != KindAgent && rule.Kind != KindMachine {
			return nil, fmt.Errorf("RULE_CATALOG_INVALID: rule %s kind=%q", rule.ID, rule.Kind)
		}
		rule.SeverityDefault = strings.ToLower(strings.TrimSpace(rule.SeverityDefault))
		switch rule.SeverityDefault {
		case "low", "medium", "high", "critical":
		default:
			return nil, fmt.Errorf("RULE_CATALOG_INVALID: rule %s severityDefault=%q", rule.ID, rule.SeverityDefault)
		}
		rule.Roles = uniqueSorted160(rule.Roles)
		if len(rule.Roles) == 0 {
			return nil, fmt.Errorf("RULE_CATALOG_INVALID: rule %s roles are empty", rule.ID)
		}
		rule.RequiredEvidence = uniqueSorted160(rule.RequiredEvidence)
		if len(rule.RequiredEvidence) == 0 {
			return nil, fmt.Errorf("RULE_CATALOG_INVALID: rule %s requiredEvidence is empty", rule.ID)
		}
		rule.Prompt = strings.TrimSpace(rule.Prompt)
		if rule.Prompt == "" {
			return nil, fmt.Errorf("RULE_CATALOG_INVALID: rule %s prompt is empty", rule.ID)
		}
		out = append(out, rule)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].Version < out[j].Version
	})
	return out, nil
}

func catalogDigest160(rules []Rule) (string, error) {
	normalized, err := normalizeRules160(rules)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(catalog160{Version: 1, Rules: normalized})
	if err != nil {
		return "", fmt.Errorf("RULE_CATALOG_INVALID: canonicalize: %w", err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

func uniqueSorted160(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, raw := range in {
		value := strings.TrimSpace(raw)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
