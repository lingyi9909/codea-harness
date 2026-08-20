package dbmysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
	"unicode"

	"codea-harness-tools/internal/dbconfig"
	"codea-harness-tools/internal/dbguard"
	mysql "github.com/go-sql-driver/mysql"
)

const redactedValue = "***REDACTED***"

type QueryRequest struct {
	RunID   string
	QueryID string
	Purpose string
	SQL     string
	Params  []any
}

type QueryResult struct {
	QueryID       string           `json:"queryId"`
	RunID         string           `json:"runId"`
	Purpose       string           `json:"purpose"`
	Schema        string           `json:"schema"`
	StatementType string           `json:"statementType"`
	Columns       []string         `json:"columns"`
	Rows          []map[string]any `json:"rows"`
	RowCount      int              `json:"rowCount"`
	Truncated     bool             `json:"truncated"`
	DurationMs    int64            `json:"durationMs"`
}

type Column struct {
	Name     string `json:"name"`
	DataType string `json:"dataType"`
	Nullable bool   `json:"nullable"`
	Key      string `json:"key"`
}

type Client struct {
	db  *sql.DB
	cfg dbconfig.Config
}

func Open(cfg dbconfig.Config) (*Client, error) {
	if !cfg.Enabled {
		return nil, errors.New("database capability is disabled")
	}
	if cfg.Version != 1 {
		return nil, errors.New("database config version must be 1")
	}
	if cfg.Environment != dbconfig.EnvironmentTest && cfg.Environment != dbconfig.EnvironmentLocal {
		return nil, errors.New("database environment must be TEST or LOCAL")
	}
	if cfg.Dialect != dbconfig.DialectMySQL {
		return nil, errors.New("database dialect must be mysql")
	}
	if cfg.Connection.Host == "" || cfg.Connection.Port <= 0 || cfg.Connection.Database == "" || cfg.Connection.Username == "" {
		return nil, errors.New("database connection configuration is incomplete")
	}
	if cfg.Safety.TimeoutSeconds < 1 || cfg.Safety.TimeoutSeconds > 30 {
		return nil, errors.New("database timeoutSeconds must be between 1 and 30")
	}
	if cfg.Safety.MaxRows < 1 || cfg.Safety.MaxRows > 1000 {
		return nil, errors.New("database maxRows must be between 1 and 1000")
	}
	if len(cfg.Safety.AllowedSchemas) == 0 {
		return nil, errors.New("database allowedSchemas must not be empty")
	}
	for _, schemaName := range cfg.Safety.AllowedSchemas {
		if !validIdentifier(schemaName) {
			return nil, errors.New("database allowedSchemas contains invalid schema")
		}
	}
	if cfg.Safety.MaxQueriesPerDiagnosis < 1 || cfg.Safety.MaxQueriesPerDiagnosis > 20 {
		return nil, errors.New("database maxQueriesPerDiagnosis must be between 1 and 20")
	}

	mcfg := mysql.NewConfig()
	mcfg.Net = "tcp"
	mcfg.Addr = net.JoinHostPort(cfg.Connection.Host, strconv.Itoa(cfg.Connection.Port))
	mcfg.DBName = cfg.Connection.Database
	mcfg.User = cfg.Connection.Username
	mcfg.Passwd = cfg.Connection.Password
	timeout := time.Duration(cfg.Safety.TimeoutSeconds) * time.Second
	mcfg.Timeout = timeout
	mcfg.ReadTimeout = timeout
	mcfg.WriteTimeout = timeout
	if cfg.Connection.Charset != "" {
		if err := mcfg.Apply(mysql.Charset(cfg.Connection.Charset, "")); err != nil {
			return nil, wrapErr("configure mysql charset", err, cfg.Connection.Password)
		}
	}

	connector, err := mysql.NewConnector(mcfg)
	if err != nil {
		return nil, wrapErr("create mysql connector", err, cfg.Connection.Password)
	}
	return newClient(sql.OpenDB(connector), cfg), nil
}

func newClient(db *sql.DB, cfg dbconfig.Config) *Client {
	return &Client{db: db, cfg: cfg}
}

func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.db == nil {
		return errors.New("database client is not open")
	}
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	if err := c.db.PingContext(ctx); err != nil {
		return wrapContextErr(ctx, "database ping failed", err, c.cfg.Connection.Password)
	}
	return nil
}

func (c *Client) ListTables(ctx context.Context, schemaName string) ([]string, error) {
	if err := c.validateDiscoveryTarget(schemaName, ""); err != nil {
		return nil, err
	}
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	rows, err := c.db.QueryContext(ctx,
		`SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? ORDER BY TABLE_NAME`,
		schemaName,
	)
	if err != nil {
		return nil, wrapContextErr(ctx, "list database tables", err, c.cfg.Connection.Password)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, wrapErr("scan database table", err, c.cfg.Connection.Password)
		}
		out = append(out, name)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapContextErr(ctx, "read database tables", err, c.cfg.Connection.Password)
	}
	return out, nil
}

