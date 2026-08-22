package schema

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

const schemaResource = "schema.json"

func compile(schemaBytes []byte) (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaBytes))
	if err != nil {
		return nil, fmt.Errorf("invalid schema JSON: %w", err)
	}

	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	if err := compiler.AddResource(schemaResource, doc); err != nil {
		return nil, fmt.Errorf("add schema resource: %w", err)
	}
	compiled, err := compiler.Compile(schemaResource)
	if err != nil {
		return nil, fmt.Errorf("invalid JSON Schema: %w", err)
	}
	return compiled, nil
}

func ValidateJSON(schemaBytes, jsonBytes []byte) error {
	compiled, err := compile(schemaBytes)
	if err != nil {
		return err
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(jsonBytes))
	if err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if err := compiled.Validate(instance); err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}
	title := schemaTitle(schemaBytes)
	switch title {
	case "Diagnosis":
		if err := validateDiagnosisSemantics(jsonBytes); err != nil {
			return fmt.Errorf("diagnosis semantic validation failed: %w", err)
		}
	case "ApplyRequest":
		if err := validateApplyRequestSemantics(jsonBytes); err != nil {
			return fmt.Errorf("apply request semantic validation failed: %w", err)
		}
	case "FixPlan", "IntegrationTestPlan":
		if err := validatePlanPatchIdentity(jsonBytes); err != nil {
			return fmt.Errorf("plan patch identity validation failed: %w", err)
		}
	}
	return nil
}

func schemaTitle(schemaBytes []byte) string {
	var meta struct {
		Title string `json:"title"`
	}
	if json.Unmarshal(schemaBytes, &meta) != nil {
		return ""
	}
	return meta.Title
}

func validateDiagnosisSemantics(jsonBytes []byte) error {
	var diagnosis struct {
		Classification string `json:"classification"`
		NextAction     string `json:"nextAction"`
		CodeEvidence   []struct {
			LineStart int `json:"lineStart"`
			LineEnd   int `json:"lineEnd"`
		} `json:"codeEvidence"`
	}
	if err := json.Unmarshal(jsonBytes, &diagnosis); err != nil {
		return fmt.Errorf("decode diagnosis: %w", err)
	}
	for i, evidence := range diagnosis.CodeEvidence {
		if evidence.LineEnd < evidence.LineStart {
			return fmt.Errorf("codeEvidence[%d].lineEnd must be >= lineStart", i)
		}
	}
	if diagnosis.Classification == "PRODUCTION_CODE_ERROR" {
		if len(diagnosis.CodeEvidence) == 0 {
			return fmt.Errorf("PRODUCTION_CODE_ERROR requires at least one codeEvidence entry")
		}
		if diagnosis.NextAction != "GENERATE_FIX_PLAN" {
			return fmt.Errorf("PRODUCTION_CODE_ERROR requires nextAction GENERATE_FIX_PLAN")
		}
	}
	return nil
}

func validateApplyRequestSemantics(jsonBytes []byte) error {
	var req struct {
		Files []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	if err := json.Unmarshal(jsonBytes, &req); err != nil {
		return fmt.Errorf("decode apply request: %w", err)
	}
	return validateUniqueWindowsPaths(req.Files)
}

func validatePlanPatchIdentity(jsonBytes []byte) error {
	var plan struct {
		UnifiedDiff string `json:"unifiedDiff"`
		DiffSha256  string `json:"diffSha256"`
		Files       []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	if err := json.Unmarshal(jsonBytes, &plan); err != nil {
		return fmt.Errorf("decode plan: %w", err)
	}
	if plan.UnifiedDiff == "" && plan.DiffSha256 == "" && len(plan.Files) == 0 {
		return nil
	}
	actual := fmt.Sprintf("%x", sha256.Sum256([]byte(plan.UnifiedDiff)))
	if !strings.EqualFold(plan.DiffSha256, actual) {
		return fmt.Errorf("diffSha256 mismatch: declared=%s actual=%s", plan.DiffSha256, actual)
	}
	if err := validateUniqueWindowsPaths(plan.Files); err != nil {
		return err
	}
	return nil
}

func validateUniqueWindowsPaths[T interface{ ~struct{ Path string `json:"path"` } }](files []T) error {
	seen := make(map[string]struct{}, len(files))
	for i, raw := range files {
		// Convert through JSON to keep this helper usable for anonymous contract structs.
		b, _ := json.Marshal(raw)
		var file struct{ Path string `json:"path"` }
		_ = json.Unmarshal(b, &file)
		key := strings.ToLower(path.Clean(strings.ReplaceAll(strings.TrimSpace(file.Path), "\\", "/")))
		if _, ok := seen[key]; ok {
			return fmt.Errorf("files[%d].path duplicates Windows-equivalent path %q", i, file.Path)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func ValidateYAML(schemaBytes, yamlBytes []byte) error {
	compiled, err := compile(schemaBytes)
	if err != nil {
		return err
	}

	var raw any
	if err := yaml.Unmarshal(yamlBytes, &raw); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	normalized, err := normalizeYAML(raw)
	if err != nil {
		return err
	}

	jsonBytes, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("convert YAML to JSON: %w", err)
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(jsonBytes))
	if err != nil {
		return fmt.Errorf("convert YAML instance: %w", err)
	}
	if err := compiled.Validate(instance); err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}
	return nil
}

func normalizeYAML(v any) (any, error) {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, value := range x {
			n, err := normalizeYAML(value)
			if err != nil {
				return nil, err
			}
			out[k] = n
		}
		return out, nil
	case map[any]any:
		out := make(map[string]any, len(x))
		for key, value := range x {
			k, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("YAML object key must be a string, got %T", key)
			}
			n, err := normalizeYAML(value)
			if err != nil {
				return nil, err
			}
			out[k] = n
		}
		return out, nil
	case []any:
		out := make([]any, len(x))
		for i, value := range x {
			n, err := normalizeYAML(value)
			if err != nil {
				return nil, err
			}
			out[i] = n
		}
		return out, nil
	default:
		return x, nil
	}
}
