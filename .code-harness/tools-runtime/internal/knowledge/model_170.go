package knowledge

type Source170 struct {
	ID       string `json:"id" yaml:"id"`
	Root     string `json:"root" yaml:"root"`
	Path     string `json:"path" yaml:"path"`
	Kind     string `json:"kind" yaml:"kind"`
	Required bool   `json:"required" yaml:"required"`
}

type Binding170 struct {
	Version   int         `json:"version" yaml:"version"`
	ProjectID string      `json:"projectId" yaml:"projectId"`
	TeamRoot  string      `json:"teamRoot,omitempty" yaml:"teamRoot,omitempty"`
	Sources   []Source170 `json:"sources" yaml:"sources"`
}

type Unit170 struct {
	ID          string
	Paths       []string
	EntryPoints []string // ownerFqcn#method(parameterTypes)
}

type AppliesTo170 struct {
	Paths       []string `json:"paths" yaml:"paths"`
	EntryPoints []string `json:"entryPoints" yaml:"entryPoints"`
}

type Rule170 struct {
	RuleID      string       `json:"ruleId" yaml:"ruleId"`
	ProjectID   string       `json:"projectId" yaml:"projectId"`
	Status      string       `json:"status" yaml:"status"`
	Version     string       `json:"version" yaml:"version"`
	Owner       string       `json:"owner" yaml:"owner"`
	Source      string       `json:"source" yaml:"source"`
	ApprovalRef string       `json:"approvalRef" yaml:"approvalRef"`
	AppliesTo   AppliesTo170 `json:"appliesTo" yaml:"appliesTo"`
	Supersedes  []string     `json:"supersedes" yaml:"supersedes"`
	Exceptions  []string     `json:"exceptions" yaml:"exceptions"`
}

type SourceRecord170 struct {
	SourceID string   `json:"sourceId"`
	Root     string   `json:"root"`
	Path     string   `json:"path"`
	Kind     string   `json:"kind"`
	Required bool     `json:"required"`
	SHA256   string   `json:"sha256,omitempty"`
	Rule     *Rule170 `json:"rule,omitempty"`
	Status   string   `json:"status"` // READY | BLOCKED | NOT_APPLICABLE
	Reasons  []string `json:"reasons"`
}

type BusinessCheck170 struct {
	ReviewUnitID string   `json:"reviewUnitId"`
	RuleKey      string   `json:"ruleKey"` // BUSINESS:<sourceId>:<ruleId>
	SourceID     string   `json:"sourceId"`
	RuleID       string   `json:"ruleId"`
	Status       string   `json:"status"` // READY | BLOCKED
	Reasons      []string `json:"reasons"`
}

type Manifest170 struct {
	RunID         string             `json:"runId"`
	ProjectID     string             `json:"projectId"`
	Status        string             `json:"status"` // NOT_CONFIGURED | READY | PARTIAL | INVALID
	BindingSHA256 string             `json:"bindingSha256"`
	Sources       []SourceRecord170  `json:"sources"`
	Checks        []BusinessCheck170 `json:"checks"`
	Issues        []string           `json:"issues"`
}

type Document170 struct {
	SourceID string
	Content  string
}

type LoadResult170 struct {
	Manifest  Manifest170
	Documents []Document170 // run-local only; never serialized into run records
}

type LoadInput170 struct {
	RunID         string
	RepoRoot      string
	Binding       Binding170
	BindingSHA256 string
	Units         []Unit170
}
