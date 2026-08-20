package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codea-harness-tools/internal/coverage"
	"codea-harness-tools/internal/nav"
	"codea-harness-tools/internal/schema"
	"codea-harness-tools/internal/selection"
	"codea-harness-tools/internal/upgrade"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: codea-harness-tools <upgrade|validate|nav>")
	}
	switch args[0] {
	case "upgrade":
		return runUpgrade(args[1:])
	case "validate":
		return runValidate(args[1:])
	case "nav":
		return runNav(args[1:])
	default:
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}
func runUpgrade(args []string) error {
	if len(args) != 0 {
		return errors.New("upgrade takes no arguments")
	}
	r := upgrade.Run(upgrade.Options{SourceDir: ".code-harness-upgrade", TargetDir: ".code-harness", Refs: upgrade.GitRefs{}})
	return writeJSONAndStatus(r, r.Status == upgrade.StatusUpgraded || r.Status == upgrade.StatusAlreadyUpToDate)
}
func runValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	schemaPath := fs.String("schema", "", "schema under .code-harness/contracts")
	input := fs.String("input", "", "input under .code-harness")
	format := fs.String("format", "auto", "auto|yaml|json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *schemaPath == "" || *input == "" {
		return errors.New("validate requires --schema and --input")
	}
	if !safeHarnessPath(*schemaPath, "contracts") || !safeHarnessPath(*input, "") {
		return errors.New("validate path outside .code-harness is not allowed")
	}
	sb, err := os.ReadFile(*schemaPath)
	if err != nil {
		return err
	}
	ib, err := os.ReadFile(*input)
	if err != nil {
		return err
	}
	f := strings.ToLower(*format)
	if f == "auto" {
		if strings.EqualFold(filepath.Ext(*input), ".json") {
			f = "json"
		} else {
			f = "yaml"
		}
	}
	switch f {
	case "yaml", "yml":
		if err := schema.ValidateYAML(sb, ib); err != nil {
			return err
		}
		fmt.Println(`{"status":"VALID","format":"yaml"}`)
	case "json":
		if err := schema.ValidateJSON(sb, ib); err != nil {
			return err
		}
		out := map[string]any{"status": "VALID", "format": "json"}
		if filepath.Base(*schemaPath) == "change-analysis.schema.json" {
			machine, err := coverage.VerifyAnalysisJSON(ib)
			if err != nil {
				return err
			}
			out["reviewCoverage"] = machine
		}
		if filepath.Base(*schemaPath) == "test-target-selection.schema.json" {
			if err := selection.VerifyJSON(ib); err != nil {
				return err
			}
			out["testTargetSelection"] = "VERIFIED"
		}
		return writeJSONAndStatus(out, true)
	default:
		return fmt.Errorf("unsupported validate format %q", *format)
	}
	return nil
}
func safeHarnessPath(p, requiredChild string) bool {
	clean := filepath.Clean(p)
	root := filepath.Clean(".code-harness")
	rel, err := filepath.Rel(root, clean)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	if requiredChild != "" {
		return rel == requiredChild || strings.HasPrefix(rel, requiredChild+string(filepath.Separator))
	}
	return true
}
func runNav(args []string) error {
	if len(args) == 0 {
		return errors.New("nav requires find-symbol, find-references, or find-implementations")
	}
	action := args[0]
	fs := flag.NewFlagSet("nav", flag.ContinueOnError)
	symbol := fs.String("symbol", "", "Java symbol")
	scope := fs.String("scope", "src/main/java", "repository-relative scope")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *symbol == "" {
		return errors.New("nav requires --symbol")
	}
	n := nav.Navigator{RepoRoot: ".", AstGrepPath: filepath.Join(".code-harness", "bin", "ast-grep.exe")}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var r nav.Result
	var err error
	switch action {
	case "find-symbol":
		r, err = n.FindSymbol(ctx, *symbol, *scope)
	case "find-references":
		r, err = n.FindReferences(ctx, *symbol, *scope)
	case "find-implementations":
		r, err = n.FindImplementations(ctx, *symbol, *scope)
	default:
		return fmt.Errorf("unknown nav action %q", action)
	}
	if err != nil {
		return err
	}
	return writeJSONAndStatus(r, true)
}
func writeJSONAndStatus(v any, success bool) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	if !success {
		return errors.New("operation did not complete successfully")
	}
	return nil
}
