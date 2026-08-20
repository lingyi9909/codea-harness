package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func withTempProject(t *testing.T) string {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	return root
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func installDatabaseSchema(t *testing.T) {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	source := filepath.Join(filepath.Dir(testFile), "..", "..", "..", "contracts", "database-config.schema.json")
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(".code-harness", "contracts", "database-config.schema.json"), string(data))
}

func validDatabaseYAML(maxQueries int) string {
	return "version: 1\n" +
		"enabled: true\n" +
		"environment: TEST\n" +
		"dialect: mysql\n" +
		"connection:\n" +
		"  host: 127.0.0.1\n" +
		"  port: 3306\n" +
		"  database: order_test\n" +
		"  username: reader\n" +
		"  password: secret\n" +
		"  charset: utf8mb4\n" +
		"safety:\n" +
		"  allowedSchemas: [order_test]\n" +
		"  maxRows: 100\n" +
		"  timeoutSeconds: 1\n" +
		"  maxQueriesPerDiagnosis: " + strconv.Itoa(maxQueries) + "\n" +
		"  allowSchemaDiscovery: true\n" +
		"  allowReadonlySql: true\n"
}

func writeQueryRequest(t *testing.T, runID, name, body string) string {
	t.Helper()
	path := filepath.Join(".code-harness", "runs", runID, "requests", name)
	writeFile(t, path, body)
	return path
}

func TestDBUnknownActionReturnsNonzero(t *testing.T) {
	err := run([]string{"db", "unknown"})
	if err == nil || !strings.Contains(err.Error(), "unknown db action") {
		t.Fatalf("err=%v", err)
	}
}

func TestDBQueryRejectsInputOutsideRuns(t *testing.T) {
	withTempProject(t)
	writeFile(t, "request.json", `{}`)
	err := run([]string{"db", "query", "--input", "request.json"})
	if err == nil || !strings.Contains(err.Error(), "outside .code-harness/runs") {
		t.Fatalf("err=%v", err)
	}
}

func TestDBQueryRejectsMissingRequiredRequestFields(t *testing.T) {
	withTempProject(t)
	path := writeQueryRequest(t, "run-001", "request.json", `{"runId":"run-001","sql":"SELECT 1"}`)
	err := run([]string{"db", "query", "--input", path})
	if err == nil || !strings.Contains(err.Error(), "runId, queryId, purpose") {
		t.Fatalf("err=%v", err)
	}
}

func TestDBMissingConfigReturnsControlledUnavailable(t *testing.T) {
	withTempProject(t)
	path := writeQueryRequest(t, "run-001", "request.json", `{"runId":"run-001","queryId":"dbq-001","purpose":"verify state","sql":"SELECT 1","params":[]}`)
	err := run([]string{"db", "query", "--input", path})
	if err == nil || !strings.Contains(err.Error(), "DATABASE_EVIDENCE_UNAVAILABLE") {
		t.Fatalf("err=%v", err)
	}
}

func TestDBProductionConfigRejectedBeforeDatabaseUse(t *testing.T) {
	withTempProject(t)
	installDatabaseSchema(t)
	writeFile(t, filepath.Join(".code-harness", "database.yaml"), strings.Replace(validDatabaseYAML(3), "environment: TEST", "environment: PRODUCTION", 1))
	err := run([]string{"db", "ping", "--run-id", "run-001"})
	if err == nil || !strings.Contains(err.Error(), "database config validation failed") {
		t.Fatalf("err=%v", err)
	}
}

func TestDBWriteSQLRejectedBeforeConnect(t *testing.T) {
	withTempProject(t)
	installDatabaseSchema(t)
	writeFile(t, filepath.Join(".code-harness", "database.yaml"), validDatabaseYAML(3))
	path := writeQueryRequest(t, "run-001", "request.json", `{"runId":"run-001","queryId":"dbq-001","purpose":"unsafe","sql":"UPDATE order_info SET status = ?","params":["X"]}`)
	err := run([]string{"db", "query", "--input", path})
	if err == nil || !strings.Contains(err.Error(), "readonly SQL rejected") {
		t.Fatalf("err=%v", err)
	}
}

func TestDBQueryBudgetExceededBeforeConnect(t *testing.T) {
	withTempProject(t)
	installDatabaseSchema(t)
	writeFile(t, filepath.Join(".code-harness", "database.yaml"), validDatabaseYAML(2))
	writeFile(t, filepath.Join(".code-harness", "runs", "run-001", "evidence", "db", "dbq-001.json"), `{}`)
	writeFile(t, filepath.Join(".code-harness", "runs", "run-001", "evidence", "db", "dbq-002.json"), `{}`)
	path := writeQueryRequest(t, "run-001", "request.json", `{"runId":"run-001","queryId":"dbq-003","purpose":"verify state","sql":"SELECT id FROM order_info","params":[]}`)
	err := run([]string{"db", "query", "--input", path})
	if err == nil || !strings.Contains(err.Error(), "QUERY_BUDGET_EXCEEDED") {
		t.Fatalf("err=%v", err)
	}
}

func TestDBQueryDoesNotAcceptRawSQLArgument(t *testing.T) {
	err := run([]string{"db", "query", "--sql", "SELECT 1"})
	if err == nil {
		t.Fatal("expected raw SQL CLI argument rejection")
	}
}
