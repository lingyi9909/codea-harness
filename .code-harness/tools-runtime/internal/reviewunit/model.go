package reviewunit

type Mode string

const (
	ModeFull     Mode = "FULL"
	ModeTargeted Mode = "TARGETED"
)

type FileRef struct {
	Path      string `json:"path"`
	Role      string `json:"role"`
	Changed   bool   `json:"changed"`
	Workspace string `json:"workspace"`
}

type HunkRef struct {
	Path     string `json:"path"`
	NewStart int    `json:"newStart"`
	NewLines int    `json:"newLines"`
}

type Unit struct {
	ID             string    `json:"id"`
	EntryPoint     string    `json:"entryPoint,omitempty"`
	Chain          []string  `json:"chain,omitempty"`
	ContextSymbols []string  `json:"contextSymbols,omitempty"`
	Files          []FileRef `json:"files"`
	ChangedHunks   []HunkRef `json:"changedHunks,omitempty"`
}

type Manifest struct {
	RunID                string `json:"runId"`
	HarnessVersion       string `json:"harnessVersion"`
	Mode                 Mode   `json:"mode"`
	ChangeSetSHA256      string `json:"changeSetSha256"`
	ChangeAnalysisSHA256 string `json:"changeAnalysisSha256"`
	ReviewScopeSHA256    string `json:"reviewScopeSha256,omitempty"`
	Units                []Unit `json:"units"`
	SHA256               string `json:"sha256"`
}

type BuildInput struct {
	RunID          string
	RepoRoot       string
	CertifiedRunID string
}
