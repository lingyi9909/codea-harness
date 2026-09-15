package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"codea-harness-tools/internal/knowledge"
	"codea-harness-tools/internal/reviewprogress"
	"codea-harness-tools/internal/reviewrules"
	"codea-harness-tools/internal/reviewunit"
)

type reviewKnowledgeRequest170 struct {
	RunID string `json:"runId"`
}

var exactKnowledgeEntryPoint170 = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$.]*#[A-Za-z_$][A-Za-z0-9_$]*\([^)]*\)$`)

func decodeReviewKnowledgeRequest170(pathRunID string, raw []byte) (reviewKnowledgeRequest170, error) {
	var req reviewKnowledgeRequest170
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return req, fmt.Errorf("REVIEW_KNOWLEDGE_REQUEST_INVALID: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return req, fmt.Errorf("REVIEW_KNOWLEDGE_REQUEST_INVALID: trailing JSON")
	}
	req.RunID = strings.TrimSpace(req.RunID)
	if req.RunID == "" || req.RunID != strings.TrimSpace(pathRunID) {
		return req, fmt.Errorf("RUN_ID_MISMATCH: body=%q path=%q", req.RunID, pathRunID)
	}
	return req, nil
}

func validateReviewKnowledgeProgress170(state reviewprogress.State) error {
	if state.ProtocolVersion != reviewprogress.Protocol170 {
		return fmt.Errorf("REVIEW_PROTOCOL_MISMATCH: knowledge requires protocol %s", reviewprogress.Protocol170)
	}
	if state.Status != reviewprogress.StatusRunning || state.CurrentStage != reviewprogress.StageReviewPlanning {
		return fmt.Errorf("REVIEW_KNOWLEDGE_PROGRESS_INVALID: current=%s status=%s", state.CurrentStage, state.Status)
	}
	return nil
}

