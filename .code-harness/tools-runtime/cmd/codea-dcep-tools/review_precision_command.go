package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/finding"
	"codea-harness-tools/internal/requestcontract"
	"codea-harness-tools/internal/reviewrules"
	"codea-harness-tools/internal/reviewunit"
)

func runReview160(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "units":
			return runReviewUnits160(args[1:])
		case "dispatch":
			return runReviewDispatch160(args[1:])
		case "certify-findings":
			return runReviewCertifyFindings160(args[1:])
		}
	}
	return runReview(args)
}

func runReviewUnits160(args []string) error {
	fs := flag.NewFlagSet("review units", flag.ContinueOnError)
	runID := fs.String("run-id", "", "same-run Certified ChangeAnalysis / Review Scope run id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *runID == "" {
		return errors.New("review units requires --run-id")
	}
	canonicalRunID := strings.TrimSpace(*runID)
	if canonicalRunID != *runID {
		return fmt.Errorf("REVIEW_UNIT_RUN_ID_INVALID: %q", *runID)
	}
	manifest, err := reviewunit.Build(reviewunit.BuildInput{RunID: canonicalRunID, CertifiedRunID: canonicalRunID, RepoRoot: "."})
	if err != nil {
		return err
	}
	encoded, err := reviewunit.CanonicalBytes(manifest)
	if err != nil {
		return fmt.Errorf("REVIEW_UNIT_ENCODE_FAILED: %w", err)
	}
	if err := validateReviewContract153("review-unit.schema.json", encoded); err != nil {
		return fmt.Errorf("REVIEW_UNIT_SCHEMA_INVALID: %w", err)
	}
	artifactPath := filepath.Join(".code-harness", "runs", canonicalRunID, "analysis", "review-units.json")
	if err := atomicReviewWrite153(artifactPath, encoded); err != nil {
		return fmt.Errorf("REVIEW_UNIT_WRITE_FAILED: %w", err)
	}
	return writeJSONAndStatus(map[string]any{
		"status":       "READY",
		"artifactPath": filepath.ToSlash(artifactPath),
		"manifest":     manifest,
	}, true)
}

func runReviewDispatch160(args []string) error {
	fs := flag.NewFlagSet("review dispatch", flag.ContinueOnError)
	runID := fs.String("run-id", "", "same-run Runtime-owned ReviewUnit run id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *runID == "" {
		return errors.New("review dispatch requires --run-id")
	}
	canonicalRunID := strings.TrimSpace(*runID)
	if canonicalRunID != *runID {
		return fmt.Errorf("RULE_DISPATCH_RUN_ID_INVALID: %q", *runID)
	}
	units, err := reviewunit.Load(reviewunit.BuildInput{RunID: canonicalRunID, CertifiedRunID: canonicalRunID, RepoRoot: "."})
	if err != nil {
		return fmt.Errorf("RULE_DISPATCH_STALE: %w", err)
	}
	rules, catalogSHA, err := reviewrules.LoadCatalog(filepath.Join(".code-harness", "review-rules", "spring-v1.yaml"))
	if err != nil {
		return err
	}
	manifest, err := reviewrules.BuildDispatch(units, rules, catalogSHA)
	if err != nil {
		return err
	}
	encoded, err := reviewrules.CanonicalBytes(manifest)
	if err != nil {
		return err
	}
	if err := validateReviewContract153("rule-dispatch.schema.json", encoded); err != nil {
		return fmt.Errorf("RULE_DISPATCH_SCHEMA_INVALID: %w", err)
	}
	artifactPath := filepath.Join(".code-harness", "runs", canonicalRunID, "analysis", "rule-dispatch.json")
	if err := atomicReviewWrite153(artifactPath, encoded); err != nil {
		return fmt.Errorf("RULE_DISPATCH_WRITE_FAILED: %w", err)
	}
	return writeJSONAndStatus(map[string]any{
		"status":       "READY",
		"artifactPath": filepath.ToSlash(artifactPath),
		"manifest":     manifest,
	}, true)
}

type findingCertifyRequest160 struct {
	RunID         string `json:"runId"`
	ProposalsPath string `json:"proposalsPath"`
}

