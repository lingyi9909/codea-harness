package dbmysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"codea-harness-tools/internal/dbconfig"
	"github.com/DATA-DOG/go-sqlmock"
)

func testConfig() dbconfig.Config {
	return dbconfig.Config{
		Version: 1, Enabled: true, Environment: dbconfig.EnvironmentTest, Dialect: dbconfig.DialectMySQL,
		Connection: dbconfig.Connection{Host: "127.0.0.1", Port: 3306, Database: "order_test", Username: "reader", Password: "top-secret-password", Charset: "utf8mb4"},
		Safety:     dbconfig.Safety{AllowedSchemas: []string{"order_test"}, MaxRows: 100, TimeoutSeconds: 1, MaxQueriesPerDiagnosis: 10, AllowSchemaDiscovery: true, AllowReadonlySQL: true},
	}
}

func mockClient(t *testing.T, cfg dbconfig.Config) (*Client, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return newClient(db, cfg), mock
}

type recordingConnector struct{ conn *recordingConn }

func (c recordingConnector) Connect(context.Context) (driver.Conn, error) { return c.conn, nil }
func (c recordingConnector) Driver() driver.Driver                        { return recordingDriver{} }

type recordingDriver struct{}

func (recordingDriver) Open(string) (driver.Conn, error) { return nil, errors.New("not used") }

type recordingConn struct {
	beginCalled bool
	beginOpts   driver.TxOptions
}

