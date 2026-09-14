package knowledge

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

var idPattern170 = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
var entryPointPattern170 = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$.]*#[A-Za-z_$][A-Za-z0-9_$]*\([^)]*\)$`)

func ParseBinding170(raw []byte) (Binding170, string, error) {
	var binding Binding170
	if !utf8.Valid(raw) {
		return binding, "", fmt.Errorf("KNOWLEDGE_BINDING_INVALID: context.yaml is not UTF-8")
	}
	if err := decodeStrictYAML170(raw, &binding); err != nil {
		return binding, "", fmt.Errorf("KNOWLEDGE_BINDING_INVALID: %w", err)
	}
	if err := validateBinding170(binding); err != nil {
		return binding, "", err
	}
	sum := sha256.Sum256(raw)
	return binding, fmt.Sprintf("%x", sum[:]), nil
}

func ParseRule170(raw []byte) (Rule170, string, error) {
	var rule Rule170
	if !utf8.Valid(raw) {
		return rule, "", fmt.Errorf("KNOWLEDGE_RULE_INVALID: rule document is not UTF-8")
	}
	normalized := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return rule, "", fmt.Errorf("KNOWLEDGE_RULE_INVALID: missing YAML frontmatter")
	}
	rest := normalized[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return rule, "", fmt.Errorf("KNOWLEDGE_RULE_INVALID: unterminated YAML frontmatter")
	}
	header := []byte(rest[:end])
	body := rest[end+len("\n---\n"):]
	if strings.HasPrefix(strings.TrimSpace(body), "---") {
		return rule, "", fmt.Errorf("KNOWLEDGE_RULE_INVALID: multiple YAML documents/frontmatter blocks are not allowed")
	}
	if err := decodeStrictYAML170(header, &rule); err != nil {
		return rule, "", fmt.Errorf("KNOWLEDGE_RULE_INVALID: %w", err)
	}
	if err := validateRule170(rule, body); err != nil {
		return rule, "", err
	}
	return rule, body, nil
}

func Applies170(rule Rule170, unit Unit170) bool {
	if rule.Status != "ACTIVE" {
		return false
	}
	for _, selector := range rule.AppliesTo.Paths {
		selector = strings.TrimSpace(strings.ReplaceAll(selector, "\\", "/"))
		if selector == "./" {
			return true
		}
		prefix := strings.HasSuffix(selector, "/")
		cleanSelector := strings.TrimPrefix(path.Clean(selector), "./")
		for _, candidate := range unit.Paths {
			candidate = strings.TrimPrefix(path.Clean(strings.ReplaceAll(strings.TrimSpace(candidate), "\\", "/")), "./")
			if prefix {
				if strings.HasPrefix(candidate, strings.TrimSuffix(cleanSelector, "/")+"/") {
					return true
				}
			} else if candidate == cleanSelector {
				return true
			}
		}
	}
	for _, selector := range rule.AppliesTo.EntryPoints {
		selector = strings.TrimSpace(selector)
		for _, entry := range unit.EntryPoints {
			if strings.TrimSpace(entry) == selector {
				return true
			}
		}
	}
	return false
}

func decodeStrictYAML170(raw []byte, out any) error {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		return err
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple YAML documents are not allowed")
		}
		return err
	}
	return nil
}

func validateBinding170(binding Binding170) error {
	if binding.Version != 1 {
		return fmt.Errorf("KNOWLEDGE_BINDING_INVALID: version must be 1")
	}
	if !idPattern170.MatchString(binding.ProjectID) {
		return fmt.Errorf("KNOWLEDGE_BINDING_INVALID: invalid projectId %q", binding.ProjectID)
	}
	if len(binding.Sources) > 100 {
		return fmt.Errorf("KNOWLEDGE_BINDING_INVALID: sources exceeds 100")
	}
	seen := map[string]bool{}
	usesTeam := false
	for _, source := range binding.Sources {
		if !idPattern170.MatchString(source.ID) {
			return fmt.Errorf("KNOWLEDGE_BINDING_INVALID: invalid source id %q", source.ID)
		}
		if seen[source.ID] {
			return fmt.Errorf("KNOWLEDGE_BINDING_INVALID: duplicate sourceId %q", source.ID)
		}
		seen[source.ID] = true
		switch source.Root {
		case "PROJECT":
		case "TEAM":
			usesTeam = true
		default:
			return fmt.Errorf("KNOWLEDGE_BINDING_INVALID: unknown root %q", source.Root)
		}
		switch source.Kind {
		case "RULES", "REFERENCE", "EXPERIENCE":
		default:
			return fmt.Errorf("KNOWLEDGE_BINDING_INVALID: unknown kind %q", source.Kind)
		}
		if source.Kind == "RULES" && !source.Required {
			return fmt.Errorf("KNOWLEDGE_BINDING_INVALID: RULES source %q must be required", source.ID)
		}
		if err := validateRelativeMarkdownPath170(source.Path); err != nil {
			return fmt.Errorf("KNOWLEDGE_BINDING_INVALID: source %q: %w", source.ID, err)
		}
	}
	if usesTeam {
		root := strings.TrimSpace(binding.TeamRoot)
		lower := strings.ToLower(root)
		if root == "" {
			return fmt.Errorf("KNOWLEDGE_BINDING_INVALID: teamRoot is required for TEAM sources")
		}
		if strings.HasPrefix(root, `\\`) || strings.HasPrefix(root, "//") || strings.Contains(lower, "://") {
			return fmt.Errorf("KNOWLEDGE_BINDING_INVALID: teamRoot must be a local non-UNC path")
		}
	}
	return nil
}

