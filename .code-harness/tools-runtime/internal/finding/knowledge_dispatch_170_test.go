package finding

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/knowledge"
	"codea-harness-tools/internal/reviewrules"
	"codea-harness-tools/internal/reviewunit"
)

func Test170KnowledgeDispatchAuthorityRebuildUsesBoundManifest(t *testing.T) {
	root := t.TempDir()
	runID := "run-knowledge-finding"
	analysisDir := filepath.Join(root, ".code-harness", "runs", runID, "analysis")
	contractsDir := filepath.Join(root, ".code-harness", "contracts")
	if err := os.MkdirAll(analysisDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(contractsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	schemaBytes, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "review-knowledge.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(contractsDir, "review-knowledge.schema.json"), schemaBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	units := reviewunit.Manifest{
		RunID:                runID,
		HarnessVersion:       "1.7.0",
		Mode:                 reviewunit.ModeFull,
		ChangeSetSHA256:      strings.Repeat("a", 64),
		ChangeAnalysisSHA256: strings.Repeat("b", 64),
		Units: []reviewunit.Unit{{
			ID:    "RU-A",
			Files: []reviewunit.FileRef{{Path: "src/main/java/A.java", Role: "Code", Changed: true, Workspace: "current"}},
		}},
	}
	unsignedUnits, err := reviewunit.CanonicalBytes(units)
	if err != nil {
		t.Fatal(err)
	}
	units.SHA256 = fmt.Sprintf("%x", sha256.Sum256(unsignedUnits))

	rules, catalogSHA, err := reviewrules.LoadCatalog(filepath.Join("..", "..", "..", "review-rules", "spring-v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	km := knowledge.Manifest170{
		RunID: runID, ProjectID: "UNCONFIGURED", Status: "NOT_CONFIGURED", BindingSHA256: "ABSENT",
		Sources: []knowledge.SourceRecord170{}, Checks: []knowledge.BusinessCheck170{}, Issues: []string{},
	}
	knowledgeSHA, err := knowledge.Digest170(km)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.MarshalIndent(km, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(filepath.Join(analysisDir, "review-knowledge.json"), encoded, 0o644); err != nil {
		t.Fatal(err)
	}

	claimed, err := reviewrules.BuildDispatch170(units, rules, catalogSHA, km)
	if err != nil {
		t.Fatal(err)
	}
	rebuilt, err := rebuildDispatchAuthority170(root, runID, units, rules, catalogSHA, claimed)
	if err != nil {
		t.Fatal(err)
	}
	claimedBytes, err := reviewrules.CanonicalBytes(claimed)
	if err != nil {
		t.Fatal(err)
	}
	rebuiltBytes, err := reviewrules.CanonicalBytes(rebuilt)
	if err != nil {
		t.Fatal(err)
	}
	if string(claimedBytes) != string(rebuiltBytes) {
		t.Fatalf("knowledge-bound dispatch rebuild differs:\nclaimed=%s\nrebuilt=%s", claimedBytes, rebuiltBytes)
	}

	claimed.KnowledgeSHA256 = strings.Repeat("f", 64)
	if _, err := rebuildDispatchAuthority170(root, runID, units, rules, catalogSHA, claimed); err == nil || !strings.Contains(err.Error(), "ReviewKnowledge sha256 mismatch") {
		t.Fatalf("mismatched knowledge digest accepted: %v", err)
	}
	if knowledgeSHA == "" {
		t.Fatal("knowledge digest unexpectedly empty")
	}
}
