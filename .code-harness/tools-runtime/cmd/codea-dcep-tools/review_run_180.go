package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"codea-harness-tools/internal/requestjson"
	"codea-harness-tools/internal/reviewrun"
)

func runReviewStart180(args []string) error {
	if len(args) != 0 {
		return errors.New("review start takes no arguments")
	}
	out, err := reviewrun.Start(".")
	if err != nil {
		return err
	}
	return writeJSONAndStatus(out, true)
}

func runReviewPrepare180(args []string) error {
	fs := flag.NewFlagSet("review prepare", flag.ContinueOnError)
	runID := fs.String("run-id", "", "1.8 review run id")
	mode := fs.String("mode", "", "CHANGES|CURRENT_IMPLEMENTATION")
	target := fs.String("target", "", "optional review target")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || strings.TrimSpace(*runID) == "" || strings.TrimSpace(*mode) == "" {
		return errors.New("review prepare requires --run-id and --mode")
	}
	out, err := reviewrun.Prepare(context.Background(), ".", *runID, reviewrun.Intent{Mode: *mode, Target: *target})
	if err != nil {
		return err
	}
	return writeJSONAndStatus(out, true)
}

func runReviewSelect180(args []string) error {
	fs := flag.NewFlagSet("review select", flag.ContinueOnError)
	runID := fs.String("run-id", "", "1.8 review run id")
	optionsHash := fs.String("options-hash", "", "hash returned by review prepare")
	ids := fs.String("ids", "", "comma-separated chain ids, for example C1,C2")
	sessionID := fs.String("session-id", "", "opaque Host session id; T3 tool supplies this from Host context")
	messageID := fs.String("message-id", "", "opaque next-user message id; T3 tool supplies this from Host context")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || strings.TrimSpace(*runID) == "" || strings.TrimSpace(*optionsHash) == "" || strings.TrimSpace(*ids) == "" || strings.TrimSpace(*sessionID) == "" || strings.TrimSpace(*messageID) == "" {
		return errors.New("review select requires --run-id --options-hash --ids --session-id and --message-id")
	}
	selectionIDs := make([]string, 0)
	for _, id := range strings.Split(*ids, ",") {
		selectionIDs = append(selectionIDs, strings.TrimSpace(id))
	}
	out, err := reviewrun.Select(context.Background(), ".", reviewrun.SelectionRequest{RunID: *runID, OptionsHash: *optionsHash, IDs: selectionIDs}, reviewrun.HostTurn{SessionID: *sessionID, MessageID: *messageID})
	if err != nil {
		return err
	}
	return writeJSONAndStatus(out, true)
}

func runReviewStatus180(args []string) error {
	fs := flag.NewFlagSet("review status", flag.ContinueOnError)
	runID := fs.String("run-id", "", "1.8 review run id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || strings.TrimSpace(*runID) == "" {
		return errors.New("review status requires --run-id")
	}
	out, err := reviewrun.Status(".", *runID)
	if err != nil {
		return err
	}
	return writeJSONAndStatus(out, true)
}

func runReviewCancel180(args []string) error {
	fs := flag.NewFlagSet("review cancel", flag.ContinueOnError)
	runID := fs.String("run-id", "", "1.8 review run id")
	reason := fs.String("reason", "", "cancellation reason")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || strings.TrimSpace(*runID) == "" {
		return errors.New("review cancel requires --run-id")
	}
	out, err := reviewrun.Cancel(".", *runID, *reason)
	if err != nil {
		return err
	}
	return writeJSONAndStatus(out, true)
}

func runReviewFinish180(args []string) error {
	fs := flag.NewFlagSet("review finish", flag.ContinueOnError)
	input := fs.String("input", "", "finish request under .code-harness/runs/<runId>/requests/*.json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || strings.TrimSpace(*input) == "" {
		return errors.New("review finish requires --input")
	}
	pathRunID, cleanInput, err := validateAnalysisRequestPath153(*input)
	if err != nil {
		return errors.New("review finish input must be under .code-harness/runs/<runId>/requests")
	}
	data, err := requestjson.ReadFile(cleanInput)
	if err != nil {
		return fmt.Errorf("REVIEW_FINISH_REQUEST_READ_FAILED: %w", err)
	}
	var req reviewrun.FinishRequest
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return fmt.Errorf("REVIEW_FINISH_REQUEST_INVALID: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("REVIEW_FINISH_REQUEST_INVALID: multiple JSON values are not allowed")
		}
		return fmt.Errorf("REVIEW_FINISH_REQUEST_INVALID: %w", err)
	}
	if req.RunID != pathRunID {
		return fmt.Errorf("REVIEW_FINISH_RUN_ID_MISMATCH: body runId %q path runId %q", req.RunID, pathRunID)
	}
	out, err := reviewrun.Finish(context.Background(), ".", req)
	if err != nil {
		return err
	}
	return writeJSONAndStatus(out, true)
}