func (c *recordingConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not supported") }
func (c *recordingConn) Close() error                        { return nil }
func (c *recordingConn) Begin() (driver.Tx, error)           { return &recordingTx{}, nil }
func (c *recordingConn) BeginTx(_ context.Context, opts driver.TxOptions) (driver.Tx, error) {
	c.beginCalled = true
	c.beginOpts = opts
	return &recordingTx{}, nil
}
func (c *recordingConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return &recordingRows{}, nil
}

type recordingTx struct{}

func (*recordingTx) Commit() error   { return nil }
func (*recordingTx) Rollback() error { return nil }

type recordingRows struct{ emitted bool }

func (*recordingRows) Columns() []string { return []string{"id"} }
func (*recordingRows) Close() error      { return nil }
func (r *recordingRows) Next(dest []driver.Value) error {
	if r.emitted {
		return io.EOF
	}
	r.emitted = true
	dest[0] = int64(1)
	return nil
}

func TestPingSuccessAndFailure(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		c, mock := mockClient(t, testConfig())
		mock.ExpectPing()
		if err := c.Ping(context.Background()); err != nil {
			t.Fatalf("Ping: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("failure redacts password", func(t *testing.T) {
		cfg := testConfig()
		c, mock := mockClient(t, cfg)
		mock.ExpectPing().WillReturnError(fmt.Errorf("server error contains %s", cfg.Connection.Password))
		err := c.Ping(context.Background())
		if err == nil {
			t.Fatal("expected error")
		}
		if strings.Contains(err.Error(), cfg.Connection.Password) {
			t.Fatalf("password leaked in error: %v", err)
		}
	})
}

func TestListTablesAndDescribeTableEnforceDiscoveryAllowlist(t *testing.T) {
	cfg := testConfig()
	c, mock := mockClient(t, cfg)
	mock.ExpectQuery("information_schema.TABLES").WithArgs("order_test").
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME"}).AddRow("audit_log").AddRow("order_info"))
	tables, err := c.ListTables(context.Background(), "order_test")
	if err != nil {
		t.Fatalf("ListTables: %v", err)
	}
	if !reflect.DeepEqual(tables, []string{"audit_log", "order_info"}) {
		t.Fatalf("tables=%v", tables)
	}

	mock.ExpectQuery("information_schema.COLUMNS").WithArgs("order_test", "order_info").
		WillReturnRows(sqlmock.NewRows([]string{"COLUMN_NAME", "DATA_TYPE", "IS_NULLABLE", "COLUMN_KEY"}).
			AddRow("id", "bigint", "NO", "PRI").AddRow("status", "varchar", "YES", ""))
	cols, err := c.DescribeTable(context.Background(), "order_test", "order_info")
	if err != nil {
		t.Fatalf("DescribeTable: %v", err)
	}
	if len(cols) != 2 || cols[0].Name != "id" || cols[0].Nullable || !cols[1].Nullable {
		t.Fatalf("columns=%+v", cols)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}

	if _, err := c.ListTables(context.Background(), "other_db"); err == nil {
		t.Fatal("expected cross-schema discovery rejection")
	}
	if _, err := c.DescribeTable(context.Background(), "order_test", "bad-table"); err == nil {
		t.Fatal("expected invalid identifier rejection")
	}
	if _, err := c.ListTables(context.Background(), "订单库"); err == nil {
		t.Fatal("expected non-ASCII identifier rejection")
	}

	cfg.Safety.AllowSchemaDiscovery = false
	c2, _ := mockClient(t, cfg)
	if _, err := c2.ListTables(context.Background(), "order_test"); err == nil {
		t.Fatal("expected discovery disabled rejection")
	}
}

func TestQueryReadonlyPassesParamsUsesReadOnlyTransactionAndRedacts(t *testing.T) {
	c, mock := mockClient(t, testConfig())
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT status, access_token FROM order_info WHERE id = \?`).WithArgs(int64(10001)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "access_token"}).AddRow("PENDING", "token-value"))
	mock.ExpectRollback()
	got, err := c.QueryReadonly(context.Background(), QueryRequest{
		RunID: "run-001", QueryID: "dbq-001", Purpose: "verify order state",
		SQL: "SELECT status, access_token FROM order_info WHERE id = ?", Params: []any{int64(10001)},
	})
	if err != nil {
		t.Fatalf("QueryReadonly: %v", err)
	}
	if got.StatementType != "SELECT" || got.Schema != "order_test" || got.RowCount != 1 || got.Truncated {
		t.Fatalf("result=%+v", got)
	}
	if got.Rows[0]["access_token"] != "***REDACTED***" {
		t.Fatalf("sensitive value not redacted: %+v", got.Rows[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestQueryReadonlyRequestsReadOnlyTransaction(t *testing.T) {
	conn := &recordingConn{}
	db := sql.OpenDB(recordingConnector{conn: conn})
	t.Cleanup(func() { _ = db.Close() })
	c := newClient(db, testConfig())
	_, err := c.QueryReadonly(context.Background(), QueryRequest{
		RunID: "run-001", QueryID: "dbq-readonly", Purpose: "verify readonly tx", SQL: "SELECT id FROM order_info",
	})
	if err != nil {
		t.Fatalf("QueryReadonly: %v", err)
	}
	if !conn.beginCalled || !conn.beginOpts.ReadOnly {
		t.Fatalf("BeginTx options=%+v called=%v; want ReadOnly=true", conn.beginOpts, conn.beginCalled)
	}
}

func TestQueryReadonlyCallsGuardBeforeDatabase(t *testing.T) {
	c, mock := mockClient(t, testConfig())
	_, err := c.QueryReadonly(context.Background(), QueryRequest{RunID: "run-001", QueryID: "dbq-002", Purpose: "unsafe", SQL: "UPDATE order_info SET status='X'"})
	if err == nil {
		t.Fatal("expected unsafe SQL rejection")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("database touched before guard: %v", err)
	}
}

func TestQueryReadonlyHonorsContextCancellation(t *testing.T) {
	c, mock := mockClient(t, testConfig())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.QueryReadonly(ctx, QueryRequest{RunID: "run-001", QueryID: "dbq-003", Purpose: "cancelled", SQL: "SELECT id FROM order_info"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v want context.Canceled", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestQueryReadonlyHonorsContextDeadline(t *testing.T) {
	c, mock := mockClient(t, testConfig())
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM order_info").WillDelayFor(100 * time.Millisecond).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectRollback()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := c.QueryReadonly(ctx, QueryRequest{RunID: "run-001", QueryID: "dbq-timeout", Purpose: "timeout", SQL: "SELECT id FROM order_info"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v want context.DeadlineExceeded", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestQueryReadonlyCapsRowsAndMarksTruncated(t *testing.T) {
	cfg := testConfig()
	cfg.Safety.MaxRows = 100
	c, mock := mockClient(t, cfg)
	rows := sqlmock.NewRows([]string{"id"})
	for i := 0; i < 500; i++ {
		rows.AddRow(i)
	}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM order_info").WillReturnRows(rows)
	mock.ExpectRollback()
	got, err := c.QueryReadonly(context.Background(), QueryRequest{RunID: "run-001", QueryID: "dbq-004", Purpose: "row cap", SQL: "SELECT id FROM order_info"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Rows) != 100 || got.RowCount != 100 || !got.Truncated {
		t.Fatalf("rows=%d count=%d truncated=%v", len(got.Rows), got.RowCount, got.Truncated)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenRejectsDisabledConfigWithoutPasswordLeak(t *testing.T) {
	cfg := testConfig()
	cfg.Enabled = false
	_, err := Open(cfg)
	if err == nil {
		t.Fatal("expected disabled config rejection")
	}
	if strings.Contains(err.Error(), cfg.Connection.Password) {
		t.Fatalf("password leaked: %v", err)
	}
}
