package apply

import (
	"errors"
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
	if err != nil {
		return Policy{}, fmt.Errorf("read harness write policy: %w", err)
	}
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		return Policy{}, fmt.Errorf("read harness config schema: %w", err)
	}
	if err := schema.ValidateYAML(schemaBytes, configBytes); err != nil {
		return Policy{}, fmt.Errorf("invalid harness write policy config: %w", err)
	}
	var cfg harnessPolicyConfig
	if err := yaml.Unmarshal(configBytes, &cfg); err != nil {
		return Policy{}, fmt.Errorf("decode harness write policy: %w", err)
	}
	return Policy{
		AllowedTest:       normalizePatterns(cfg.Write.AllowedTestPaths),
		AllowedProduction: normalizePatterns(cfg.Write.AllowedProductionPaths),
		Denied:            normalizePatterns(cfg.Write.DeniedPaths),
	}, nil
}

func (p Policy) Allow(planType, rawPath string) error {
	clean, err := safeRepoPath(rawPath)
	if err != nil {
		return err
	}
	for _, pattern := range []string{".git", ".git/**", ".code-harness", ".code-harness/**"} {
		if globMatch(pattern, clean) {
			return fmt.Errorf("PATH_HARD_DENIED: PATH_DENIED_BY_RUNTIME_HARD_RULE: %q matches runtime hard-deny %q", clean, pattern)
		}
	}
	for _, pattern := range p.Denied {
		if globMatch(pattern, clean) {
			return fmt.Errorf("PATH_DENIED: %q matches denied path %q", clean, pattern)
		}
	}
	var allowed []string
	switch planType {
	case "FIX":
		allowed = p.AllowedProduction
	case "TEST":
		allowed = p.AllowedTest
	default:
		return fmt.Errorf("invalid planType %q", planType)
	}
	for _, pattern := range allowed {
		if globMatch(pattern, clean) {
			return nil
		}
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

func ensureNoSymlinkEscape(repoRoot, rel string) error {
	clean, err := safeRepoPath(rel)
	if err != nil {
		return err
	}
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return fmt.Errorf("UNSAFE_SYMLINK_PATH: resolve repo root: %w", err)
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return fmt.Errorf("UNSAFE_SYMLINK_PATH: resolve repo root: %w", err)
	}
	current := absRoot
	parts := strings.Split(filepath.FromSlash(clean), string(filepath.Separator))
	for _, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if err != nil {
			return fmt.Errorf("UNSAFE_SYMLINK_PATH: lstat %q: %w", clean, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("UNSAFE_SYMLINK_PATH: %q contains symbolic link component", clean)
		}
		realCurrent, err := filepath.EvalSymlinks(current)
		if err != nil {
			return fmt.Errorf("UNSAFE_SYMLINK_PATH: resolve %q: %w", clean, err)
		}
		relToRoot, err := filepath.Rel(realRoot, realCurrent)
		if err != nil || relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) {
			return fmt.Errorf("UNSAFE_SYMLINK_PATH: %q resolves outside repository", clean)
		}
	}
	return nil
}

func normalizePatterns(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func globMatch(pattern, value string) bool {
	pattern = strings.ReplaceAll(pattern, "\\", "/")
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); {
		if i+1 < len(pattern) && pattern[i] == '*' && pattern[i+1] == '*' {
			b.WriteString(".*")
			i += 2
			continue
		}
		switch pattern[i] {
		case '*':
			b.WriteString("[^/]*")
		case '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
		i++
	}
	b.WriteString("$")
	matched, err := regexp.MatchString(b.String(), value)
	return err == nil && matched
}
