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

// Snapshot is the Runtime-owned canonical Review ChangeSet fact.
// RequestedBaseRef and CurrentBranch are provenance. SnapshotSHA256 is derived
// from resolved Git identity + working-tree inclusion + canonical Review files,
// so equivalent ref spellings that resolve to the same commit share identity.
type Snapshot struct {
	RequestedBaseRef   string `json:"requestedBaseRef"`
	ResolvedBaseCommit string `json:"resolvedBaseCommit"`
	MergeBase          string `json:"mergeBase"`
	HeadCommit         string `json:"headCommit"`
	CurrentBranch      string `json:"currentBranch"`
	IncludeWorkingTree bool   `json:"includeWorkingTree"`
	Files              []File `json:"files"`
	SnapshotSHA256     string `json:"snapshotSha256"`

	// Legacy in-process aliases retained for existing 1.5.x/1.6.x consumers.
	// They are deliberately excluded from the canonical JSON artifact.
	BaseRef string `json:"-"`
	Head    string `json:"-"`
	SHA256  string `json:"-"`
}
