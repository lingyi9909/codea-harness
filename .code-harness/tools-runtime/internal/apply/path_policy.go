package apply

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"codea-harness-tools/internal/schema"
	"gopkg.in/yaml.v3"
)

type Policy struct {
	AllowedTest       []string
	AllowedProduction []string
	Denied            []string
}

type harnessPolicyConfig struct {
	Write struct {
		AllowedTestPaths       []string `yaml:"allowedTestPaths"`
		AllowedProductionPaths []string `yaml:"allowedProductionPaths"`
		DeniedPaths            []string `yaml:"deniedPaths"`
	} `yaml:"write"`
}

func LoadPolicy(repoRoot string) (Policy, error) {
	configPath := filepath.Join(repoRoot, ".code-harness", "harness.yaml")
	schemaPath := filepath.Join(repoRoot, ".code-harness", "contracts", "harness-config.schema.json")
	configBytes, err := os.ReadFile(configPath)
	if err != nil { return Policy{}, fmt.Errorf("read harness write policy: %w", err) }
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil { return Policy{}, fmt.Errorf("read harness config schema: %w", err) }
	if err := schema.ValidateYAML(schemaBytes, configBytes); err != nil {
		return Policy{}, fmt.Errorf("invalid harness write policy config: %w", err)
	}
	var cfg harnessPolicyConfig
	if err := yaml.Unmarshal(configBytes, &cfg); err != nil {
		return Policy{}, fmt.Errorf("decode harness write policy: %w", err)
	}
	return Policy{
		AllowedTest: normalizePatterns(cfg.Write.AllowedTestPaths),
		AllowedProduction: normalizePatterns(cfg.Write.AllowedProductionPaths),
		Denied: normalizePatterns(cfg.Write.DeniedPaths),
	}, nil
}

func (p Policy) Allow(planType, rawPath string) error {
	clean, err := safeRepoPath(rawPath)
	if err != nil { return err }
	for _, pattern := range append(append([]string(nil), p.Denied...), ".code-harness/**") {
		if globMatch(pattern, clean) {
			return fmt.Errorf("PATH_DENIED: %q matches denied path %q", clean, pattern)
		}
	}
	var allowed []string
	switch planType {
	case "FIX": allowed = p.AllowedProduction
	case "TEST": allowed = p.AllowedTest
	default: return fmt.Errorf("invalid planType %q", planType)
	}
	for _, pattern := range allowed {
		if globMatch(pattern, clean) { return nil }
	}
	return fmt.Errorf("PATH_NOT_ALLOWED: %q is outside %s allowlist", clean, planType)
}

func safeRepoPath(raw string) (string, error) {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if raw == "" || strings.HasPrefix(raw, "/") || strings.Contains(raw, ":") {
		return "", fmt.Errorf("UNSAFE_PATH: %q must be repository-relative", raw)
	}
	clean := path.Clean(raw)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("UNSAFE_PATH: %q escapes repository root", raw)
	}
	return clean, nil
}

func normalizePatterns(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
		if value != "" { out = append(out, value) }
	}
	return out
}

func globMatch(pattern, value string) bool {
	pattern = strings.ReplaceAll(pattern, "\\", "/")
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); {
		if i+1 < len(pattern) && pattern[i] == '*' && pattern[i+1] == '*' {
			b.WriteString(".*"); i += 2; continue
		}
		switch pattern[i] {
		case '*': b.WriteString("[^/]*")
		case '?': b.WriteString("[^/]")
		default: b.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
		i++
	}
	b.WriteString("$")
	matched, err := regexp.MatchString(b.String(), value)
	return err == nil && matched
}
