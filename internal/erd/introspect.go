package erd

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrUnsupported is returned by Introspect when dbtype has no introspector yet.
var ErrUnsupported = errors.New("erd: unsupported database type for introspection")

// Introspect queries information_schema (or equivalent) for the given dbtype
// and returns the raw []Table slice ready for Normalize.
// tables, when non-nil, restricts the output to those table names.
func Introspect(ctx context.Context, db *sql.DB, dbtype, schema string, tables []string) ([]Table, error) {
	switch dbtype {
	case "mysql":
		return introspectMySQL(ctx, db, schema, tables)
	case "postgres":
		return introspectPostgres(ctx, db, schema, tables)
	default:
		return nil, fmt.Errorf("%w: %s (introspection planned)", ErrUnsupported, dbtype)
	}
}
