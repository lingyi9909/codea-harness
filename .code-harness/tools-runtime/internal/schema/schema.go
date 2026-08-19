package schema

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type valueInfo struct {
	kind  string
	raw   string
	count int
}

type schemaNode struct {
	Type                 any                    `json:"type"`
	Required             []string               `json:"required"`
	Properties           map[string]*schemaNode `json:"properties"`
	AdditionalProperties *bool                  `json:"additionalProperties"`
	Enum                 []any                  `json:"enum"`
	Const                any                    `json:"const"`
	MinLength            *int                   `json:"minLength"`
	MinItems             *int                   `json:"minItems"`
	MaxItems             *int                   `json:"maxItems"`
	Minimum              *float64               `json:"minimum"`
	AllOf                []*schemaNode          `json:"allOf"`
	If                   *schemaNode            `json:"if"`
	Then                 *schemaNode            `json:"then"`
}

type yamlEntry struct {
	indent int
	path   string
}

func parseYAMLShape(data []byte) map[string]valueInfo {
	out := map[string]valueInfo{"": {kind: "object"}}
	var stack []yamlEntry
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") || trimmed == "-" {
			if len(stack) > 0 {
				p := stack[len(stack)-1].path
				v := out[p]
				v.kind = "array"
				v.count++
				out[p] = v
			}
			continue
		}
		colon := strings.Index(trimmed, ":")
		if colon <= 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:colon])
		val := strings.TrimSpace(trimmed[colon+1:])
		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		parent := ""
		if len(stack) > 0 {
			parent = stack[len(stack)-1].path
		}
		p := key
		if parent != "" {
			p = parent + "." + key
		}
		info := valueInfo{kind: "object", raw: val}
		if val != "" {
			info.kind = inferKind(val)
			if info.kind == "array" {
				info.count = countInlineArray(val)
			}
		}
		out[p] = info
		if val == "" {
			stack = append(stack, yamlEntry{indent: indent, path: p})
		}
	}
	return out
}

func inferKind(v string) string {
	if v == "[]" || (strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]")) {
		return "array"
	}
	if v == "true" || v == "false" {
		return "boolean"
	}
	if v == "null" || v == "~" {
		return "null"
	}
	if _, err := strconv.Atoi(v); err == nil {
		return "integer"
	}
	return "string"
}
func countInlineArray(v string) int {
	s := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(v, "["), "]"))
	if s == "" {
		return 0
	}
	return len(strings.Split(s, ","))
}
func unquote(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
		return v[1 : len(v)-1]
	}
	return v
}

func ValidateYAML(schemaBytes, yamlBytes []byte) error {
	var root schemaNode
	if err := json.Unmarshal(schemaBytes, &root); err != nil {
		return fmt.Errorf("invalid schema: %w", err)
	}
	shape := parseYAMLShape(yamlBytes)
	return validateNode(&root, "", shape)
}

func validateNode(s *schemaNode, path string, shape map[string]valueInfo) error {
	info, exists := shape[path]
	if path != "" && !exists {
		return fmt.Errorf("missing %s", path)
	}
	if path != "" && !typeMatches(s.Type, info.kind) {
		return fmt.Errorf("%s: expected %v, got %s", path, s.Type, info.kind)
	}
	if s.MinLength != nil && info.kind == "string" && len(unquote(info.raw)) < *s.MinLength {
		return fmt.Errorf("%s: minLength", path)
	}
	if s.MinItems != nil && info.kind == "array" && info.count < *s.MinItems {
		return fmt.Errorf("%s: minItems", path)
	}
	if s.MaxItems != nil && info.kind == "array" && info.count > *s.MaxItems {
		return fmt.Errorf("%s: maxItems", path)
	}
	if s.Minimum != nil && info.kind == "integer" {
		n, _ := strconv.ParseFloat(info.raw, 64)
		if n < *s.Minimum {
			return fmt.Errorf("%s: minimum", path)
		}
	}
	if len(s.Enum) > 0 && path != "" {
		ok := false
		actual := scalarValue(info)
		for _, e := range s.Enum {
			if fmt.Sprint(e) == fmt.Sprint(actual) {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("%s: enum", path)
		}
	}
	if s.Const != nil && path != "" && fmt.Sprint(s.Const) != fmt.Sprint(scalarValue(info)) {
		return fmt.Errorf("%s: const", path)
	}
	for _, req := range s.Required {
		p := req
		if path != "" {
			p = path + "." + req
		}
		if _, ok := shape[p]; !ok {
			return fmt.Errorf("missing required field %s", p)
		}
	}
	for name, child := range s.Properties {
		p := name
		if path != "" {
			p = path + "." + name
		}
		if _, ok := shape[p]; ok {
			if err := validateNode(child, p, shape); err != nil {
				return err
			}
		}
	}
	if s.AdditionalProperties != nil && !*s.AdditionalProperties {
		allowed := map[string]bool{}
		for k := range s.Properties {
			allowed[k] = true
		}
		prefix := ""
		if path != "" {
			prefix = path + "."
		}
		for p := range shape {
			if p == "" || p == path || !strings.HasPrefix(p, prefix) {
				continue
			}
			rest := strings.TrimPrefix(p, prefix)
			if strings.Contains(rest, ".") {
				continue
			}
			if !allowed[rest] {
				return fmt.Errorf("%s: additional property %s", path, rest)
			}
		}
	}
	for _, a := range s.AllOf {
		if a.If != nil && conditionMatches(a.If, path, shape) && a.Then != nil {
			if err := validateNode(a.Then, path, shape); err != nil {
				return err
			}
		}
	}
	return nil
}

func conditionMatches(s *schemaNode, path string, shape map[string]valueInfo) bool {
	return validateNode(s, path, shape) == nil
}
func scalarValue(i valueInfo) any {
	switch i.kind {
	case "boolean":
		return i.raw == "true"
	case "integer":
		n, _ := strconv.Atoi(i.raw)
		return n
	case "null":
		return nil
	default:
		return unquote(i.raw)
	}
}
func typeMatches(t any, kind string) bool {
	if t == nil {
		return true
	}
	switch v := t.(type) {
	case string:
		return typeOne(v, kind)
	case []any:
		for _, x := range v {
			if s, ok := x.(string); ok && typeOne(s, kind) {
				return true
			}
		}
	}
	return false
}
func typeOne(t, kind string) bool { return t == kind || (t == "number" && kind == "integer") }
