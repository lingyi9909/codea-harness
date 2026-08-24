package chain

type Status string

const (
	StatusDiscovered Status = "DISCOVERED"
	StatusAccepted   Status = "ACCEPTED"
	StatusStale      Status = "STALE"
)

type EntryPoint struct {
	Symbol string `json:"symbol" yaml:"symbol"`
	Path   string `json:"path" yaml:"path"`
}

type Node struct {
	Symbol string `json:"symbol" yaml:"symbol"`
	Path   string `json:"path" yaml:"path"`
	Role   string `json:"role" yaml:"role"`
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
