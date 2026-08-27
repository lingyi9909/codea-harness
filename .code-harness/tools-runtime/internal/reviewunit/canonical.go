package reviewunit

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
)

func CanonicalBytes(m Manifest) ([]byte, error) {
	normalized := normalizeManifest160(m)
	data, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func sealManifest160(m Manifest) (Manifest, error) {
	m = normalizeManifest160(m)
	m.SHA256 = ""
	unsigned, err := CanonicalBytes(m)
	if err != nil {
		return Manifest{}, err
	}
	m.SHA256 = hash160(unsigned)
	return normalizeManifest160(m), nil
}

func verifyManifestHash160(m Manifest) error {
	want := m.SHA256
	candidate := m
	candidate.SHA256 = ""
	unsigned, err := CanonicalBytes(candidate)
	if err != nil {
		return err
	}
	if want == "" || want != hash160(unsigned) {
		return fmt.Errorf("REVIEW_UNIT_STALE: manifest sha256 mismatch")
	}
	return nil
}

func normalizeManifest160(m Manifest) Manifest {
	m.Units = append([]Unit(nil), m.Units...)
	for i := range m.Units {
		m.Units[i] = normalizeUnit160(m.Units[i])
	}
	sort.Slice(m.Units, func(i, j int) bool {
		if m.Units[i].ID != m.Units[j].ID {
			return m.Units[i].ID < m.Units[j].ID
		}
		left, _ := json.Marshal(m.Units[i])
		right, _ := json.Marshal(m.Units[j])
		return bytes.Compare(left, right) < 0
	})
	if m.Units == nil {
		m.Units = []Unit{}
	}
	return m
}

func normalizeUnit160(u Unit) Unit {
	u.Chain = append([]string(nil), u.Chain...)
	u.ContextSymbols = uniqueSortedStrings160(u.ContextSymbols)
	u.Files = append([]FileRef(nil), u.Files...)
	sort.Slice(u.Files, func(i, j int) bool {
		if u.Files[i].Path != u.Files[j].Path { return u.Files[i].Path < u.Files[j].Path }
		if u.Files[i].Workspace != u.Files[j].Workspace { return u.Files[i].Workspace < u.Files[j].Workspace }
		if u.Files[i].Role != u.Files[j].Role { return u.Files[i].Role < u.Files[j].Role }
		return !u.Files[i].Changed && u.Files[j].Changed
	})
	u.Files = dedupeFiles160(u.Files)
	if u.Files == nil {
		u.Files = []FileRef{}
	}
	u.ChangedHunks = append([]HunkRef(nil), u.ChangedHunks...)
	sort.Slice(u.ChangedHunks, func(i, j int) bool {
		if u.ChangedHunks[i].Path != u.ChangedHunks[j].Path { return u.ChangedHunks[i].Path < u.ChangedHunks[j].Path }
		if u.ChangedHunks[i].NewStart != u.ChangedHunks[j].NewStart { return u.ChangedHunks[i].NewStart < u.ChangedHunks[j].NewStart }
		return u.ChangedHunks[i].NewLines < u.ChangedHunks[j].NewLines
	})
	u.ChangedHunks = dedupeHunks160(u.ChangedHunks)
	return u
}

func canonicalUnitDigest160(u Unit) (string, error) {
	u.ID = ""
	u = normalizeUnit160(u)
	data, err := json.Marshal(u)
	if err != nil {
		return "", err
	}
	return hash160(data), nil
}

func hash160(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func uniqueSortedStrings160(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, value := range in {
		if value == "" || seen[value] { continue }
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	if len(out) == 0 { return nil }
	return out
}

func dedupeFiles160(in []FileRef) []FileRef {
	seen := map[FileRef]bool{}
	out := make([]FileRef, 0, len(in))
	for _, file := range in {
		if seen[file] { continue }
		seen[file] = true
		out = append(out, file)
	}
	return out
}

func dedupeHunks160(in []HunkRef) []HunkRef {
	seen := map[HunkRef]bool{}
	out := make([]HunkRef, 0, len(in))
	for _, hunk := range in {
		if seen[hunk] { continue }
		seen[hunk] = true
		out = append(out, hunk)
	}
	return out
}
