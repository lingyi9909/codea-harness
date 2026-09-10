package main

import (
	"errors"
	"flag"
	"strings"

	"codea-harness-tools/internal/reviewprogress"
)

func runReviewProgress164(args []string) error {
	fs := flag.NewFlagSet("review progress", flag.ContinueOnError)
	runID := fs.String("run-id", "", "review run id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || strings.TrimSpace(*runID) == "" || *runID != strings.TrimSpace(*runID) {
		return errors.New("review progress requires canonical --run-id")
	}
	state, err := reviewprogress.Read(".", *runID)
	if err != nil {
		return err
	}
	return writeJSONAndStatus(state, true)
}
