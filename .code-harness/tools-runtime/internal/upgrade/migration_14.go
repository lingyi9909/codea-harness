package upgrade

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var configV1VersionLineRE = regexp.MustCompile(`^version:([ \t]*)1([ \t]*(?:#.*)?)$`)

func migrateConfigV1ToV2ResourceScopes(cfg []byte) ([]byte, bool, error) {
	var meta struct {
		Version int            `yaml:"version"`
		Scope   map[string]any `yaml:"scope"`
	}
	if err := yaml.Unmarshal(cfg, &meta); err != nil {
		return nil, false, fmt.Errorf("decode harness.yaml for 1.4 migration: %w", err)
	}
	if meta.Version >= 2 {
		return append([]byte(nil), cfg...), false, nil
	}
	if meta.Version != 1 {
		return nil, false, fmt.Errorf("unsupported harness config version %d for 1.4 migration", meta.Version)
	}
	if meta.Scope == nil {
		return nil, false, fmt.Errorf("scope is required for 1.4 migration")
	}
	_, hasMapper := meta.Scope["mapperIncludes"]
	_, hasConfig := meta.Scope["configIncludes"]

	text := string(cfg)
	useCRLF := strings.Contains(text, "\r\n")
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")

	versionIndex := -1
	for i, line := range lines {
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && configV1VersionLineRE.MatchString(line) {
			versionIndex = i
			break
		}
	}
	if versionIndex < 0 {
		return nil, false, fmt.Errorf("cannot locate top-level version: 1 for 1.4 migration")
	}
	lines[versionIndex] = configV1VersionLineRE.ReplaceAllString(lines[versionIndex], `version:${1}2${2}`)

	if !hasMapper || !hasConfig {
		scopeIndex := -1
		for i, line := range lines {
			if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
				trimmed := strings.TrimSpace(line)
				if trimmed == "scope:" || strings.HasPrefix(trimmed, "scope: #") {
					scopeIndex = i
					break
				}
			}
		}
		if scopeIndex < 0 {
			return nil, false, fmt.Errorf("1.4 migration requires block-style top-level scope mapping")
		}

		end := len(lines)
		indent := "  "
		for i := scopeIndex + 1; i < len(lines); i++ {
			line := lines[i]
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			if line[0] != ' ' && line[0] != '\t' {
				end = i
				break
			}
			if keyPos := strings.Index(line, "sourceIncludes:"); keyPos > 0 && strings.TrimSpace(line[keyPos:]) == "sourceIncludes:" {
				indent = line[:keyPos]
			}
		}

		var inserted []string
		if !hasMapper {
			inserted = append(inserted,
				indent+"mapperIncludes:",
				indent+"  - src/main/resources/**/*Mapper.xml",
			)
		}
		if !hasConfig {
			inserted = append(inserted,
				indent+"configIncludes:",
				indent+"  - src/main/resources/**/*.yml",
			)
		}
		lines = append(lines[:end], append(inserted, lines[end:]...)...)
	}

	out := strings.Join(lines, "\n")
	if useCRLF {
		out = strings.ReplaceAll(out, "\n", "\r\n")
	}
	return []byte(out), true, nil
}
