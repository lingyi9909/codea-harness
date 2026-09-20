package dbevidence

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"codea-harness-tools/internal/dbmysql"
	"codea-harness-tools/internal/schema"
)

const redactedValue = "***REDACTED***"

func WriteEvidence(root string, result dbmysql.QueryResult) (string, error) {
	if !safeArtifactID(result.RunID) || !safeArtifactID(result.QueryID) {
		return "", errors.New("database evidence runId/queryId contains invalid characters")
	}

	clean := sanitizeResult(result)
	if err := validateSemantics(clean); err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(clean, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode database evidence: %w", err)
	}
	data = append(data, '\n')

	schemaPath := filepath.Join(root, ".code-harness", "contracts", "database-evidence.schema.json")
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		return "", fmt.Errorf("read database evidence schema: %w", err)
	}
	if err := schema.ValidateJSON(schemaBytes, data); err != nil {
		return "", fmt.Errorf("validate database evidence: %w", err)
	}

	runsRoot := filepath.Join(root, ".code-harness", "runs")
	dir := filepath.Join(runsRoot, clean.RunID, "evidence", "db")
	if err := ensureContained(runsRoot, dir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create database evidence directory: %w", err)
	}
	path := filepath.Join(dir, clean.QueryID+".json")
	if err := ensureContained(runsRoot, path); err != nil {
		return "", err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return "", fmt.Errorf("database evidence %q already exists", clean.QueryID)
	}
	if err != nil {
		return "", fmt.Errorf("create database evidence: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("write database evidence: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("close database evidence: %w", err)
	}
	return path, nil
}

func sanitizeResult(result dbmysql.QueryResult) dbmysql.QueryResult {
	clean := result
	clean.Columns = append([]string(nil), result.Columns...)
	clean.Rows = make([]map[string]any, 0, len(result.Rows))
	for _, source := range result.Rows {
		row := make(map[string]any, len(source))
		for column, value := range source {
			if sensitiveColumn(column) {
				row[column] = redactedValue
				continue
			}
			switch v := value.(type) {
			case []byte:
				row[column] = string(v)
			default:
				row[column] = value
			}
		}
		clean.Rows = append(clean.Rows, row)
	}
	return clean
}

func validateSemantics(result dbmysql.QueryResult) error {
	if result.RunID == "" || result.QueryID == "" || strings.TrimSpace(result.Purpose) == "" {
		return errors.New("database evidence runId, queryId, and purpose are required")
	}
	if result.Schema == "" {
		return errors.New("database evidence schema is required")
	}
	if result.StatementType != "SELECT" {
		return errors.New("database evidence statementType must be SELECT")
	}
	if result.RowCount != len(result.Rows) {
		return errors.New("database evidence rowCount must equal rows length")
	}
	if result.DurationMs < 0 {
		return errors.New("database evidence durationMs must not be negative")
	}
	return nil
}

func safeArtifactID(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func ensureContained(base, target string) error {
	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return fmt.Errorf("resolve database evidence runs directory: %w", err)
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("resolve database evidence path: %w", err)
	}
	rel, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil {
		return fmt.Errorf("check database evidence path: %w", err)
	}
	if rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("database evidence path escapes runs directory")
	}
	return nil
}

func sensitiveColumn(column string) bool {
	var b strings.Builder
	for _, r := range strings.ToLower(column) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	name := b.String()
	for _, marker := range []string{"password", "passwd", "token", "secret", "accesskey", "privatekey"} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}
