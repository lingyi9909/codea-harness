package finding

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"codea-harness-tools/internal/knowledge"
	"codea-harness-tools/internal/nav"
	"codea-harness-tools/internal/reviewcontext"
)

func prepareCertifyContext170(ctx CertifyContext) (CertifyContext, error) {
	if !strings.HasPrefix(strings.TrimSpace(ctx.HarnessVersion), "1.7") {
		return ctx, nil
	}
	root := filepath.Clean(ctx.Verify.repoRoot)
	if root == "" || root == "." && strings.TrimSpace(ctx.Verify.repoRoot) == "" {
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
