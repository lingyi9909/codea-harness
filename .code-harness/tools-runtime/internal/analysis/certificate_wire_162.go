package analysis

import "encoding/json"

// MarshalJSON preserves the pre-hotfix 1.6.2 certificate wire format for
// retained/legacy certificates while requiring canonical certificates to emit
// includeWorkingTree even when its value is false. Canonical identity fields
// are the authority-mode discriminator; legacy certificates have none of them.
func (c Certificate) MarshalJSON() ([]byte, error) {
	type certificateWire162 struct {
		RunID                     string  `json:"runId"`
		RuntimeVersion            string  `json:"runtimeVersion"`
		AnalysisSHA256            string  `json:"analysisSha256"`
		ChangeSetSHA256           string  `json:"changeSetSha256"`
		EntrypointInventorySHA256 string  `json:"entrypointInventorySha256"`
		BaseRef                   string  `json:"baseRef"`
		Head                      string  `json:"head"`
		ResolvedBaseCommit        string  `json:"resolvedBaseCommit,omitempty"`
		MergeBase                 string  `json:"mergeBase,omitempty"`
		CurrentBranch             string  `json:"currentBranch,omitempty"`
		IncludeWorkingTree        *bool   `json:"includeWorkingTree,omitempty"`
		SnapshotSHA256            string  `json:"snapshotSha256,omitempty"`
		Intent                    *Intent `json:"intent,omitempty"`
	}

	canonicalIdentity := c.ResolvedBaseCommit != "" || c.MergeBase != "" || c.CurrentBranch != "" || c.SnapshotSHA256 != ""
	var includeWorkingTree *bool
	if canonicalIdentity {
		value := c.IncludeWorkingTree
		includeWorkingTree = &value
	}

	return json.Marshal(certificateWire162{
		RunID: c.RunID,
		RuntimeVersion: c.RuntimeVersion,
		AnalysisSHA256: c.AnalysisSHA256,
		ChangeSetSHA256: c.ChangeSetSHA256,
		EntrypointInventorySHA256: c.EntrypointInventorySHA256,
		BaseRef: c.BaseRef,
		Head: c.Head,
		ResolvedBaseCommit: c.ResolvedBaseCommit,
		MergeBase: c.MergeBase,
		CurrentBranch: c.CurrentBranch,
		IncludeWorkingTree: includeWorkingTree,
		SnapshotSHA256: c.SnapshotSHA256,
		Intent: c.Intent,
	})
}