func runReviewCertifyFindings160(args []string) error {
	fs := flag.NewFlagSet("review certify-findings", flag.ContinueOnError)
	input := fs.String("input", "", "same-run finding certification request under .code-harness/runs/<runId>/requests")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || strings.TrimSpace(*input) == "" {
		return errors.New("review certify-findings requires --input")
	}
	runID, cleanInput, err := validateAnalysisRequestPath153(*input)
	if err != nil {
		return errors.New("finding certify input must be under .code-harness/runs/<runId>/requests")
	}
	if err := verifyReviewTransportPath153(runID, cleanInput); err != nil {
		return err
	}
	requestBytes, err := os.ReadFile(cleanInput)
	if err != nil {
		return fmt.Errorf("FINDING_CERTIFY_REQUEST_READ_FAILED: %w", err)
	}
	if err := requestcontract.Validate("finding-certify-request.schema.json", requestBytes); err != nil {
		return fmt.Errorf("FINDING_CERTIFY_REQUEST_SCHEMA_INVALID: %w", err)
	}
	var req findingCertifyRequest160
	dec := json.NewDecoder(bytes.NewReader(requestBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return fmt.Errorf("FINDING_CERTIFY_REQUEST_INVALID: %w", err)
	}
	if strings.TrimSpace(req.RunID) != runID || req.RunID != strings.TrimSpace(req.RunID) {
		return fmt.Errorf("FINDING_CERTIFY_RUN_ID_MISMATCH: body runId %q path runId %q", req.RunID, runID)
	}
	expectedProposals := path.Clean(".code-harness/runs/" + runID + "/requests/finding-proposals.json")
	candidate := path.Clean(strings.ReplaceAll(strings.TrimSpace(req.ProposalsPath), "\\", "/"))
	if candidate != expectedProposals || candidate != strings.ReplaceAll(req.ProposalsPath, "\\", "/") {
		return fmt.Errorf("FINDING_PROPOSALS_PATH_INVALID: must be %s", expectedProposals)
	}
	proposalBytes, err := os.ReadFile(filepath.FromSlash(candidate))
	if err != nil {
		return fmt.Errorf("FINDING_PROPOSALS_READ_FAILED: %w", err)
	}
	if err := validateReviewContract153("finding-proposals.schema.json", proposalBytes); err != nil {
		return fmt.Errorf("FINDING_PROPOSALS_SCHEMA_INVALID: %w", err)
	}
	proposals, err := finding.DecodeProposals(proposalBytes)
	if err != nil {
		return err
	}
	verifyCtx, err := finding.LoadVerifyContext(".", runID, filepath.Join(".code-harness", "bin", "ast-grep.exe"))
	if err != nil {
		return err
	}
	analysisPath := filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json"))
	_, analysisCert, err := analysisruntime.LoadCertified(".", analysisPath)
	if err != nil {
		return err
	}
	units, err := reviewunit.Load(reviewunit.BuildInput{RunID: runID, CertifiedRunID: runID, RepoRoot: "."})
	if err != nil {
		return err
	}
	unitBytes, err := os.ReadFile(filepath.Join(".code-harness", "runs", runID, "analysis", "review-units.json"))
	if err != nil {
		return fmt.Errorf("FINDING_REVIEW_UNITS_READ_FAILED: %w", err)
	}
	dispatchBytes, err := os.ReadFile(filepath.Join(".code-harness", "runs", runID, "analysis", "rule-dispatch.json"))
	if err != nil {
		return fmt.Errorf("FINDING_RULE_DISPATCH_READ_FAILED: %w", err)
	}
	ctx := finding.CertifyContext{
		Verify:                 verifyCtx,
		RunID:                  runID,
		HarnessVersion:         analysisCert.RuntimeVersion,
		ChangeSetSHA256:        analysisCert.ChangeSetSHA256,
		ChangeAnalysisSHA256:   analysisCert.AnalysisSHA256,
		ReviewUnitsSHA256:      fmt.Sprintf("%x", sha256.Sum256(unitBytes)),
		RuleDispatchSHA256:     fmt.Sprintf("%x", sha256.Sum256(dispatchBytes)),
		FindingProposalsSHA256: fmt.Sprintf("%x", sha256.Sum256(proposalBytes)),
		Mode:                   string(units.Mode),
		ScopeSHA256:            units.ReviewScopeSHA256,
	}
	set, cert, rejections, err := finding.Certify(ctx, proposals)
	if err != nil {
		return err
	}
	if err := finding.WriteCertified(".", set, cert); err != nil {
		return err
	}
	setPath := filepath.Join(".code-harness", "runs", runID, "analysis", "certified-findings.json")
	certPath := filepath.Join(".code-harness", "runs", runID, "analysis", "certified-findings.cert.json")
	return writeJSONAndStatus(map[string]any{
		"status":          "CERTIFIED",
		"findingsPath":    filepath.ToSlash(setPath),
		"certificatePath": filepath.ToSlash(certPath),
		"findingCount":    len(set.Findings),
		"rejections":      rejections,
	}, true)
}
