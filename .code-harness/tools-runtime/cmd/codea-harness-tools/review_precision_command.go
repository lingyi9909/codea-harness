package main

import (
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"strings"

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
