package changeset

// Source identifies one local Git source that contributes a path to the Review Change Set.
type Source string

const (
	SourceCommitted Source = "COMMITTED"
	SourceStaged    Source = "STAGED"
	SourceUnstaged  Source = "UNSTAGED"
	SourceUntracked Source = "UNTRACKED"
)

type Hunk struct {
	OldStart int `json:"oldStart"`
	OldLines int `json:"oldLines"`
	NewStart int `json:"newStart"`
	NewLines int `json:"newLines"`
}

type File struct {
	Path    string   `json:"path"`
	Status  string   `json:"status"`
	Sources []Source `json:"sources"`
	Hunks   []Hunk   `json:"hunks"`
}

type Snapshot struct {
	BaseRef string `json:"baseRef"`
	Head    string `json:"head"`
	Files   []File `json:"files"`
	SHA256  string `json:"sha256"`
}
