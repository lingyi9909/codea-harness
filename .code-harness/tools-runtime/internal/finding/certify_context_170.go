package finding

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"codea-harness-tools/internal/knowledge"
	"codea-harness-tools/internal/nav"
	"codea-harness-tools/internal/reviewcontext"
	"codea-harness-tools/internal/reviewunit"
)

var exactKnowledgeEntryPointFinding170 = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$.]*#[A-Za-z_$][A-Za-z0-9_$]*\([^)]*\)$`)

func prepareCertifyContext170(ctx CertifyContext) (CertifyContext, error) {
	if !strings.HasPrefix(strings.TrimSpace(ctx.HarnessVersion), "1.7") {
		return ctx, nil
	}
	root := filepath.Clean(ctx.Verify.repoRoot)
	if strings.TrimSpace(ctx.Verify.repoRoot) == "" {
		return ctx, findingError160("FINDING_CERTIFY_CONTEXT_INVALID", "1.7 VerifyContext has no repository root")
	}

	rulesPath := filepath.Join(root, ".code-harness", "runs", ctx.RunID, "analysis", "review-rule-context.json")
	rulesBytes, err := os.ReadFile(rulesPath)
	if err != nil {
		return ctx, findingError160("CONTEXT_RELATION_NOT_VERIFIED", "1.7 RULES context unavailable: %v", err)
	}
	var rules reviewcontext.Context170
	if err := strictJSON170(rulesBytes, &rules); err != nil {
		return ctx, findingError160("CONTEXT_RELATION_NOT_VERIFIED", "decode RULES context: %v", err)
	}
	if rules.RunID != ctx.RunID || !strings.EqualFold(strings.TrimSpace(rules.Phase), "RULES") {
		return ctx, findingError160("CONTEXT_RELATION_NOT_VERIFIED", "RULES context identity mismatch")
	}
	for _, relation := range rules.Relations {
		if err := nav.ValidateRelation170(relation); err != nil {
			return ctx, findingError160("CONTEXT_RELATION_NOT_VERIFIED", "%v", err)
		}
	}
	ctx.Verify.reviewContext170 = &rules

	knowledgePath := filepath.Join(root, ".code-harness", "runs", ctx.RunID, "analysis", "review-knowledge.json")
	knowledgeBytes, err := os.ReadFile(knowledgePath)
	if err != nil {
		return ctx, findingError160("REVIEW_KNOWLEDGE_REQUIRED", "1.7 knowledge manifest unavailable: %v", err)
	}
	var manifest knowledge.Manifest170
	if err := strictJSON170(knowledgeBytes, &manifest); err != nil {
		return ctx, findingError160("REVIEW_KNOWLEDGE_INVALID", "%v", err)
	}
	if manifest.RunID != ctx.RunID {
		return ctx, findingError160("REVIEW_KNOWLEDGE_STALE", "runId mismatch")
	}
	if err := verifyKnowledgeManifestFresh170(root, ctx.RunID, ctx.Verify.units, manifest); err != nil {
		return ctx, err
	}
	knowledgeSHA, err := knowledge.Digest170(manifest)
	if err != nil {
		return ctx, findingError160("REVIEW_KNOWLEDGE_DIGEST_FAILED", "%v", err)
	}
	if strings.TrimSpace(ctx.Verify.dispatch.KnowledgeSHA256) != knowledgeSHA {
		return ctx, findingError160("KNOWLEDGE_BINDING_CHANGED", "RuleDispatch knowledgeSha256 mismatch")
	}
	ctx.Verify.knowledge170 = &manifest
	if ctx.KnowledgeSHA256 != "" && ctx.KnowledgeSHA256 != knowledgeSHA {
		return ctx, findingError160("FINDING_CERTIFY_CONTEXT_INVALID", "knowledgeSha256 differs from Runtime authority")
	}
	ctx.KnowledgeSHA256 = knowledgeSHA

	checksPath := filepath.Join(root, ".code-harness", "runs", ctx.RunID, "requests", "review-checks.json")
	checksBytes, err := os.ReadFile(checksPath)
	if err != nil {
		return ctx, findingError160("REVIEW_CHECK_MISSING", "1.7 Reviewer checks unavailable: %v", err)
	}
	var checks []CheckResult170
	if err := strictJSON170(checksBytes, &checks); err != nil {
		return ctx, findingError160("REVIEW_CHECK_INVALID", "%v", err)
	}
	if checks == nil {
		checks = []CheckResult170{}
	}
	summary, err := ValidateCheckResults170(ctx.Verify, checks)
	if err != nil {
		return ctx, err
	}
	checksSHA := hashFindingBytes160(checksBytes)
	if ctx.ReviewChecksSHA256 != "" && ctx.ReviewChecksSHA256 != checksSHA {
		return ctx, findingError160("FINDING_CERTIFY_CONTEXT_INVALID", "reviewChecksSha256 differs from Runtime authority")
	}
	ctx.ReviewChecksSHA256 = checksSHA
	ctx.ReviewContext = &summary
	return ctx, nil
}

