package analysis

import (
	"context"
	"strings"
	"testing"

	"codea-harness-tools/internal/changeset"
)

func Test164Task1ARejectsEntrypointResultOutsideSnapshotScope(t *testing.T) {
	path := "src/main/java/acme/AController.java"
	snap := changeset.Snapshot{
		BaseRef: "develop",
		Head:    "abc",
		SHA256:  strings.Repeat("a", 64),
		Files: []changeset.File{{
			Path: path, Status: "A", Sources: []changeset.Source{changeset.SourceStaged},
		}},
	}
	scanner := fake153EntrypointScanner{current: map[string][]ControllerEndpoint{
		path: {{
			Controller: "UnrelatedController",
			Symbol: "UnrelatedController.get",
			Path: "src/main/java/acme/UnrelatedController.java",
			ControllerStartLine: 1, ControllerEndLine: 20,
			StartLine: 5, EndLine: 10,
		}},
	}}

	_, err := buildEntrypointInventoryWithScanner(context.Background(), "r164-scope-red", snap, Intent{Mode: "FULL"}, scanner)
	if err == nil || !strings.Contains(err.Error(), "ENTRYPOINT_SCAN_RESULT_OUT_OF_SCOPE") {
		t.Fatalf("out-of-scope scanner result must fail closed with ENTRYPOINT_SCAN_RESULT_OUT_OF_SCOPE, got %v", err)
	}
}