func knowledgeUnits170(manifest reviewunit.Manifest) []knowledge.Unit170 {
	out := make([]knowledge.Unit170, 0, len(manifest.Units))
	for _, unit := range manifest.Units {
		paths := []string{}
		seenPaths := map[string]bool{}
		for _, file := range unit.Files {
			if strings.TrimSpace(file.Workspace) != "current" {
				continue
			}
			p := filepath.ToSlash(filepath.Clean(strings.TrimSpace(file.Path)))
			if p == "." || filepath.IsAbs(p) || strings.HasPrefix(p, "../") || seenPaths[p] {
				continue
			}
			seenPaths[p] = true
			paths = append(paths, p)
		}
		sort.Strings(paths)
		entryPoints := []string{}
		entry := strings.TrimSpace(unit.EntryPoint)
		if exactKnowledgeEntryPoint170.MatchString(entry) {
			entryPoints = append(entryPoints, entry)
		}
		out = append(out, knowledge.Unit170{
			ID:          strings.TrimSpace(unit.ID),
			Paths:       paths,
			EntryPoints: entryPoints,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func reviewKnowledgeArtifactPath170(runID string) string {
	return filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "analysis", "review-knowledge.json"))
}

func invalidKnowledgeManifest170(runID string, raw []byte, err error) knowledge.LoadResult170 {
	sum := sha256.Sum256(raw)
	issues := []string{err.Error()}
	return knowledge.LoadResult170{
		Manifest: knowledge.Manifest170{
			RunID:         runID,
			ProjectID:     "INVALID",
			Status:        "INVALID",
			BindingSHA256: fmt.Sprintf("%x", sum[:]),
			Sources:       []knowledge.SourceRecord170{},
			Checks:        []knowledge.BusinessCheck170{},
			Issues:        issues,
		},
		Documents: []knowledge.Document170{},
	}
}

func notConfiguredKnowledge170(runID string) knowledge.LoadResult170 {
	return knowledge.LoadResult170{
		Manifest: knowledge.Manifest170{
			RunID:         runID,
			ProjectID:     "UNCONFIGURED",
			Status:        "NOT_CONFIGURED",
			BindingSHA256: "ABSENT",
			Sources:       []knowledge.SourceRecord170{},
			Checks:        []knowledge.BusinessCheck170{},
			Issues:        []string{},
		},
		Documents: []knowledge.Document170{},
	}
}

func loadReviewKnowledge170(repoRoot, runID string, units []knowledge.Unit170) (knowledge.LoadResult170, error) {
	bindingPath := filepath.Join(repoRoot, ".code-harness", "context.yaml")
	raw, err := os.ReadFile(bindingPath)
	if errors.Is(err, os.ErrNotExist) {
		return notConfiguredKnowledge170(runID), nil
	}
	if err != nil {
		return knowledge.LoadResult170{}, fmt.Errorf("KNOWLEDGE_BINDING_READ_FAILED: %w", err)
	}
	binding, digest, err := knowledge.ParseBinding170(raw)
	if err != nil {
		return invalidKnowledgeManifest170(runID, raw, err), nil
	}
	return knowledge.Load170(knowledge.LoadInput170{
		RunID:         runID,
		RepoRoot:      repoRoot,
		Binding:       binding,
		BindingSHA256: digest,
		Units:         units,
	})
}

func readReviewKnowledgeManifest170(repoRoot, runID string) (knowledge.Manifest170, error) {
	path := filepath.Join(repoRoot, filepath.FromSlash(reviewKnowledgeArtifactPath170(runID)))
	raw, err := os.ReadFile(path)
	if err != nil {
		return knowledge.Manifest170{}, fmt.Errorf("REVIEW_KNOWLEDGE_REQUIRED: %w", err)
	}
	if err := validateReviewContract153("review-knowledge.schema.json", raw); err != nil {
		return knowledge.Manifest170{}, fmt.Errorf("REVIEW_KNOWLEDGE_SCHEMA_INVALID: %w", err)
	}
	var manifest knowledge.Manifest170
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&manifest); err != nil {
		return manifest, fmt.Errorf("REVIEW_KNOWLEDGE_INVALID: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return manifest, fmt.Errorf("REVIEW_KNOWLEDGE_INVALID: trailing JSON")
	}
	if manifest.RunID != runID {
		return manifest, fmt.Errorf("REVIEW_KNOWLEDGE_STALE: runId mismatch")
	}
	return manifest, nil
}

func verifyReviewKnowledgeUse170(repoRoot, runID string, units []knowledge.Unit170, expected knowledge.Manifest170) error {
	bindingPath := filepath.Join(repoRoot, ".code-harness", "context.yaml")
	raw, err := os.ReadFile(bindingPath)
	if expected.Status == "NOT_CONFIGURED" {
		if errors.Is(err, os.ErrNotExist) && expected.BindingSHA256 == "ABSENT" {
			return nil
		}
		return fmt.Errorf("KNOWLEDGE_BINDING_CHANGED: knowledge configuration changed after NOT_CONFIGURED")
	}
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("KNOWLEDGE_BINDING_CHANGED: context.yaml disappeared")
		}
		return fmt.Errorf("KNOWLEDGE_BINDING_READ_FAILED: %w", err)
	}
	binding, digest, parseErr := knowledge.ParseBinding170(raw)
	if expected.Status == "INVALID" {
		fresh, loadErr := loadReviewKnowledge170(repoRoot, runID, units)
		if loadErr != nil {
			return loadErr
		}
		want, digestErr := knowledge.Digest170(expected)
		if digestErr != nil {
			return digestErr
		}
		got, digestErr := knowledge.Digest170(fresh.Manifest)
		if digestErr != nil {
			return digestErr
		}
		if parseErr == nil || want != got {
			return fmt.Errorf("KNOWLEDGE_BINDING_CHANGED: invalid binding changed")
		}
		return nil
	}
	if parseErr != nil {
		return fmt.Errorf("KNOWLEDGE_BINDING_CHANGED: %w", parseErr)
	}
	input := knowledge.LoadInput170{
		RunID:         runID,
		RepoRoot:      repoRoot,
		Binding:       binding,
		BindingSHA256: digest,
		Units:         units,
	}
	return knowledge.Verify170(input, expected)
}

