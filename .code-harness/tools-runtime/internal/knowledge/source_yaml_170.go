package knowledge

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

func (source *Source170) UnmarshalYAML(node *yaml.Node) error {
	if node == nil || node.Kind != yaml.MappingNode {
		return fmt.Errorf("source must be a mapping")
	}

	allowed := map[string]bool{
		"id":       true,
		"root":     true,
		"path":     true,
		"kind":     true,
		"required": true,
	}
	seen := map[string]bool{}
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		if !allowed[key] {
			return fmt.Errorf("field %s not found in type knowledge.Source170", key)
		}
		if seen[key] {
			return fmt.Errorf("mapping key %q already defined", key)
		}
		seen[key] = true
	}
	if !seen["required"] {
		return fmt.Errorf("required must be explicitly set")
	}

	var decoded struct {
		ID       string `yaml:"id"`
		Root     string `yaml:"root"`
		Path     string `yaml:"path"`
		Kind     string `yaml:"kind"`
		Required *bool  `yaml:"required"`
	}
	if err := node.Decode(&decoded); err != nil {
		return err
	}
	if decoded.Required == nil {
		return fmt.Errorf("required must be explicitly set")
	}

	source.ID = decoded.ID
	source.Root = decoded.Root
	source.Path = decoded.Path
	source.Kind = decoded.Kind
	source.Required = *decoded.Required
	return nil
}
