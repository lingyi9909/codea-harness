package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"codea-harness-tools/internal/report"
)

func runReport(args []string) error {
	if len(args) == 0 {
		return errors.New("report requires review")
	}
	if args[0] != "review" {
		return fmt.Errorf("unknown report action %q", args[0])
	}
	fs := flag.NewFlagSet("report review", flag.ContinueOnError)
	input := fs.String("input", "", "structured review report request under .code-harness/runs/<runId>/requests")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 || *input == "" {
		return errors.New("report review requires --input")
	}
	path, err := report.WriteRequestFile(".", *input)
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(cwd, path)
	if err != nil {
		return err
	}
	return writeJSONAndStatus(map[string]any{"status": "OK", "reportPath": filepath.ToSlash(rel)}, true)
}
