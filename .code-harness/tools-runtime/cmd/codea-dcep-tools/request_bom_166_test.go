package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// A BOM written by Windows editors must not prevent the real schema/decode
// pipeline from publishing a canonical snapshot. Invalid input must not replace it.
func Test166SnapshotRequestBOMCompatibilityRemainsStrict(t *testing.T) {
	root := t.TempDir()
	git153Cmd(t, root, "init")
	git153Cmd(t, root, "config", "user.email", "bom@example.test")
	git153Cmd(t, root, "config", "user.name", "BOM ingress test")
	mustWrite153Cmd(t, filepath.Join(root, "seed.txt"), "seed\n")
	git153Cmd(t, root, "add", ".")
	git153Cmd(t, root, "commit", "-m", "base")
	copyTask153CommandContract(t, root, "change-set-request.schema.json")
	copyTask153CommandContract(t, root, "change-set.schema.json")
	const requestPath = ".code-harness/runs/bom166/requests/snapshot.json"
	const artifactPath = ".code-harness/runs/bom166/analysis/change-set.json"
	const valid = `{"runId":"bom166","baseRef":"HEAD","includeWorkingTree":true}`
	const bom = "\xef\xbb\xbf"

	withChdir153Cmd(t, root, func() {
		var canonical []byte
		for _, input := range []string{valid, bom + valid} {
			mustWrite153Cmd(t, requestPath, input)
			if err := run([]string{"analysis", "snapshot", "--input", requestPath}); err != nil {
				t.Fatalf("valid snapshot request %q rejected: %v", input, err)
			}
			data, err := os.ReadFile(artifactPath)
			if err != nil {
				t.Fatal(err)
			}
			var snapshot map[string]any
			if err := json.Unmarshal(data, &snapshot); err != nil {
				t.Fatal(err)
			}
			if snapshot["requestedBaseRef"] != "HEAD" {
				t.Fatalf("unexpected snapshot: %s", data)
			}
			if hash, _ := snapshot["snapshotSha256"].(string); len(hash) != 64 {
				t.Fatalf("missing snapshot hash: %s", data)
			}
			canonical = data
		}
		for _, tc := range []struct{ name, input string }{
			{"double BOM", bom + bom + valid},
			{"BOM after whitespace", " " + bom + valid},
			{"malformed", bom + `{"runId":`},
			{"trailing JSON", bom + valid + `{}`},
			{"unknown field", bom + `{"runId":"bom166","baseRef":"HEAD","includeWorkingTree":true,"authority":"forged"}`},
			{"invalid UTF8", bom + `{"runId":"bom166","baseRef":"` + "\xff" + `","includeWorkingTree":true}`},
			{"UTF16 LE", "\xff\xfe{\x00}\x00"},
			{"UTF16 BE", "\xfe\xff\x00{\x00}"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				mustWrite153Cmd(t, requestPath, tc.input)
				if err := run([]string{"analysis", "snapshot", "--input", requestPath}); err == nil {
					t.Fatal("invalid request accepted")
				}
				data, err := os.ReadFile(artifactPath)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(data, canonical) {
					t.Fatal("rejected request changed canonical snapshot")
				}
			})
		}
	})
}