func verifyReviewKnowledgeArtifactUse170(repoRoot, runID string, units reviewunit.Manifest, dispatch reviewrules.Manifest) error {
	state, err := reviewprogress.Read(repoRoot, runID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if state.ProtocolVersion != reviewprogress.Protocol170 {
		return nil
	}
	manifest, err := readReviewKnowledgeManifest170(repoRoot, runID)
	if err != nil {
		return err
	}
	if err := verifyReviewKnowledgeUse170(repoRoot, runID, knowledgeUnits170(units), manifest); err != nil {
		return err
	}
	digest, err := knowledge.Digest170(manifest)
	if err != nil {
		return fmt.Errorf("REVIEW_KNOWLEDGE_DIGEST_FAILED: %w", err)
	}
	if strings.TrimSpace(dispatch.KnowledgeSHA256) != digest {
		return fmt.Errorf("KNOWLEDGE_BINDING_CHANGED: final RuleDispatch knowledgeSha256 no longer matches review knowledge")
	}
	return nil
}

func runReviewKnowledge170(args []string) error {
	fs := flag.NewFlagSet("review knowledge", flag.ContinueOnError)
	inputPath := fs.String("input", "", "same-run review knowledge request under .code-harness/runs/<runId>/requests")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || strings.TrimSpace(*inputPath) == "" {
		return errors.New("review knowledge requires --input")
	}
	runID, cleanInput, err := validateAnalysisRequestPath153(*inputPath)
	if err != nil {
		return errors.New("review knowledge input must be under .code-harness/runs/<runId>/requests")
	}
	if err := verifyReviewTransportPath153(runID, cleanInput); err != nil {
		return err
	}
	raw, err := os.ReadFile(cleanInput)
	if err != nil {
		return fmt.Errorf("REVIEW_KNOWLEDGE_REQUEST_READ_FAILED: %w", err)
	}
	req, err := decodeReviewKnowledgeRequest170(runID, raw)
	if err != nil {
		return err
	}
	state, err := reviewprogress.Read(".", req.RunID)
	if err != nil {
		return err
	}
	if err := validateReviewKnowledgeProgress170(state); err != nil {
		return err
	}
	units, err := reviewunit.Load(reviewunit.BuildInput{RunID: req.RunID, CertifiedRunID: req.RunID, RepoRoot: "."})
	if err != nil {
		return fmt.Errorf("REVIEW_KNOWLEDGE_UNITS_UNAVAILABLE: %w", err)
	}
	loaded, err := loadReviewKnowledge170(".", req.RunID, knowledgeUnits170(units))
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(loaded.Manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("REVIEW_KNOWLEDGE_ENCODE_FAILED: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := validateReviewContract153("review-knowledge.schema.json", encoded); err != nil {
		return fmt.Errorf("REVIEW_KNOWLEDGE_SCHEMA_INVALID: %w", err)
	}
	artifactPath := reviewKnowledgeArtifactPath170(req.RunID)
	if err := atomicReviewWrite153(filepath.FromSlash(artifactPath), encoded); err != nil {
		return fmt.Errorf("REVIEW_KNOWLEDGE_WRITE_FAILED: %w", err)
	}
	knowledgeSHA, err := knowledge.Digest170(loaded.Manifest)
	if err != nil {
		return fmt.Errorf("REVIEW_KNOWLEDGE_DIGEST_FAILED: %w", err)
	}
	return writeJSONAndStatus(map[string]any{
		"status":          loaded.Manifest.Status,
		"runId":           req.RunID,
		"artifactPath":    artifactPath,
		"knowledgeSha256": knowledgeSHA,
		"manifest":        loaded.Manifest,
		"documents":       loaded.Documents,
	}, true)
}
