package analysis

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"codea-harness-tools/internal/changeset"
)

// legacyChangeSetSHA256E737 reproduces the exact ChangeSet identity stored in
// pre-hotfix 1.6.2 certificates at e737a3b6. It is used only by the retained
// legacy certificate loader path; canonical Snapshot authority continues to
// use snapshotSha256/gitStateSha256.
func legacyChangeSetSHA256E737(snapshot changeset.Snapshot) (string, error) {
	canonical, err := json.Marshal(struct {
		BaseRef string           `json:"baseRef"`
		Head    string           `json:"head"`
		Files   []changeset.File `json:"files"`
	}{
		BaseRef: snapshot.RequestedBaseRef,
		Head: snapshot.HeadCommit,
		Files: snapshot.Files,
	})
	if err != nil {
		return "", fmt.Errorf("LEGACY_CHANGE_SET_CANONICALIZE_FAILED: %w", err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(canonical)), nil
}
