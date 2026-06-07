package erd

import (
	"context"
	"database/sql"
	"strings"
)

func introspectMySQL(ctx context.Context, db *sql.DB, schema string, filter []string) ([]Table, error) {
	if schema == "" {
		if err := db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&schema); err != nil {
			return nil, err
		}
	}

	filterSet := make(map[string]bool, len(filter))
	for _, t := range filter {
		filterSet[t] = true
	}
	keep := func(name string) bool {
		return len(filterSet) == 0 || filterSet[name]
	}

	tables, err := mysqlTables(ctx, db, schema, keep)
	if err != nil {
		return nil, err
	}
	idx := make(map[string]int, len(tables))
	for i, t := range tables {
		idx[t.Name] = i
	}

	if err := mysqlColumns(ctx, db, schema, tables, idx, keep); err != nil {
		return nil, err
	}
	if err := mysqlPKs(ctx, db, schema, tables, idx, keep); err != nil {
		return nil, err
	}
	if err := mysqlFKs(ctx, db, schema, tables, idx, keep); err != nil {
		return nil, err
	}
	if err := mysqlIndexes(ctx, db, schema, tables, idx, keep); err != nil {
		return nil, err
	}
	return tables, nil
}

func mysqlTables(ctx context.Context, db *sql.DB, schema string, keep func(string) bool) ([]Table, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT TABLE_NAME, COALESCE(TABLE_ROWS,0)
		 FROM information_schema.TABLES
		 WHERE TABLE_SCHEMA=? AND TABLE_TYPE='BASE TABLE'
		 ORDER BY TABLE_NAME`, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tables []Table
	for rows.Next() {
		var name string
		var rowCount int64
		if err := rows.Scan(&name, &rowCount); err != nil {
			return nil, err
		}
		if keep(name) {
			tables = append(tables, Table{Name: name, Rows: rowCount, PK: []string{}, Columns: []Column{}, FKs: []FK{}, Indexes: []Index{}})
		}
	}
	return tables, rows.Err()
}

func mysqlColumns(ctx context.Context, db *sql.DB, schema string, tables []Table, idx map[string]int, keep func(string) bool) error {
	rows, err := db.QueryContext(ctx,
		`SELECT TABLE_NAME, COLUMN_NAME, DATA_TYPE, COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT, ORDINAL_POSITION
		 FROM information_schema.COLUMNS
		 WHERE TABLE_SCHEMA=?
		 ORDER BY TABLE_NAME, ORDINAL_POSITION`, schema)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var tbl, col, dataType, colType, nullable string
		var def sql.NullString
		var ord int
		if err := rows.Scan(&tbl, &col, &dataType, &colType, &nullable, &def, &ord); err != nil {
			return err
		}
		i, ok := idx[tbl]
		if !ok || !keep(tbl) {
			continue
		}
		var defPtr *string
		if def.Valid {
			s := def.String
			defPtr = &s
		}
		tables[i].Columns = append(tables[i].Columns, Column{
			Name:     col,
			Type:     dataType,
			UDT:      colType,
			Nullable: nullable,
			Default:  defPtr,
			Ord:      ord,
		})
	}
	return rows.Err()
}

func mysqlPKs(ctx context.Context, db *sql.DB, schema string, tables []Table, idx map[string]int, keep func(string) bool) error {
	rows, err := db.QueryContext(ctx,
		`SELECT TABLE_NAME, COLUMN_NAME
		 FROM information_schema.KEY_COLUMN_USAGE
		 WHERE TABLE_SCHEMA=? AND CONSTRAINT_NAME='PRIMARY'
		 ORDER BY TABLE_NAME, ORDINAL_POSITION`, schema)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var tbl, col string
		if err := rows.Scan(&tbl, &col); err != nil {
			return err
		}
		i, ok := idx[tbl]
		if !ok || !keep(tbl) {
			continue
		}
		tables[i].PK = append(tables[i].PK, col)
	}
	return rows.Err()
}

func mysqlFKs(ctx context.Context, db *sql.DB, schema string, tables []Table, idx map[string]int, keep func(string) bool) error {
	rows, err := db.QueryContext(ctx,
		`SELECT k.TABLE_NAME, k.COLUMN_NAME, k.REFERENCED_TABLE_NAME, k.REFERENCED_COLUMN_NAME,
		        rc.DELETE_RULE, k.CONSTRAINT_NAME
		 FROM information_schema.KEY_COLUMN_USAGE k
		 JOIN information_schema.REFERENTIAL_CONSTRAINTS rc
		   ON rc.CONSTRAINT_SCHEMA=k.TABLE_SCHEMA AND rc.CONSTRAINT_NAME=k.CONSTRAINT_NAME
		 WHERE k.TABLE_SCHEMA=? AND k.REFERENCED_TABLE_NAME IS NOT NULL
		 ORDER BY k.TABLE_NAME, k.CONSTRAINT_NAME, k.ORDINAL_POSITION`, schema)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var tbl, col, refTable, refCol, delRule, constraint string
		if err := rows.Scan(&tbl, &col, &refTable, &refCol, &delRule, &constraint); err != nil {
			return err
		}
		i, ok := idx[tbl]
		if !ok || !keep(tbl) {
			continue
		}
		tables[i].FKs = append(tables[i].FKs, FK{
			Column:     col,
			RefTable:   refTable,
			RefColumn:  refCol,
			OnDelete:   delRule,
			Constraint: constraint,
		})
	}
	return rows.Err()
}

func mysqlIndexes(ctx context.Context, db *sql.DB, schema string, tables []Table, idx map[string]int, keep func(string) bool) error {
	rows, err := db.QueryContext(ctx,
		`SELECT TABLE_NAME, INDEX_NAME, NON_UNIQUE, SEQ_IN_INDEX, COLUMN_NAME
		 FROM information_schema.STATISTICS
		 WHERE TABLE_SCHEMA=? AND INDEX_NAME<>'PRIMARY'
		 ORDER BY TABLE_NAME, INDEX_NAME, SEQ_IN_INDEX`, schema)
	if err != nil {
		return err
	}
	defer rows.Close()

	type idxKey struct{ table, name string }
	type idxEntry struct {
		nonUnique int
		cols      []string
	}
	var order []idxKey
	entries := map[idxKey]*idxEntry{}

	for rows.Next() {
		var tbl, idxName, col string
		var nonUnique, seq int
		if err := rows.Scan(&tbl, &idxName, &nonUnique, &seq, &col); err != nil {
			return err
		}
		if !keep(tbl) {
			continue
		}
		k := idxKey{tbl, idxName}
		e, ok := entries[k]
		if !ok {
			e = &idxEntry{nonUnique: nonUnique}
			entries[k] = e
			order = append(order, k)
		}
		e.cols = append(e.cols, col)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, k := range order {
		ti, ok := idx[k.table]
		if !ok {
			continue
		}
		e := entries[k]
		tables[ti].Indexes = append(tables[ti].Indexes, Index{
			Name: k.name,
			Def:  buildIndexDef(k.name, e.cols, e.nonUnique == 0),
		})
	}
	return nil
}

// buildIndexDef assembles the human-readable index definition stored in the IR.
func buildIndexDef(name string, cols []string, unique bool) string {
	prefix := "INDEX "
	if unique {
		prefix = "UNIQUE INDEX "
	}
	return prefix + name + " (" + strings.Join(cols, ", ") + ")"
}
