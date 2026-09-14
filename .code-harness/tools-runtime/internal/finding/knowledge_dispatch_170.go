package finding

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"codea-harness-tools/internal/knowledge"
	"codea-harness-tools/internal/reviewrules"
	"codea-harness-tools/internal/reviewunit"
	"codea-harness-tools/internal/schema"
)

func rebuildDispatchAuthority170(root, runID string, units reviewunit.Manifest, rules []reviewrules.Rule, catalogSHA string, claimed reviewrules.Manifest) (reviewrules.Manifest, error) {
	if strings.TrimSpace(claimed.KnowledgeSHA256) == "" {
		return reviewrules.BuildDispatch(units, rules, catalogSHA)
	}
	knowledgeManifest, err := loadKnowledgeAuthority170(root, runID, claimed.KnowledgeSHA256)
	if err != nil {
		return reviewrules.Manifest{}, err
	}
	return reviewrules.BuildDispatch170(units, rules, catalogSHA, knowledgeManifest)
}

func loadKnowledgeAuthority170(root, runID, expectedSHA string) (knowledge.Manifest170, error) {
	artifact := filepath.Join(root, ".code-harness", "runs", runID, "analysis", "review-knowledge.json")
	raw, err := os.ReadFile(artifact)
	if err != nil {
		return knowledge.Manifest170{}, findingError160("FINDING_PROPOSAL_INVALID", "read ReviewKnowledge authority: %v", err)
	}
	schemaBytes, err := os.ReadFile(filepath.Join(root, ".code-harness", "contracts", "review-knowledge.schema.json"))
	if err != nil {
		return knowledge.Manifest170{}, findingError160("FINDING_PROPOSAL_INVALID", "read ReviewKnowledge schema: %v", err)
	}
	if err := schema.ValidateJSON(schemaBytes, raw); err != nil {
		return knowledge.Manifest170{}, findingError160("FINDING_PROPOSAL_INVALID", "ReviewKnowledge schema invalid: %v", err)
	}
	var manifest knowledge.Manifest170
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return manifest, findingError160("FINDING_PROPOSAL_INVALID", "decode ReviewKnowledge: %v", err)
	}
	if manifest.RunID != runID {
		return manifest, findingError160("FINDING_PROPOSAL_INVALID", "ReviewKnowledge runId mismatch")
	}
	digest, err := knowledge.Digest170(manifest)
	if err != nil {
		return manifest, findingError160("FINDING_PROPOSAL_INVALID", "digest ReviewKnowledge: %v", err)
	}
	if digest != strings.TrimSpace(expectedSHA) {
		return manifest, findingError160("FINDING_PROPOSAL_INVALID", "ReviewKnowledge sha256 mismatch")
	}
	canonical, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return manifest, findingError160("FINDING_PROPOSAL_INVALID", "canonicalize ReviewKnowledge: %v", err)
	}
	canonical = append(canonical, '\n')
	if !bytes.Equal(raw, canonical) {
		return manifest, findingError160("FINDING_PROPOSAL_INVALID", "ReviewKnowledge bytes are not canonical")
	}
	return manifest, nil
}
