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

	dir := filepath.Join(root, ".code-harness", "runs", clean.RunID, "evidence", "db")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create database evidence directory: %w", err)
	}
	path := filepath.Join(dir, clean.QueryID+".json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("write database evidence: %w", err)
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
	if value == "" {
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
