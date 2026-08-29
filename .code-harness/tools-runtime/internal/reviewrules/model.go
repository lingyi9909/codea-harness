package reviewrules

type Kind string

const (
	KindAgent   Kind = "AGENT"
	KindMachine Kind = "MACHINE"
)

type Rule struct {
	ID               string   `yaml:"id" json:"id"`
	Version          int      `yaml:"version" json:"version"`
	Kind             Kind     `yaml:"kind" json:"kind"`
	SeverityDefault  string   `yaml:"severityDefault" json:"severityDefault"`
	Roles            []string `yaml:"roles" json:"roles"`
	RequiredEvidence []string `yaml:"requiredEvidence" json:"requiredEvidence"`
	Prompt           string   `yaml:"prompt" json:"prompt"`
}

type Dispatch struct {
	ReviewUnitID     string   `json:"reviewUnitId"`
	RuleID           string   `json:"ruleId"`
	RuleVersion      int      `json:"ruleVersion"`
	Kind             Kind     `json:"kind"`
	SeverityDefault  string   `json:"severityDefault"`
	RequiredEvidence []string `json:"requiredEvidence"`
	DispatchReason   []string `json:"dispatchReason"`
}

type Manifest struct {
	RunID              string     `json:"runId"`
	ReviewUnitsSHA256  string     `json:"reviewUnitsSha256"`
	RuleCatalogSHA256  string     `json:"ruleCatalogSha256"`
	Dispatches         []Dispatch `json:"dispatches"`
	SHA256             string     `json:"sha256"`
}
