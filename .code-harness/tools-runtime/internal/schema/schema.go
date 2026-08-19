package schema

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type valueInfo struct {
	kind  string
	raw   string
	count int
}
type schemaNode struct {
	Ref                  string                 `json:"$ref"`
	Type                 any                    `json:"type"`
	Required             []string               `json:"required"`
	Properties           map[string]*schemaNode `json:"properties"`
	Defs                 map[string]*schemaNode `json:"$defs"`
	AdditionalProperties *bool                  `json:"additionalProperties"`
	Enum                 []any                  `json:"enum"`
	Const                any                    `json:"const"`
	MinLength            *int                   `json:"minLength"`
	MinItems             *int                   `json:"minItems"`
	MaxItems             *int                   `json:"maxItems"`
	Minimum              *float64               `json:"minimum"`
	Items                *schemaNode            `json:"items"`
	UniqueItems          bool                   `json:"uniqueItems"`
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
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
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
	if _, e := strconv.Atoi(v); e == nil {
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
func parseSchema(b []byte) (*schemaNode, error) {
	var root schemaNode
	if err := json.Unmarshal(b, &root); err != nil {
		return nil, fmt.Errorf("invalid schema: %w", err)
	}
	return &root, nil
}

func ValidateYAML(schemaBytes, yamlBytes []byte) error {
	root, err := parseSchema(schemaBytes)
	if err != nil {
		return err
	}
	return validateYAMLNode(root, root, "", parseYAMLShape(yamlBytes))
}
func validateYAMLNode(root, s *schemaNode, path string, shape map[string]valueInfo) error {
	s, err := resolveRef(root, s)
	if err != nil {
		return err
	}
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
		actual := scalarValue(info)
		if !enumContains(s.Enum, actual) {
			return fmt.Errorf("%s: enum", path)
		}
	}
	if s.Const != nil && path != "" && !reflect.DeepEqual(normalizeNumber(s.Const), normalizeNumber(scalarValue(info))) {
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
			if err := validateYAMLNode(root, child, p, shape); err != nil {
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
		if a.If != nil && validateYAMLNode(root, a.If, path, shape) == nil && a.Then != nil {
			if err := validateYAMLNode(root, a.Then, path, shape); err != nil {
				return err
			}
		}
	}
	return nil
}

func ValidateJSON(schemaBytes, jsonBytes []byte) error {
	root, err := parseSchema(schemaBytes)
	if err != nil {
		return err
	}
	var v any
	dec := json.NewDecoder(strings.NewReader(string(jsonBytes)))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return validateJSONNode(root, root, "$", v)
}
func validateJSONNode(root, s *schemaNode, path string, v any) error {
	var err error
	s, err = resolveRef(root, s)
	if err != nil {
		return err
	}
	kind := jsonKind(v)
	if !typeMatches(s.Type, kind) {
		return fmt.Errorf("%s: expected %v, got %s", path, s.Type, kind)
	}
	if s.MinLength != nil && kind == "string" && len(v.(string)) < *s.MinLength {
		return fmt.Errorf("%s: minLength", path)
	}
	if len(s.Enum) > 0 && !enumContains(s.Enum, v) {
		return fmt.Errorf("%s: enum", path)
	}
	if s.Const != nil && !reflect.DeepEqual(normalizeNumber(s.Const), normalizeNumber(v)) {
		return fmt.Errorf("%s: const", path)
	}
	switch x := v.(type) {
	case map[string]any:
		for _, req := range s.Required {
			if _, ok := x[req]; !ok {
				return fmt.Errorf("%s: missing required field %s", path, req)
			}
		}
		for name, val := range x {
			if child, ok := s.Properties[name]; ok {
				if err := validateJSONNode(root, child, path+"."+name, val); err != nil {
					return err
				}
			} else if s.AdditionalProperties != nil && !*s.AdditionalProperties {
				return fmt.Errorf("%s: additional property %s", path, name)
			}
		}
	case []any:
		if s.MinItems != nil && len(x) < *s.MinItems {
			return fmt.Errorf("%s: minItems", path)
		}
		if s.MaxItems != nil && len(x) > *s.MaxItems {
			return fmt.Errorf("%s: maxItems", path)
		}
		if s.UniqueItems {
			for i := range x {
				for j := 0; j < i; j++ {
					if reflect.DeepEqual(normalizeNumber(x[i]), normalizeNumber(x[j])) {
						return fmt.Errorf("%s: uniqueItems", path)
					}
				}
			}
		}
		if s.Items != nil {
			for i, val := range x {
				if err := validateJSONNode(root, s.Items, fmt.Sprintf("%s[%d]", path, i), val); err != nil {
					return err
				}
			}
		}
	case json.Number:
		if s.Minimum != nil {
			n, _ := x.Float64()
			if n < *s.Minimum {
				return fmt.Errorf("%s: minimum", path)
			}
		}
	}
	for _, a := range s.AllOf {
		if a.If != nil && validateJSONNode(root, a.If, path, v) == nil && a.Then != nil {
			if err := validateJSONNode(root, a.Then, path, v); err != nil {
				return err
			}
		}
	}
	return nil
}
func resolveRef(root, s *schemaNode) (*schemaNode, error) {
	if s.Ref == "" {
		return s, nil
	}
	const prefix = "#/$defs/"
	if !strings.HasPrefix(s.Ref, prefix) {
		return nil, fmt.Errorf("unsupported $ref %s", s.Ref)
	}
	name := strings.TrimPrefix(s.Ref, prefix)
	d, ok := root.Defs[name]
	if !ok {
		return nil, fmt.Errorf("unknown $ref %s", s.Ref)
	}
	return d, nil
}
func jsonKind(v any) string {
	switch v.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case bool:
		return "boolean"
	case nil:
		return "null"
	case json.Number:
		n := v.(json.Number)
		if !strings.ContainsAny(n.String(), ".eE") {
			return "integer"
		}
		return "number"
	default:
		return "unknown"
	}
}
func enumContains(xs []any, v any) bool {
	nv := normalizeNumber(v)
	for _, e := range xs {
		if reflect.DeepEqual(normalizeNumber(e), nv) {
			return true
		}
	}
	return false
}
func normalizeNumber(v any) any {
	switch n := v.(type) {
	case json.Number:
		return n.String()
	case float64:
		if n == float64(int64(n)) {
			return strconv.FormatInt(int64(n), 10)
		}
		return strconv.FormatFloat(n, 'g', -1, 64)
	default:
		return v
	}
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
