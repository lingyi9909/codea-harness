package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codea-harness-tools/internal/reviewprogress"
	"codea-harness-tools/internal/reviewrules"
	"codea-harness-tools/internal/reviewunit"
)

// runReview170 is the 1.7 entry shim. Historical runs that never produced a
// DISCOVERY context continue through the 1.6 dispatch path; same-run 1.7
// reviews defer REVIEW_PLANNING advancement until RULES context succeeds.
func runReview170(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "context":
			return runReviewContext170(args[1:])
		case "dispatch":
			if isContext170Run(args[1:]) { return runReviewDispatch170(args[1:]) }
		}
	}
	return runReview160(args)
}

func isContext170Run(args []string) bool {
	fs := flag.NewFlagSet("review dispatch detect 170", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	runID := fs.String("run-id", "", "")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 || strings.TrimSpace(*runID) == "" { return false }
	_, err := os.Stat(filepath.Join(".code-harness","runs",strings.TrimSpace(*runID),"analysis","review-context-discovery.json"))
	return err == nil
}

func runReviewDispatch170(args []string) error {
	fs := flag.NewFlagSet("review dispatch", flag.ContinueOnError)
	runID := fs.String("run-id", "", "same-run Runtime-owned ReviewUnit run id")
	if err := fs.Parse(args); err != nil { return err }
	if fs.NArg() != 0 || *runID == "" { return errors.New("review dispatch requires --run-id") }
	canonicalRunID := strings.TrimSpace(*runID)
	if canonicalRunID != *runID { return fmt.Errorf("RULE_DISPATCH_RUN_ID_INVALID: %q", *runID) }
	state, err := reviewprogress.Read(".", canonicalRunID)
	if err != nil { return err }
	if state.Status != reviewprogress.StatusRunning || state.CurrentStage != reviewprogress.StageReviewPlanning {
		return fmt.Errorf("RULE_DISPATCH_PROGRESS_INVALID: current=%s status=%s", state.CurrentStage, state.Status)
	}
	failPlanning := func(err error) error { return failReviewProgressStage164(canonicalRunID, reviewprogress.StageReviewPlanning, "REVIEW_PLANNING_FAILED", err) }
	units, err := reviewunit.Load(reviewunit.BuildInput{RunID:canonicalRunID, CertifiedRunID:canonicalRunID, RepoRoot:"."})
	if err != nil { return failPlanning(fmt.Errorf("RULE_DISPATCH_STALE: %w", err)) }
	rules, catalogSHA, err := reviewrules.LoadCatalog(filepath.Join(".code-harness","review-rules","spring-v1.yaml"))
	if err != nil { return failPlanning(err) }
	manifest, err := reviewrules.BuildDispatch(units,rules,catalogSHA)
	if err != nil { return failPlanning(err) }
	encoded, err := reviewrules.CanonicalBytes(manifest)
	if err != nil { return failPlanning(err) }
	if err := validateReviewContract153("rule-dispatch.schema.json",encoded); err != nil { return failPlanning(fmt.Errorf("RULE_DISPATCH_SCHEMA_INVALID: %w",err)) }
	artifactPath := filepath.Join(".code-harness","runs",canonicalRunID,"analysis","rule-dispatch.json")
	if err := atomicReviewWrite153(artifactPath,encoded); err != nil { return failPlanning(fmt.Errorf("RULE_DISPATCH_WRITE_FAILED: %w",err)) }
	// Deliberately no progress advance here. RULES context owns finalization of
	// REVIEW_PLANNING for 1.7 runs.
	return writeJSONAndStatus(map[string]any{"status":"READY","artifactPath":filepath.ToSlash(artifactPath),"manifest":manifest,"planningFinalized":false},true)
}