func (c *Client) DescribeTable(ctx context.Context, schemaName, tableName string) ([]Column, error) {
	if err := c.validateDiscoveryTarget(schemaName, tableName); err != nil {
		return nil, err
	}
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	rows, err := c.db.QueryContext(ctx,
		`SELECT COLUMN_NAME, DATA_TYPE, IS_NULLABLE, COLUMN_KEY
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
ORDER BY ORDINAL_POSITION`,
		schemaName, tableName,
	)
	if err != nil {
		return nil, wrapContextErr(ctx, "describe database table", err, c.cfg.Connection.Password)
	}
	defer rows.Close()

	var out []Column
	for rows.Next() {
		var name, dataType, nullable, key string
		if err := rows.Scan(&name, &dataType, &nullable, &key); err != nil {
			return nil, wrapErr("scan database column", err, c.cfg.Connection.Password)
		}
		out = append(out, Column{Name: name, DataType: dataType, Nullable: nullable == "YES", Key: key})
	}
	if err := rows.Err(); err != nil {
		return nil, wrapContextErr(ctx, "read database columns", err, c.cfg.Connection.Password)
	}
	return out, nil
}

func (c *Client) QueryReadonly(ctx context.Context, req QueryRequest) (QueryResult, error) {
	if c == nil || c.db == nil {
		return QueryResult{}, errors.New("database client is not open")
	}
	if !c.cfg.Safety.AllowReadonlySQL {
		return QueryResult{}, errors.New("readonly SQL is disabled")
	}
	if req.RunID == "" || req.QueryID == "" || strings.TrimSpace(req.Purpose) == "" {
		return QueryResult{}, errors.New("query runId, queryId, and purpose are required")
	}

	info, err := dbguard.ValidateReadonlyQuery(req.SQL, c.cfg.Connection.Database, c.cfg.Safety.AllowedSchemas)
	if err != nil {
		return QueryResult{}, fmt.Errorf("readonly SQL rejected: %w", err)
	}

	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	start := time.Now()

	tx, err := c.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return QueryResult{}, wrapContextErr(ctx, "begin readonly transaction", err, c.cfg.Connection.Password)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, req.SQL, req.Params...)
	if err != nil {
		return QueryResult{}, wrapContextErr(ctx, "execute readonly query", err, c.cfg.Connection.Password)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return QueryResult{}, wrapErr("read query columns", err, c.cfg.Connection.Password)
	}
	resultRows := make([]map[string]any, 0, min(c.cfg.Safety.MaxRows, 16))
	truncated := false

	for rows.Next() {
		values := make([]any, len(columns))
		targets := make([]any, len(columns))
		for i := range values {
			targets[i] = &values[i]
		}
		if err := rows.Scan(targets...); err != nil {
			return QueryResult{}, wrapErr("scan readonly query row", err, c.cfg.Connection.Password)
		}
		if len(resultRows) >= c.cfg.Safety.MaxRows {
			truncated = true
			break
		}
		row := make(map[string]any, len(columns))
		for i, col := range columns {
			row[col] = sanitizeValue(col, values[i])
		}
		resultRows = append(resultRows, row)
	}
	if err := rows.Err(); err != nil {
		return QueryResult{}, wrapContextErr(ctx, "read readonly query rows", err, c.cfg.Connection.Password)
	}

	return QueryResult{
		QueryID:       req.QueryID,
		RunID:         req.RunID,
		Purpose:       req.Purpose,
		Schema:        c.cfg.Connection.Database,
		StatementType: info.StatementType,
		Columns:       append([]string(nil), columns...),
		Rows:          resultRows,
		RowCount:      len(resultRows),
		Truncated:     truncated,
		DurationMs:    time.Since(start).Milliseconds(),
	}, nil
}

func (c *Client) validateDiscoveryTarget(schemaName, tableName string) error {
	if !c.cfg.Safety.AllowSchemaDiscovery {
		return errors.New("database schema discovery is disabled")
	}
	if !validIdentifier(schemaName) {
		return errors.New("invalid database schema identifier")
	}
	if tableName != "" && !validIdentifier(tableName) {
		return errors.New("invalid database table identifier")
	}
	allowed := false
	for _, candidate := range c.cfg.Safety.AllowedSchemas {
		if candidate == schemaName {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("schema %q is not allowed", schemaName)
	}
	return nil
}

func validIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		b := value[i]
		if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_' || b == '$' {
			continue
		}
		return false
	}
	return true
}

func (c *Client) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, time.Duration(c.cfg.Safety.TimeoutSeconds)*time.Second)
}

func sanitizeValue(column string, value any) any {
	if sensitiveColumn(column) {
		return redactedValue
	}
	switch v := value.(type) {
	case []byte:
		return string(v)
	default:
		return value
	}
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

func wrapContextErr(ctx context.Context, prefix string, err error, password string) error {
	if err == nil {
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf("%s: %w", prefix, ctxErr)
	}
	return wrapErr(prefix, err, password)
}

func wrapErr(prefix string, err error, password string) error {
	if err == nil {
		return nil
	}
	if password != "" && strings.Contains(err.Error(), password) {
		return fmt.Errorf("%s: %s", prefix, strings.ReplaceAll(err.Error(), password, redactedValue))
	}
	return fmt.Errorf("%s: %w", prefix, err)
}
