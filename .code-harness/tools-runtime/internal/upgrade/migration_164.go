package upgrade

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

var (
	configMigration163Source = [3]int{1, 6, 3}
	configMigration164Target = [3]int{1, 6, 4}
)

const configMigration163To164 = "config-1.6.3-to-1.6.4"

// migrateConfigForReleaseEdge applies Runtime-owned release migrations before
// the candidate target schema is consulted. The exact 1.6.3 and 1.6.4 schemas
// are compatible, so this explicit release edge preserves the 1.6.3 bytes and
// leaves the retained config-version migration to handle version=1 later in the
// authoritative migration sequence.
func migrateConfigForReleaseEdge(cfg []byte, from, to [3]int) ([]byte, []string, error) {
	if cmp(from, configMigration163Source) != 0 {
		return append([]byte(nil), cfg...), nil, nil
	}
	if cmp(to, configMigration164Target) != 0 {
		return nil, nil, fmt.Errorf(
			"unsupported config migration edge %d.%d.%d -> %d.%d.%d",
			from[0], from[1], from[2], to[0], to[1], to[2],
		)
	}
	migrated, err := migrateConfig163To164(cfg)
	if err != nil {
		return nil, nil, err
	}
	return migrated, []string{configMigration163To164}, nil
}

func migrateConfig163To164(cfg []byte) ([]byte, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(cfg, &root); err != nil {
		return nil, fmt.Errorf("decode 1.6.3 harness.yaml: %w", err)
	}
	if len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("1.6.3 harness.yaml must be a top-level mapping")
	}

	mapping := root.Content[0]
	versionCount := 0
	configVersion := ""
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key := mapping.Content[i]
		value := mapping.Content[i+1]
		if key.Kind != yaml.ScalarNode || key.Value != "version" {
			continue
		}
		versionCount++
		if value.Kind != yaml.ScalarNode || value.Tag != "!!int" {
			return nil, fmt.Errorf("1.6.3 harness config version must be integer 1 or 2")
		}
		configVersion = value.Value
	}
	if versionCount != 1 {
		return nil, fmt.Errorf("1.6.3 harness config requires exactly one top-level version")
	}
	if configVersion != "1" && configVersion != "2" {
		return nil, fmt.Errorf("1.6.3 harness config version must be integer 1 or 2")
	}

	return append([]byte(nil), cfg...), nil
}
