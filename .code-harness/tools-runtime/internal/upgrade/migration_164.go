package upgrade

import (
	"bytes"
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
)

var (
	configMigration163Source = [3]int{1, 6, 3}
	configMigration164Target = [3]int{1, 6, 4}

	legacyEmptyUnresolvedLineRE = regexp.MustCompile(`^([ ]*)unresolved:[ \t]*\[[ \t]*\]([ \t]*(?:#.*)?)$`)
)

const (
	configMigration163To164          = "config-1.6.3-to-1.6.4"
	legacyInitializationUnresolvedID = "projectNotInitialized"
)

// migrateConfigForReleaseEdge applies Runtime-owned release migrations before
// the candidate target schema is consulted. The exact packaged 1.6.3 and 1.6.4
// schemas are byte-equivalent after line-ending normalization, but historical
// Project State can predate the initialization invariant introduced by
// 680c58d. The explicit 1.6.3 -> 1.6.4 edge repairs only that known legacy
// state and otherwise preserves source bytes. The retained config-version
// migration still handles version=1 later in the authoritative sequence.
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
	var initialization *yaml.Node
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key := mapping.Content[i]
		value := mapping.Content[i+1]
		if key.Kind != yaml.ScalarNode {
			continue
		}
		switch key.Value {
		case "version":
			versionCount++
			if value.Kind != yaml.ScalarNode || value.Tag != "!!int" {
				return nil, fmt.Errorf("1.6.3 harness config version must be integer 1 or 2")
			}
			configVersion = value.Value
		case "initialization":
			if initialization != nil {
				return nil, fmt.Errorf("1.6.3 harness config has ambiguous duplicate initialization")
			}
			initialization = value
		}
	}
	if versionCount != 1 {
		return nil, fmt.Errorf("1.6.3 harness config requires exactly one top-level version")
	}
	if configVersion != "1" && configVersion != "2" {
		return nil, fmt.Errorf("1.6.3 harness config version must be integer 1 or 2")
	}

	migrated, repaired, err := repairLegacyInitializationState(cfg, initialization)
	if err != nil {
		return nil, err
	}
	if repaired {
		return migrated, nil
	}
	return append([]byte(nil), cfg...), nil
}

func repairLegacyInitializationState(cfg []byte, initialization *yaml.Node) ([]byte, bool, error) {
	if initialization == nil || initialization.Kind != yaml.MappingNode {
		return append([]byte(nil), cfg...), false, nil
	}

	var status, unresolved *yaml.Node
	statusCount := 0
	unresolvedCount := 0
	for i := 0; i+1 < len(initialization.Content); i += 2 {
		key := initialization.Content[i]
		value := initialization.Content[i+1]
		if key.Kind != yaml.ScalarNode {
			continue
		}
		switch key.Value {
		case "status":
			statusCount++
			status = value
		case "unresolved":
			unresolvedCount++
			unresolved = value
		}
	}
	if statusCount > 1 || unresolvedCount > 1 {
		return nil, false, fmt.Errorf("1.6.3 harness config has ambiguous initialization state")
	}
	if statusCount != 1 || unresolvedCount != 1 || status.Kind != yaml.ScalarNode || status.Value != "NEEDS_CONFIRMATION" {
		return append([]byte(nil), cfg...), false, nil
	}
	if unresolved.Kind != yaml.SequenceNode || len(unresolved.Content) != 0 {
		return append([]byte(nil), cfg...), false, nil
	}

	// The pre-680c58d template represented NEEDS_CONFIRMATION as unresolved: [].
	// 680c58d introduced projectNotInitialized when the schema began requiring a
	// non-empty unresolved list. Repair exactly that historical state and retain
	// every other byte, including user-authored ordering/comments/values.
	return replaceLegacyEmptyUnresolvedLine(cfg, unresolved.Line)
}

func replaceLegacyEmptyUnresolvedLine(cfg []byte, lineNumber int) ([]byte, bool, error) {
	if lineNumber <= 0 {
		return nil, false, fmt.Errorf("historical initialization migration cannot locate unresolved line")
	}
	lines := bytes.SplitAfter(cfg, []byte("\n"))
	if lineNumber > len(lines) {
		return nil, false, fmt.Errorf("historical initialization migration unresolved line out of range")
	}

	index := lineNumber - 1
	line := lines[index]
	ending := []byte{}
	content := line
	switch {
	case bytes.HasSuffix(line, []byte("\r\n")):
		ending = []byte("\r\n")
		content = line[:len(line)-2]
	case bytes.HasSuffix(line, []byte("\n")):
		ending = []byte("\n")
		content = line[:len(line)-1]
	}

	matches := legacyEmptyUnresolvedLineRE.FindSubmatch(content)
	if matches == nil {
		return nil, false, fmt.Errorf("historical initialization state is semantically empty but not in the supported deterministic unresolved: [] form")
	}
	indent := matches[1]
	suffix := matches[2]
	separator := ending
	if len(separator) == 0 {
		if bytes.Contains(cfg, []byte("\r\n")) {
			separator = []byte("\r\n")
		} else {
			separator = []byte("\n")
		}
	}

	replacement := make([]byte, 0, len(line)+len(indent)+len(legacyInitializationUnresolvedID)+8)
	replacement = append(replacement, indent...)
	replacement = append(replacement, []byte("unresolved:")...)
	replacement = append(replacement, suffix...)
	replacement = append(replacement, separator...)
	replacement = append(replacement, indent...)
	replacement = append(replacement, []byte("  - "+legacyInitializationUnresolvedID)...)
	replacement = append(replacement, ending...)

	out := make([]byte, 0, len(cfg)+len(replacement)-len(line))
	for i, current := range lines {
		if i == index {
			out = append(out, replacement...)
			continue
		}
		out = append(out, current...)
	}
	return out, true, nil
}
