package dbevidence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/dbmysql"
)

func installSchema(t *testing.T, root string) {
	t.Helper()
	contractDir := filepath.Join(root, ".code-harness", "contracts")
	if err := os.MkdirAll(contractDir, 0o755); err != nil {
		t.Fatal(err)
	}
	schemaBytes, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "database-evidence.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(contractDir, "database-evidence.schema.json"), schemaBytes, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWriteEvidenceSanitizesValidatesAndWritesDeterministically(t *testing.T) {
	root := t.TempDir()
	installSchema(t, root)
	result := dbmysql.QueryResult{
		QueryID: "dbq-001", RunID: "run-001", Purpose: "verify state", Schema: "order_test", StatementType: "SELECT",
		Columns: []string{"status", "private_key"},
		Rows:    []map[string]any{{"status": "PENDING", "private_key": "top-secret-password"}}, RowCount: 1, DurationMs: 12,
	}
	path, err := WriteEvidence(root, result)
	if err != nil {
		t.Fatalf("WriteEvidence: %v", err)
	}
	want := filepath.Join(root, ".code-harness", "runs", "run-001", "evidence", "db", "dbq-001.json")
	if path != want {
		t.Fatalf("path=%q want %q", path, want)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "top-secret-password") {
		t.Fatalf("sensitive value leaked: %s", data)
	}
	var got dbmysql.QueryResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Rows[0]["private_key"] != "***REDACTED***" {
		t.Fatalf("row=%v", got.Rows[0])
	}
}

func TestWriteEvidenceRejectsInvalidArtifactAndPathTraversal(t *testing.T) {
	root := t.TempDir()
	installSchema(t, root)
	_, err := WriteEvidence(root, dbmysql.QueryResult{
		QueryID: "../escape", RunID: "run-001", Purpose: "x", Schema: "order_test", StatementType: "SELECT",
		Columns: []string{}, Rows: []map[string]any{}, RowCount: 0,
	})
	if err == nil {
		t.Fatal("expected invalid queryId rejection")
	}
}
