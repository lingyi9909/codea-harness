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
		return errors.New("report requires review or api-doc")
	}
	switch args[0] {
	case "review":
		return runReviewReport(args[1:])
	case "api-doc":
		return runApiDocReport(args[1:])
	default:
		return fmt.Errorf("unknown report action %q", args[0])
	}
}

func runReviewReport(args []string) error {
	fs := flag.NewFlagSet("report review", flag.ContinueOnError)
	input := fs.String("input", "", "structured review report request under .code-harness/runs/<runId>/requests")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *input == "" {
		return errors.New("report review requires --input")
	}
	path, err := report.WriteRequestFile(".", *input)
	if err != nil {
		return err
	}
	return reportPathJSON(path)
}

func runApiDocReport(args []string) error {
	fs := flag.NewFlagSet("report api-doc", flag.ContinueOnError)
	input := fs.String("input", "", "structured api-doc report request under .code-harness/runs/<runId>/requests")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *input == "" {
		return errors.New("report api-doc requires --input")
	}
	path, err := report.WriteApiDocRequestFile(".", *input)
	if err != nil {
		return err
	}
	return reportPathJSON(path)
}

func reportPathJSON(path string) error {
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
