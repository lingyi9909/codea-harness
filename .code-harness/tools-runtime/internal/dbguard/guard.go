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
	switch stmt.(type) {
	case *sqlparser.Select, *sqlparser.Union:
		// allowed top-level read query types
	default:
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

	cteNames := make(map[string]struct{})
	if err := sqlparser.Walk(func(node sqlparser.SQLNode) (bool, error) {
		switch n := node.(type) {
		case *sqlparser.Select:
			addCTENames(cteNames, n.With)
		case *sqlparser.Union:
			addCTENames(cteNames, n.With)
		}
		return true, nil
	}, stmt); err != nil {
		return StatementInfo{}, fmt.Errorf("inspect CTEs: %w", err)
	}

	schemaSet := map[string]struct{}{}
	tableSet := map[string]struct{}{}
	err = sqlparser.Walk(func(node sqlparser.SQLNode) (bool, error) {
		switch n := node.(type) {
		case *sqlparser.Select:
			if n.Lock != 0 {
				return false, errors.New("locking SELECT is not allowed")
			}
			if n.Into != nil {
				return false, errors.New("SELECT INTO is not allowed")
			}
		case *sqlparser.Union:
			if n.Lock != 0 {
				return false, errors.New("locking SELECT is not allowed")
			}
			if n.Into != nil {
				return false, errors.New("SELECT INTO is not allowed")
			}
		case *sqlparser.AliasedTableExpr:
			tableName, ok := n.Expr.(sqlparser.TableName)
			if !ok {
				return true, nil
			}
			table := tableName.Name.String()
			if table == "" {
				return true, nil
			}
			schema := tableName.Qualifier.String()
			if schema == "" {
				if _, isCTE := cteNames[table]; isCTE {
					return true, nil
				}
				schema = defaultSchema
				if schema == "" {
					return false, fmt.Errorf("unqualified table %q requires a default schema", table)
				}
			}
			if _, ok := allowed[schema]; !ok {
				return false, fmt.Errorf("schema %q is not allowed", schema)
			}
			schemaSet[schema] = struct{}{}
			tableSet[table] = struct{}{}
		}
		return true, nil
	}, stmt)
	if err != nil {
		return StatementInfo{}, err
	}

	return StatementInfo{
		StatementType: "SELECT",
		Schemas:       sortedKeys(schemaSet),
		Tables:        sortedKeys(tableSet),
	}, nil
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

func sortedKeys(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