func validateRelativeMarkdownPath170(value string) error {
	if value == "" || value != strings.TrimSpace(value) {
		return fmt.Errorf("path must be a non-empty canonical relative Markdown path")
	}
	if strings.Contains(value, "\\") || strings.Contains(value, ":") || strings.ContainsAny(value, "*?[]{}") {
		return fmt.Errorf("path contains denied syntax")
	}
	if strings.HasPrefix(value, "/") || filepath.VolumeName(value) != "" || strings.Contains(strings.ToLower(value), "://") {
		return fmt.Errorf("absolute/URL path is denied")
	}
	clean := path.Clean(value)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains("/"+clean+"/", "/../") {
		return fmt.Errorf("path traversal is denied")
	}
	if !strings.HasSuffix(value, ".md") {
		return fmt.Errorf("path must end in .md")
	}
	return nil
}

func validateRule170(rule Rule170, body string) error {
	if !idPattern170.MatchString(rule.RuleID) {
		return fmt.Errorf("KNOWLEDGE_RULE_INVALID: invalid ruleId %q", rule.RuleID)
	}
	if rule.ProjectID != "*" && !idPattern170.MatchString(rule.ProjectID) {
		return fmt.Errorf("KNOWLEDGE_RULE_INVALID: invalid projectId %q", rule.ProjectID)
	}
	switch rule.Status {
	case "DRAFT", "ACTIVE", "RETIRED":
	default:
		return fmt.Errorf("KNOWLEDGE_RULE_INVALID: invalid status %q", rule.Status)
	}
	for name, value := range map[string]string{"version": rule.Version, "owner": rule.Owner, "source": rule.Source, "approvalRef": rule.ApprovalRef} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("KNOWLEDGE_RULE_INVALID: %s is required", name)
		}
	}
	if len(rule.AppliesTo.Paths) == 0 && len(rule.AppliesTo.EntryPoints) == 0 {
		return fmt.Errorf("KNOWLEDGE_RULE_INVALID: appliesTo must select a path or exact entry point")
	}
	for _, selector := range rule.AppliesTo.Paths {
		if selector == "./" {
			continue
		}
		if selector == "" || selector != strings.TrimSpace(selector) || strings.Contains(selector, "\\") || strings.Contains(selector, ":") || strings.ContainsAny(selector, "*?[]{}") || strings.HasPrefix(selector, "/") {
			return fmt.Errorf("KNOWLEDGE_RULE_INVALID: invalid path selector %q", selector)
		}
		clean := path.Clean(selector)
		if clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains("/"+clean+"/", "/../") {
			return fmt.Errorf("KNOWLEDGE_RULE_INVALID: path traversal selector denied")
		}
	}
	for _, selector := range rule.AppliesTo.EntryPoints {
		if !entryPointPattern170.MatchString(strings.TrimSpace(selector)) {
			return fmt.Errorf("KNOWLEDGE_RULE_INVALID: entry point must include exact parameter signature: %q", selector)
		}
	}
	seen := map[string]bool{}
	for _, superseded := range rule.Supersedes {
		if !idPattern170.MatchString(superseded) || seen[superseded] {
			return fmt.Errorf("KNOWLEDGE_RULE_INVALID: invalid/duplicate supersedes id %q", superseded)
		}
		seen[superseded] = true
	}
	if rule.Status == "ACTIVE" && strings.TrimSpace(body) == "" {
		return fmt.Errorf("KNOWLEDGE_RULE_INVALID: ACTIVE rule body is empty")
	}
	return nil
}
