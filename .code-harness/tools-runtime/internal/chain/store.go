package chain

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

const editingHeader = `# Codea Harness Business Chain
#
# 这是项目长期业务 Chain，可直接编辑。
# 修改后请执行：harness chain validate <id>
# 代码结构变化后请执行：harness chain refresh <id>
# symbol/path/call relation 必须真实存在，Runtime 会重新校验。
# 本文件属于 Project State，Harness 升级不会覆盖。
`

func ValidateID(id string) error {
	if !idPattern.MatchString(id) {
		return fmt.Errorf("invalid chain id %q: must match ^[a-z0-9][a-z0-9-]{0,63}$", id)
	}
	return nil
}

func ChainPath(root, id string) (string, error) {
	if err := ValidateID(id); err != nil {
		return "", err
	}
	base := filepath.Join(filepath.Clean(root), ".code-harness", "chains")
	candidate := filepath.Join(base, id+".yaml")
	rel, err := filepath.Rel(base, candidate)
	if err != nil {
		return "", fmt.Errorf("resolve chain path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("chain path escapes project state directory")
	}
	return candidate, nil
}

func Load(path string) (Chain, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Chain{}, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	var c Chain
	if err := dec.Decode(&c); err != nil {
		return Chain{}, fmt.Errorf("decode chain YAML: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return Chain{}, fmt.Errorf("chain YAML must contain exactly one document")
		}
		return Chain{}, fmt.Errorf("decode trailing chain YAML: %w", err)
	}
	if err := validateModel(c); err != nil {
		return Chain{}, err
	}
	return c, nil
}

func MarshalYAML(c Chain) ([]byte, error) {
	if err := validateModel(c); err != nil {
		return nil, err
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("marshal chain YAML: %w", err)
	}
	out := make([]byte, 0, len(editingHeader)+1+len(b))
	out = append(out, editingHeader...)
	out = append(out, '\n')
	out = append(out, b...)
	return out, nil
}

func validateModel(c Chain) error {
	if c.Version != 1 {
		return fmt.Errorf("chain version must be 1")
	}
	if err := ValidateID(c.ID); err != nil {
		return err
	}
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("chain name must not be empty")
	}
	switch c.Status {
	case StatusDiscovered, StatusAccepted, StatusStale:
	default:
		return fmt.Errorf("invalid chain status %q", c.Status)
	}
	if len(c.EntryPoints) == 0 {
		return fmt.Errorf("chain must contain at least one entryPoint")
	}
	for i, entry := range c.EntryPoints {
		if strings.TrimSpace(entry.Symbol) == "" || strings.TrimSpace(entry.Path) == "" {
			return fmt.Errorf("entryPoints[%d] symbol/path must not be empty", i)
		}
	}
	for i, node := range c.Nodes {
		if strings.TrimSpace(node.Symbol) == "" || strings.TrimSpace(node.Path) == "" {
			return fmt.Errorf("nodes[%d] symbol/path must not be empty", i)
		}
		switch node.Role {
		case "SERVICE", "REPOSITORY", "MAPPER", "OTHER":
		default:
			return fmt.Errorf("invalid nodes[%d].role %q", i, node.Role)
		}
	}
	for i, resource := range c.Resources {
		if strings.TrimSpace(resource.Path) == "" {
			return fmt.Errorf("resources[%d].path must not be empty", i)
		}
		switch resource.Role {
		case "MAPPER_XML", "YAML_CONFIG":
		default:
			return fmt.Errorf("invalid resources[%d].role %q", i, resource.Role)
		}
	}
	for i, boundary := range c.Boundaries {
		if strings.TrimSpace(boundary.Symbol) == "" || strings.TrimSpace(boundary.Path) == "" {
			return fmt.Errorf("boundaries[%d] symbol/path must not be empty", i)
		}
		switch boundary.Role {
		case "EXTERNAL", "CACHE", "MQ":
		default:
			return fmt.Errorf("invalid boundaries[%d].role %q", i, boundary.Role)
		}
	}
	return nil
}
