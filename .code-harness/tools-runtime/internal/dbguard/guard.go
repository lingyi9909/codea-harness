package dbguard

import (
	"errors"
	"fmt"
	"sort"

	"vitess.io/vitess/go/vt/sqlparser"
)

type StatementInfo struct {
	StatementType string
	Schemas       []string
	Tables        []string
}

type collector struct {
	defaultSchema string
	allowed       map[string]struct{}
	schemaSet     map[string]struct{}
	tableSet      map[string]struct{}
}

func ValidateReadonlyQuery(sqlText, defaultSchema string, allowedSchemas []string) (StatementInfo, error) {
	parser, err := sqlparser.New(sqlparser.Options{})
	if err != nil {
		return StatementInfo{}, fmt.Errorf("initialize SQL parser: %w", err)
	}

	pieces, err := parser.SplitStatementToPieces(sqlText)
	if err != nil {
		return StatementInfo{}, fmt.Errorf("parse SQL statements: %w", err)
	}
	if len(pieces) != 1 {
		return StatementInfo{}, fmt.Errorf("readonly SQL requires exactly one statement, got %d", len(pieces))
	}

	stmt, err := parser.Parse(pieces[0])
	if err != nil {
		return StatementInfo{}, fmt.Errorf("parse SQL: %w", err)
	}
	selectStmt, ok := stmt.(sqlparser.SelectStatement)
	if !ok {
		return StatementInfo{}, fmt.Errorf("statement type %T is not readonly SELECT", stmt)
	}

	allowed := make(map[string]struct{}, len(allowedSchemas))
	for _, schema := range allowedSchemas {
		if schema == "" {
			return StatementInfo{}, errors.New("allowedSchemas must not contain empty schema")
		}
		allowed[schema] = struct{}{}
	}
	if len(allowed) == 0 {
		return StatementInfo{}, errors.New("allowedSchemas must not be empty")
	}

	c := &collector{
		defaultSchema: defaultSchema,
		allowed:       allowed,
		schemaSet:     map[string]struct{}{},
		tableSet:      map[string]struct{}{},
	}
	if err := c.collectSelectStatement(selectStmt, nil); err != nil {
		return StatementInfo{}, err
	}

	return StatementInfo{
		StatementType: "SELECT",
		Schemas:       sortedKeys(c.schemaSet),
		Tables:        sortedKeys(c.tableSet),
	}, nil
}

func (c *collector) collectSelectStatement(stmt sqlparser.SelectStatement, inherited map[string]struct{}) error {
	var with *sqlparser.With
	switch n := stmt.(type) {
	case *sqlparser.Select:
		if n.Lock != 0 {
			return errors.New("locking SELECT is not allowed")
		}
		if n.Into != nil {
			return errors.New("SELECT INTO is not allowed")
		}
		with = n.With
	case *sqlparser.Union:
		if n.Lock != 0 {
			return errors.New("locking SELECT is not allowed")
		}
		if n.Into != nil {
			return errors.New("SELECT INTO is not allowed")
		}
		with = n.With
	default:
		return fmt.Errorf("statement type %T is not readonly SELECT", stmt)
	}

	if err := c.collectCTEDefinitions(with, inherited); err != nil {
		return err
	}

	bodyScope := cloneScope(inherited)
	addCTENames(bodyScope, with)
	root := sqlparser.SQLNode(stmt)
	return sqlparser.Walk(func(node sqlparser.SQLNode) (bool, error) {
		switch n := node.(type) {
		case *sqlparser.CommonTableExpr:
			return false, nil
		case *sqlparser.Select:
			if node == root {
				return true, nil
			}
			if err := c.collectSelectStatement(n, bodyScope); err != nil {
				return false, err
			}
			return false, nil
		case *sqlparser.Union:
			if node == root {
				return true, nil
			}
			if err := c.collectSelectStatement(n, bodyScope); err != nil {
				return false, err
			}
			return false, nil
		case *sqlparser.AliasedTableExpr:
			if err := c.collectTable(n, bodyScope); err != nil {
				return false, err
			}
		}
		return true, nil
	}, root)
}

func (c *collector) collectCTEDefinitions(with *sqlparser.With, inherited map[string]struct{}) error {
	if with == nil {
		return nil
	}
	visible := cloneScope(inherited)
	for _, cte := range with.CTEs {
		if cte == nil {
			continue
		}
		name := cte.ID.String()
		definitionScope := cloneScope(visible)
		if with.Recursive && name != "" {
			definitionScope[name] = struct{}{}
		}
		if err := c.collectSelectStatement(cte.Subquery, definitionScope); err != nil {
			return err
		}
		if name != "" {
			visible[name] = struct{}{}
		}
	}
	return nil
}

func (c *collector) collectTable(expr *sqlparser.AliasedTableExpr, scope map[string]struct{}) error {
	tableName, ok := expr.Expr.(sqlparser.TableName)
	if !ok {
		return nil
	}
	table := tableName.Name.String()
	if table == "" {
		return nil
	}
	schema := tableName.Qualifier.String()
	if schema == "" && table == "dual" {
		return nil
	}
	if schema == "" {
		if _, isCTE := scope[table]; isCTE {
			return nil
		}
		schema = c.defaultSchema
		if schema == "" {
			return fmt.Errorf("unqualified table %q requires a default schema", table)
		}
	}
	if _, ok := c.allowed[schema]; !ok {
		return fmt.Errorf("schema %q is not allowed", schema)
	}
	c.schemaSet[schema] = struct{}{}
	c.tableSet[table] = struct{}{}
	return nil
}

func addCTENames(names map[string]struct{}, with *sqlparser.With) {
	if with == nil {
		return
	}
	for _, cte := range with.CTEs {
		if cte == nil {
			continue
		}
		if name := cte.ID.String(); name != "" {
			names[name] = struct{}{}
		}
	}
}

func cloneScope(scope map[string]struct{}) map[string]struct{} {
	cloned := make(map[string]struct{}, len(scope))
	for name := range scope {
		cloned[name] = struct{}{}
	}
	return cloned
}

func sortedKeys(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