func knowledgeUnitsFinding170(units reviewunit.Manifest) []knowledge.Unit170 {
	out := make([]knowledge.Unit170, 0, len(units.Units))
	for _, unit := range units.Units {
		paths := []string{}
		seen := map[string]bool{}
		for _, file := range unit.Files {
			if strings.TrimSpace(file.Workspace) != "current" {
				continue
			}
			p := filepath.ToSlash(filepath.Clean(strings.TrimSpace(file.Path)))
			if p == "." || filepath.IsAbs(p) || strings.HasPrefix(p, "../") || seen[p] {
				continue
			}
			seen[p] = true
			paths = append(paths, p)
		}
		sort.Strings(paths)
		entryPoints := []string{}
		entry := strings.TrimSpace(unit.EntryPoint)
		if exactKnowledgeEntryPointFinding170.MatchString(entry) {
			entryPoints = append(entryPoints, entry)
		}
		out = append(out, knowledge.Unit170{ID: strings.TrimSpace(unit.ID), Paths: paths, EntryPoints: entryPoints})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func verifyKnowledgeManifestFresh170(root, runID string, units reviewunit.Manifest, expected knowledge.Manifest170) error {
	bindingPath := filepath.Join(root, ".code-harness", "context.yaml")
	raw, err := os.ReadFile(bindingPath)
	if expected.Status == "NOT_CONFIGURED" {
		if errors.Is(err, os.ErrNotExist) && expected.BindingSHA256 == "ABSENT" {
			return nil
		}
		return findingError160("KNOWLEDGE_BINDING_CHANGED", "knowledge configuration changed after NOT_CONFIGURED")
	}
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return findingError160("KNOWLEDGE_BINDING_CHANGED", "context.yaml disappeared")
		}
		return findingError160("KNOWLEDGE_BINDING_READ_FAILED", "%v", err)
	}
	if expected.Status == "INVALID" {
		sum := sha256.Sum256(raw)
		if expected.BindingSHA256 != fmt.Sprintf("%x", sum[:]) {
			return findingError160("KNOWLEDGE_BINDING_CHANGED", "invalid binding bytes changed")
		}
		if _, _, parseErr := knowledge.ParseBinding170(raw); parseErr == nil {
			return findingError160("KNOWLEDGE_BINDING_CHANGED", "previously invalid binding is now valid")
		}
		return nil
	}
	binding, digest, err := knowledge.ParseBinding170(raw)
	if err != nil {
		return findingError160("KNOWLEDGE_BINDING_CHANGED", "%v", err)
	}
	input := knowledge.LoadInput170{RunID: runID, RepoRoot: root, Binding: binding, BindingSHA256: digest, Units: knowledgeUnitsFinding170(units)}
	if err := knowledge.Verify170(input, expected); err != nil {
		return findingError160("KNOWLEDGE_SOURCE_CHANGED", "%v", err)
	}
	return nil
}

func strictJSON170(data []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}
