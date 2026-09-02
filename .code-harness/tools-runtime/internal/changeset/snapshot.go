package changeset

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

func finalizeSnapshot162(snapshot Snapshot) (Snapshot, error) {
	if snapshot.Files == nil {
		snapshot.Files = []File{}
	}
	for i := range snapshot.Files {
		if snapshot.Files[i].Sources == nil {
			snapshot.Files[i].Sources = []Source{}
		}
		if snapshot.Files[i].Hunks == nil {
			snapshot.Files[i].Hunks = []Hunk{}
		}
	}
	hash, err := snapshotIdentitySHA256162(snapshot)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot.SnapshotSHA256 = hash
	snapshot.BaseRef = snapshot.RequestedBaseRef
	snapshot.Head = snapshot.HeadCommit
	snapshot.SHA256 = snapshot.SnapshotSHA256
	if err := validateCanonicalSnapshot162(snapshot); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

func snapshotIdentitySHA256162(snapshot Snapshot) (string, error) {
	identity := struct {
		ResolvedBaseCommit string `json:"resolvedBaseCommit"`
		MergeBase          string `json:"mergeBase"`
		HeadCommit         string `json:"headCommit"`
		IncludeWorkingTree bool   `json:"includeWorkingTree"`
		Files              []File `json:"files"`
	}{
		ResolvedBaseCommit: snapshot.ResolvedBaseCommit,
		MergeBase:          snapshot.MergeBase,
		HeadCommit:         snapshot.HeadCommit,
		IncludeWorkingTree: snapshot.IncludeWorkingTree,
		Files:              snapshot.Files,
	}
	canonical, err := json.Marshal(identity)
	if err != nil {
		return "", fmt.Errorf("CHANGE_SET_CANONICALIZE_FAILED: %w", err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(canonical)), nil
}

func validateCanonicalSnapshot162(snapshot Snapshot) error {
	if snapshot.RequestedBaseRef == "" || snapshot.ResolvedBaseCommit == "" || snapshot.MergeBase == "" || snapshot.HeadCommit == "" || snapshot.CurrentBranch == "" {
		return errors.New("CHANGE_SET_SNAPSHOT_IDENTITY_INCOMPLETE")
	}
	for i, file := range snapshot.Files {
		if file.Path == "" {
			return fmt.Errorf("CHANGE_SET_SNAPSHOT_FILE_INVALID: index=%d", i)
		}
		if i > 0 && snapshot.Files[i-1].Path >= file.Path {
			return fmt.Errorf("CHANGE_SET_SNAPSHOT_FILES_NOT_CANONICAL: %s", file.Path)
		}
	}
	expected, err := snapshotIdentitySHA256162(snapshot)
	if err != nil {
		return err
	}
	if snapshot.SnapshotSHA256 != expected {
		return fmt.Errorf("CHANGE_SET_SNAPSHOT_HASH_MISMATCH: got=%s expected=%s", snapshot.SnapshotSHA256, expected)
	}
	return nil
}

// CanonicalBytes returns the stable Runtime-owned snapshot artifact bytes.
func CanonicalBytes(snapshot Snapshot) ([]byte, error) {
	if err := validateCanonicalSnapshot162(snapshot); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("CHANGE_SET_SNAPSHOT_ENCODE_FAILED: %w", err)
	}
	return append(data, '\n'), nil
}

// DecodeCanonical strictly decodes a Runtime-owned snapshot artifact and
// restores in-process legacy aliases without changing canonical JSON identity.
func DecodeCanonical(data []byte) (Snapshot, error) {
	var snapshot Snapshot
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("CHANGE_SET_SNAPSHOT_DECODE_FAILED: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Snapshot{}, errors.New("CHANGE_SET_SNAPSHOT_DECODE_FAILED: multiple JSON values are not allowed")
		}
		return Snapshot{}, fmt.Errorf("CHANGE_SET_SNAPSHOT_DECODE_FAILED: %w", err)
	}
	if err := validateCanonicalSnapshot162(snapshot); err != nil {
		return Snapshot{}, err
	}
	snapshot.BaseRef = snapshot.RequestedBaseRef
	snapshot.Head = snapshot.HeadCommit
	snapshot.SHA256 = snapshot.SnapshotSHA256
	return snapshot, nil
}
