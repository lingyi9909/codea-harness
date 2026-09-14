package main

import (
	"crypto/rand"
	"encoding/hex"
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

func runReview170(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "begin":
			return runReviewBegin170(args[1:])
		case "context":
			return runReviewContext170(args[1:])
		case "knowledge":
			return runReviewKnowledge170(args[1:])
		case "dispatch":
			state, err := reviewDispatchState170(args[1:])
			if err != nil {
				return err
			}
			if reviewDispatchProtocol170(state) == reviewprogress.Protocol170 {
				return runReviewDispatch170(args[1:])
			}
		}
	}
	return runReview160(args)
}

func isReviewProtocol170(state reviewprogress.State) bool {
	return state.ProtocolVersion == reviewprogress.Protocol170
}

func reviewDispatchProtocol170(state reviewprogress.State) string {
	if isReviewProtocol170(state) {
		return reviewprogress.Protocol170
	}
	return "LEGACY"
}

func reviewDispatchState170(args []string) (reviewprogress.State, error) {
	fs := flag.NewFlagSet("review dispatch protocol", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	runID := fs.String("run-id", "", "")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 || strings.TrimSpace(*runID) == "" {
		return reviewprogress.State{}, errors.New("review dispatch requires --run-id")
	}
	state, err := reviewprogress.Read(".", strings.TrimSpace(*runID))
	if err != nil {
		// Legacy review dispatches predate Runtime-owned review progress. Only an
		// absent progress file is allowed to route to legacy behavior. If a
		// progress artifact exists but is malformed/stale, Read returns a
		// different error and 1.7 fails closed instead of downgrading.
		if errors.Is(err, os.ErrNotExist) {
			return reviewprogress.State{}, nil
		}
		return reviewprogress.State{}, err
	}
	return state, nil
}

func runReviewBegin170(args []string) error {
	if len(args) != 0 {
		return errors.New("review begin takes no arguments")
	}
	runsRoot := filepath.Join(".code-harness", "runs")
	if err := os.MkdirAll(runsRoot, 0o755); err != nil {
		return fmt.Errorf("REVIEW_BEGIN_RUNS_DIR_FAILED: %w", err)
	}
	for attempt := 0; attempt < 32; attempt++ {
		entropy := make([]byte, 16)
		if _, err := rand.Read(entropy); err != nil {
			return fmt.Errorf("REVIEW_BEGIN_RANDOM_FAILED: %w", err)
		}
		runID := "review-" + hex.EncodeToString(entropy)
		runPath := filepath.Join(runsRoot, runID)
		if err := os.Mkdir(runPath, 0o755); err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return err
		}
		if _, err := reviewprogress.Begin170(".", runID); err != nil {
			_ = os.RemoveAll(runPath)
			return fmt.Errorf("REVIEW_BEGIN_PROGRESS_FAILED: %w", err)
		}
		progressPath, _ := reviewprogress.Path(runID)
		return writeJSONAndStatus(map[string]any{
			"status": "READY", "runId": runID, "runPath": filepath.ToSlash(runPath),
			"progressPath": filepath.ToSlash(progressPath), "protocolVersion": reviewprogress.Protocol170,
		}, true)
	}
	return errors.New("REVIEW_BEGIN_RUN_ID_EXHAUSTED")
}

func requireReviewDiscovery170(runID string) error {
	ctx, err := loadReviewContextArtifact170(runID, "DISCOVERY")
	if err != nil {
		return fmt.Errorf("REVIEW_CONTEXT_DISCOVERY_REQUIRED: %w", err)
	}
	if ctx.RunID != runID || ctx.Phase != "DISCOVERY" {
		return fmt.Errorf("REVIEW_CONTEXT_DISCOVERY_REQUIRED: context identity mismatch")
	}
	return nil
}

func runReviewDispatch170(args []string) error {
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
	state, err := reviewprogress.Read(".", canonicalRunID)
	if err != nil {
		return err
	}
	if !isReviewProtocol170(state) {
		return fmt.Errorf("REVIEW_PROTOCOL_MISMATCH: run %s is not 1.7", canonicalRunID)
	}
	if err := requireReviewDiscovery170(canonicalRunID); err != nil {
		return err
	}
	if state.Status != reviewprogress.StatusRunning || state.CurrentStage != reviewprogress.StageReviewPlanning {
		return fmt.Errorf("RULE_DISPATCH_PROGRESS_INVALID: current=%s status=%s", state.CurrentStage, state.Status)
	}
	failPlanning := func(err error) error {
		return failReviewProgressStage164(canonicalRunID, reviewprogress.StageReviewPlanning, "REVIEW_PLANNING_FAILED", err)
	}
	units, err := reviewunit.Load(reviewunit.BuildInput{RunID: canonicalRunID, CertifiedRunID: canonicalRunID, RepoRoot: "."})
	if err != nil {
		return failPlanning(fmt.Errorf("RULE_DISPATCH_STALE: %w", err))
	}
	knowledgeManifest, err := readReviewKnowledgeManifest170(".", canonicalRunID)
	if err != nil {
		return failPlanning(err)
	}
	if err := verifyReviewKnowledgeUse170(".", canonicalRunID, knowledgeUnits170(units), knowledgeManifest); err != nil {
		return failPlanning(err)
	}
	rules, catalogSHA, err := reviewrules.LoadCatalog(filepath.Join(".code-harness", "review-rules", "spring-v1.yaml"))
	if err != nil {
		return failPlanning(err)
	}
	manifest, err := reviewrules.BuildDispatch170(units, rules, catalogSHA, knowledgeManifest)
	if err != nil {
		return failPlanning(err)
	}
	encoded, err := reviewrules.CanonicalBytes(manifest)
	if err != nil {
		return failPlanning(err)
	}
	if err := validateReviewContract153("rule-dispatch.schema.json", encoded); err != nil {
		return failPlanning(fmt.Errorf("RULE_DISPATCH_SCHEMA_INVALID: %w", err))
	}
	artifactPath := filepath.Join(".code-harness", "runs", canonicalRunID, "analysis", "rule-dispatch.json")
	if err := atomicReviewWrite153(artifactPath, encoded); err != nil {
		return failPlanning(fmt.Errorf("RULE_DISPATCH_WRITE_FAILED: %w", err))
	}
	// 1.7 planning finalizes only after RULES context succeeds.
	return writeJSONAndStatus(map[string]any{
		"status": "READY", "artifactPath": filepath.ToSlash(artifactPath), "manifest": manifest, "planningFinalized": false,
	}, true)
}
