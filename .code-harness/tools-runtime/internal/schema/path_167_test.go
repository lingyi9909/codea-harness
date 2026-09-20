package schema

import (
	"os"
	"path/filepath"
	"testing"
)

func Test167LocalReferencesDoNotDependOnProjectDirectory(t *testing.T) {
	before, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "Review space & # percent% 中文")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(before); err != nil {
			t.Error(err)
		}
	})
	for _, id := range []string{"", `"$id":"proposal.schema.json",`} {
		source := []byte(`{` + id + `"$schema":"https://json-schema.org/draft/2020-12/schema","$defs":{"name":{"type":"string"}},"type":"object","properties":{"name":{"$ref":"#/$defs/name"}},"required":["name"]}`)
		if err := ValidateJSON(source, []byte(`{"name":"valid"}`)); err != nil {
			t.Fatalf("in-memory local ref with id %q: %v", id, err)
		}
		if err := ValidateJSON(source, []byte(`{"name":42}`)); err == nil {
			t.Fatal("invalid instance accepted")
		}
	}
}
