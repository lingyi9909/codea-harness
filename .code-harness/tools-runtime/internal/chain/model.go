package chain

import "gopkg.in/yaml.v3"

type Status string

const (
	StatusDiscovered Status = "DISCOVERED"
	StatusAccepted   Status = "ACCEPTED"
	StatusStale      Status = "STALE"
)

const CurrentWorkspace = "current"

func effectiveWorkspace(value string) string {
	if value == "" {
		return CurrentWorkspace
	}
	return value
}

type EntryPoint struct {
	Workspace string `json:"workspace,omitempty" yaml:"workspace,omitempty"`
	Symbol    string `json:"symbol" yaml:"symbol"`
	Path      string `json:"path" yaml:"path"`
}

func (e *EntryPoint) UnmarshalYAML(value *yaml.Node) error {
	type plain EntryPoint
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	*e = EntryPoint(decoded)
	e.Workspace = effectiveWorkspace(e.Workspace)
	return nil
}

type Node struct {
	Workspace string `json:"workspace,omitempty" yaml:"workspace,omitempty"`
	Symbol    string `json:"symbol" yaml:"symbol"`
	Path      string `json:"path" yaml:"path"`
	Role      string `json:"role" yaml:"role"`
}

func (n *Node) UnmarshalYAML(value *yaml.Node) error {
	type plain Node
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	*n = Node(decoded)
	n.Workspace = effectiveWorkspace(n.Workspace)
	return nil
}

type Resource struct {
	Path   string `json:"path" yaml:"path"`
	Symbol string `json:"symbol,omitempty" yaml:"symbol,omitempty"`
	Role   string `json:"role" yaml:"role"`
}

type Boundary struct {
	Symbol string `json:"symbol" yaml:"symbol"`
	Path   string `json:"path" yaml:"path"`
	Role   string `json:"role" yaml:"role"`
}

type Chain struct {
	Version     int          `json:"version" yaml:"version"`
	ID          string       `json:"id" yaml:"id"`
	Name        string       `json:"name" yaml:"name"`
	Status      Status       `json:"status" yaml:"status"`
	EntryPoints []EntryPoint `json:"entryPoints" yaml:"entryPoints"`
	Nodes       []Node       `json:"nodes" yaml:"nodes"`
	Resources   []Resource   `json:"resources,omitempty" yaml:"resources,omitempty"`
	Boundaries  []Boundary   `json:"boundaries,omitempty" yaml:"boundaries,omitempty"`
	Notes       string       `json:"notes,omitempty" yaml:"notes,omitempty"`
}
