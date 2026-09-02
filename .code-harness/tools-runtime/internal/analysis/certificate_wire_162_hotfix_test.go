package analysis

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func Test162HotfixCanonicalCertificateIncludesWorkingTreeWhenFalse(t *testing.T) {
	cert := Certificate{
		RunID:                     "canonical-false",
		RuntimeVersion:            "1.6.2",
		AnalysisSHA256:            strings.Repeat("a", 64),
		ChangeSetSHA256:           strings.Repeat("b", 64),
		EntrypointInventorySHA256: strings.Repeat("c", 64),
		BaseRef:                   "HEAD",
		Head:                      strings.Repeat("d", 40),
		ResolvedBaseCommit:        strings.Repeat("e", 40),
		MergeBase:                 strings.Repeat("f", 40),
		CurrentBranch:             "feature",
		IncludeWorkingTree:        false,
		SnapshotSHA256:            strings.Repeat("1", 64),
		Intent:                    &Intent{Mode: "FULL"},
	}

	data, err := json.MarshalIndent(cert, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"includeWorkingTree": false`)) {
		t.Fatalf("canonical certificate must encode includeWorkingTree=false, got:\n%s", data)
	}
}
