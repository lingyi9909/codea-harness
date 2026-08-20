package dbguard

import (
	"reflect"
	"testing"
)

func TestValidateReadonlyQueryAllowsReadOnlySQL(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		wantSchemas []string
		wantTables  []string
	}{
		{"simple select", "SELECT id,status FROM order_info WHERE id = ?", []string{"order_test"}, []string{"order_info"}},
		{"join", "SELECT o.id,a.action FROM order_info o LEFT JOIN audit_log a ON a.order_id=o.id WHERE o.id=?", []string{"order_test"}, []string{"audit_log", "order_info"}},
		{"cte select", "WITH recent AS (SELECT id FROM order_info WHERE id=?) SELECT * FROM recent", []string{"order_test"}, []string{"order_info"}},
		{"subquery select", "SELECT * FROM order_info WHERE id IN (SELECT order_id FROM audit_log)", []string{"order_test"}, []string{"audit_log", "order_info"}},
		{"qualified allowed schema", "SELECT * FROM order_test.order_info WHERE id=?", []string{"order_test"}, []string{"order_info"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := ValidateReadonlyQuery(tt.sql, "order_test", []string{"order_test"})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if info.StatementType != "SELECT" {
				t.Fatalf("StatementType=%q", info.StatementType)
			}
			if !reflect.DeepEqual(info.Schemas, tt.wantSchemas) {
				t.Fatalf("Schemas=%v want %v", info.Schemas, tt.wantSchemas)
			}
			if !reflect.DeepEqual(info.Tables, tt.wantTables) {
				t.Fatalf("Tables=%v want %v", info.Tables, tt.wantTables)
			}
		})
	}
}

func TestValidateReadonlyQueryRejectsUnsafeSQL(t *testing.T) {
	tests := []struct{ name, sql string }{
		{"update", "UPDATE order_info SET status='X'"},
		{"delete", "DELETE FROM order_info"},
		{"insert", "INSERT INTO order_info(id) VALUES(1)"},
		{"drop", "DROP TABLE order_info"},
		{"truncate", "TRUNCATE TABLE order_info"},
		{"for update", "SELECT * FROM order_info FOR UPDATE"},
		{"outfile", "SELECT * FROM order_info INTO OUTFILE 'x'"},
		{"dumpfile", "SELECT * FROM order_info INTO DUMPFILE 'x'"},
		{"multi statement", "SELECT 1; DELETE FROM order_info"},
		{"cross schema", "SELECT * FROM other_db.user"},
		{"call", "CALL dangerous_proc()"},
		{"set", "SET autocommit=0"},
		{"cte delete", "WITH doomed AS (SELECT id FROM order_info) DELETE FROM order_info WHERE id IN (SELECT id FROM doomed)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ValidateReadonlyQuery(tt.sql, "order_test", []string{"order_test"}); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestValidateReadonlyQueryRejectsMissingDefaultSchemaForUnqualifiedTable(t *testing.T) {
	if _, err := ValidateReadonlyQuery("SELECT * FROM order_info", "", []string{"order_test"}); err == nil {
		t.Fatal("expected rejection without default schema")
	}
}

func TestValidateReadonlyQueryCTEScopeDoesNotHideOuterPhysicalTable(t *testing.T) {
	sql := `SELECT *
FROM order_info
WHERE EXISTS (
    WITH order_info AS (SELECT 1)
    SELECT 1 FROM order_info
)`
	info, err := ValidateReadonlyQuery(sql, "order_test", []string{"order_test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(info.Schemas, []string{"order_test"}) {
		t.Fatalf("Schemas=%v want [order_test]", info.Schemas)
	}
	if !reflect.DeepEqual(info.Tables, []string{"order_info"}) {
		t.Fatalf("Tables=%v want [order_info]", info.Tables)
	}
}

func TestValidateReadonlyQueryRejectsNestedCTEShadowSchemaBypass(t *testing.T) {
	sql := `SELECT *
FROM order_info
WHERE EXISTS (
    WITH order_info AS (SELECT id FROM order_test.audit_log)
    SELECT id FROM order_info
)`
	if _, err := ValidateReadonlyQuery(sql, "other_db", []string{"order_test"}); err == nil {
		t.Fatal("expected outer physical table to be rejected because default schema is not allowed")
	}
}

func TestValidateReadonlyQueryLegalCTEStillPasses(t *testing.T) {
	info, err := ValidateReadonlyQuery(
		"WITH recent AS (SELECT id FROM order_info WHERE id=?) SELECT * FROM recent",
		"order_test",
		[]string{"order_test"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(info.Tables, []string{"order_info"}) {
		t.Fatalf("Tables=%v want [order_info]", info.Tables)
	}
}

func TestValidateReadonlyQueryCTEBodyPhysicalTableStillCollected(t *testing.T) {
	info, err := ValidateReadonlyQuery(
		"WITH recent AS (SELECT id FROM audit_log) SELECT * FROM recent",
		"order_test",
		[]string{"order_test"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(info.Schemas, []string{"order_test"}) {
		t.Fatalf("Schemas=%v want [order_test]", info.Schemas)
	}
	if !reflect.DeepEqual(info.Tables, []string{"audit_log"}) {
		t.Fatalf("Tables=%v want [audit_log]", info.Tables)
	}
}
