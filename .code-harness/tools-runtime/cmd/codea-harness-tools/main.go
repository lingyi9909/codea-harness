package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codea-harness-tools/internal/coverage"
	"codea-harness-tools/internal/dbconfig"
	"codea-harness-tools/internal/dbevidence"
	"codea-harness-tools/internal/dbguard"
	"codea-harness-tools/internal/dbmysql"
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
		return errors.New("usage: codea-harness-tools <upgrade|validate|nav|db>")
	}
	switch args[0] {
	case "upgrade":
		return runUpgrade(args[1:])
	case "validate":
		return runValidate(args[1:])
	case "nav":
		return runNav(args[1:])
	case "db":
		return runDB(args[1:])
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
	changeAnalysisPath := fs.String("change-analysis", "", "validated ChangeAnalysis input under .code-harness")
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
			if *changeAnalysisPath == "" {
				return errors.New("test target selection validation requires --change-analysis")
			}
			if !safeHarnessPath(*changeAnalysisPath, "") {
				return errors.New("change analysis path outside .code-harness is not allowed")
			}
			changeAnalysisJSON, err := os.ReadFile(*changeAnalysisPath)
			if err != nil {
				return err
			}
			changeAnalysisSchemaPath := filepath.Join(".code-harness", "contracts", "change-analysis.schema.json")
			changeAnalysisSchema, err := os.ReadFile(changeAnalysisSchemaPath)
			if err != nil {
				return err
			}
			if err := schema.ValidateJSON(changeAnalysisSchema, changeAnalysisJSON); err != nil {
				return err
			}
			if _, err := coverage.VerifyAnalysisJSON(changeAnalysisJSON); err != nil {
				return err
			}
			if err := selection.VerifyAgainstChangeAnalysis(ib, changeAnalysisJSON); err != nil {
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

type dbQueryInput struct {
	RunID   string `json:"runId"`
	QueryID string `json:"queryId"`
	Purpose string `json:"purpose"`
	SQL     string `json:"sql"`
	Params  []any  `json:"params"`
}

func runDB(args []string) error {
	if len(args) == 0 {
		return errors.New("db requires ping, list-tables, describe-table, or query")
	}
	switch args[0] {
	case "ping":
		return runDBPing(args[1:])
	case "list-tables":
		return runDBListTables(args[1:])
	case "describe-table":
		return runDBDescribeTable(args[1:])
	case "query":
		return runDBQuery(args[1:])
	default:
		return fmt.Errorf("unknown db action %q", args[0])
	}
}

func runDBPing(args []string) error {
	fs := flag.NewFlagSet("db ping", flag.ContinueOnError)
	runID := fs.String("run-id", "", "diagnostic run id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || !validDBArtifactID(*runID) {
		return errors.New("db ping requires a valid --run-id")
	}
	cfg, err := loadRuntimeDBConfig()
	if err != nil {
		return err
	}
	client, err := dbmysql.Open(cfg)
	if err != nil {
		return err
	}
	if err := client.Ping(context.Background()); err != nil {
		return err
	}
	return writeJSONAndStatus(map[string]any{"status": "OK", "runId": *runID}, true)
}

func runDBListTables(args []string) error {
	fs := flag.NewFlagSet("db list-tables", flag.ContinueOnError)
	schemaName := fs.String("schema", "", "allowed database schema")
	runID := fs.String("run-id", "", "diagnostic run id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *schemaName == "" || !validDBArtifactID(*runID) {
		return errors.New("db list-tables requires --schema and a valid --run-id")
	}
	cfg, err := loadRuntimeDBConfig()
	if err != nil {
		return err
	}
	client, err := dbmysql.Open(cfg)
	if err != nil {
		return err
	}
	tables, err := client.ListTables(context.Background(), *schemaName)
	if err != nil {
		return err
	}
	return writeJSONAndStatus(map[string]any{"status": "OK", "runId": *runID, "schema": *schemaName, "tables": tables}, true)
}

func runDBDescribeTable(args []string) error {
	fs := flag.NewFlagSet("db describe-table", flag.ContinueOnError)
	schemaName := fs.String("schema", "", "allowed database schema")
	tableName := fs.String("table", "", "database table")
	runID := fs.String("run-id", "", "diagnostic run id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *schemaName == "" || *tableName == "" || !validDBArtifactID(*runID) {
		return errors.New("db describe-table requires --schema, --table, and a valid --run-id")
	}
	cfg, err := loadRuntimeDBConfig()
	if err != nil {
		return err
	}
	client, err := dbmysql.Open(cfg)
	if err != nil {
		return err
	}
	columns, err := client.DescribeTable(context.Background(), *schemaName, *tableName)
	if err != nil {
		return err
	}
	return writeJSONAndStatus(map[string]any{"status": "OK", "runId": *runID, "schema": *schemaName, "table": *tableName, "columns": columns}, true)
}

func runDBQuery(args []string) error {
	fs := flag.NewFlagSet("db query", flag.ContinueOnError)
	inputPath := fs.String("input", "", "structured query request under .code-harness/runs/<runId>/requests")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *inputPath == "" {
		return errors.New("db query requires --input")
	}
	if !safeDBRunsPath(*inputPath) {
		return errors.New("db query input outside .code-harness/runs is not allowed")
	}
	data, err := os.ReadFile(*inputPath)
	if err != nil {
		return fmt.Errorf("read db query request: %w", err)
	}
	var req dbQueryInput
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return fmt.Errorf("decode db query request: %w", err)
	}
	if err := ensureJSONEOF(dec); err != nil {
		return err
	}
	if req.RunID == "" || req.QueryID == "" || strings.TrimSpace(req.Purpose) == "" {
		return errors.New("db query request requires runId, queryId, purpose, sql, and params")
	}
	if strings.TrimSpace(req.SQL) == "" {
		return errors.New("db query request requires sql")
	}
	if !validDBArtifactID(req.RunID) || !validDBArtifactID(req.QueryID) {
		return errors.New("db query request contains invalid runId/queryId")
	}
	if !safeDBQueryRequestPath(*inputPath, req.RunID) {
		return errors.New("db query input must be under .code-harness/runs/<runId>/requests")
	}
	params, err := normalizeDBParams(req.Params)
	if err != nil {
		return err
	}

	cfg, err := loadRuntimeDBConfig()
	if err != nil {
		return err
	}
	if _, err := dbguard.ValidateReadonlyQuery(req.SQL, cfg.Connection.Database, cfg.Safety.AllowedSchemas); err != nil {
		return fmt.Errorf("readonly SQL rejected: %w", err)
	}
	used, err := countDBEvidence(req.RunID)
	if err != nil {
		return err
	}
	if used >= cfg.Safety.MaxQueriesPerDiagnosis {
		return fmt.Errorf("QUERY_BUDGET_EXCEEDED: run %q already has %d database evidence queries; max is %d", req.RunID, used, cfg.Safety.MaxQueriesPerDiagnosis)
	}

	client, err := dbmysql.Open(cfg)
	if err != nil {
		return err
	}
	result, err := client.QueryReadonly(context.Background(), dbmysql.QueryRequest{
		RunID: req.RunID, QueryID: req.QueryID, Purpose: req.Purpose, SQL: req.SQL, Params: params,
	})
	if err != nil {
		return err
	}
	evidencePath, err := dbevidence.WriteEvidence(".", result)
	if err != nil {
		return err
	}
	return writeJSONAndStatus(map[string]any{"status": "OK", "evidencePath": evidencePath, "result": result}, true)
}

func loadRuntimeDBConfig() (dbconfig.Config, error) {
	cfg, err := dbconfig.Load(
		filepath.Join(".code-harness", "database.yaml"),
		filepath.Join(".code-harness", "contracts", "database-config.schema.json"),
	)
	if err != nil {
		return dbconfig.Config{}, err
	}
	if !cfg.Enabled {
		return dbconfig.Config{}, errors.New("DATABASE_EVIDENCE_UNAVAILABLE: database.yaml is missing or database evidence is disabled")
	}
	return cfg, nil
}

func safeDBRunsPath(path string) bool {
	if filepath.IsAbs(path) {
		return false
	}
	base := filepath.Clean(filepath.Join(".code-harness", "runs"))
	clean := filepath.Clean(path)
	rel, err := filepath.Rel(base, clean)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func safeDBQueryRequestPath(path, runID string) bool {
	if !safeDBRunsPath(path) || !strings.EqualFold(filepath.Ext(path), ".json") {
		return false
	}
	base := filepath.Clean(filepath.Join(".code-harness", "runs", runID, "requests"))
	clean := filepath.Clean(path)
	rel, err := filepath.Rel(base, clean)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func validDBArtifactID(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	for i := 0; i < len(value); i++ {
		b := value[i]
		if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '.' || b == '_' || b == '-' {
			continue
		}
		return false
	}
	return true
}

func countDBEvidence(runID string) (int, error) {
	dir := filepath.Join(".code-harness", "runs", runID, "evidence", "db")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("count database evidence: %w", err)
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			count++
		}
	}
	return count, nil
}

func normalizeDBParams(params []any) ([]any, error) {
	out := make([]any, len(params))
	for i, value := range params {
		switch v := value.(type) {
		case nil, string, bool:
			out[i] = v
		case json.Number:
			if integer, err := v.Int64(); err == nil {
				out[i] = integer
				continue
			}
			floating, err := v.Float64()
			if err != nil {
				return nil, fmt.Errorf("invalid numeric query param at index %d", i)
			}
			out[i] = floating
		default:
			return nil, fmt.Errorf("unsupported query param at index %d; only null, string, boolean, and number are allowed", i)
		}
	}
	return out, nil
}

func ensureJSONEOF(dec *json.Decoder) error {
	var extra any
	if err := dec.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return fmt.Errorf("decode db query request: %w", err)
	}
	return errors.New("decode db query request: multiple JSON values are not allowed")
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
