package chain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateRejectsFilenameMismatchAndDuplicateProjectID(t *testing.T) {
	c := task3Chain()
	evidence := EvidenceSnapshot(task3Evidence())
	data, err := MarshalYAML(c)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("filename mismatch", func(t *testing.T) {
		root := t.TempDir()
		writeTask3Resource(t, root)
		dir := filepath.Join(root, ".code-harness", "chains")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "wrong-name.yaml"), data, 0o644); err != nil {
			t.Fatal(err)
		}
		got := Validate(root, c, evidence)
		if got.Status != ValidationInvalid || !strings.Contains(strings.Join(got.Errors, "\n"), "CHAIN_ID_FILENAME_MISMATCH") {
			t.Fatalf("filename/id mismatch must be INVALID: %+v", got)
		}
	})

	t.Run("duplicate id", func(t *testing.T) {
		root := t.TempDir()
		writeTask3Resource(t, root)
		dir := filepath.Join(root, ".code-harness", "chains")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"order-approve.yaml", "duplicate.yaml"} {
			if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		got := Validate(root, c, evidence)
		if got.Status != ValidationInvalid || !strings.Contains(strings.Join(got.Errors, "\n"), "DUPLICATE_PROJECT_CHAIN_ID") {
			t.Fatalf("duplicate project id must be INVALID: %+v", got)
		}
	})
}
