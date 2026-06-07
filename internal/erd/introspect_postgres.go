package erd

import (
	"context"
	"database/sql"
)

func introspectPostgres(ctx context.Context, db *sql.DB, schema string, filter []string) ([]Table, error) {
	if schema == "" {
		schema = "public"
	}

	filterSet := make(map[string]bool, len(filter))
	for _, t := range filter {
		filterSet[t] = true
	}
	keep := func(name string) bool {
		return len(filterSet) == 0 || filterSet[name]
	}

	tables, err := pgTables(ctx, db, schema, keep)
	if err != nil {
		return nil, err
	}
	idx := make(map[string]int, len(tables))
	for i, t := range tables {
		idx[t.Name] = i
	}

	if err := pgColumns(ctx, db, schema, tables, idx, keep); err != nil {
		return nil, err
	}
	if err := pgPKs(ctx, db, schema, tables, idx, keep); err != nil {
		return nil, err
	}
	if err := pgFKs(ctx, db, schema, tables, idx, keep); err != nil {
		return nil, err
	}
	if err := pgIndexes(ctx, db, schema, tables, idx, keep); err != nil {
		return nil, err
	}
	return tables, nil
}

func pgTables(ctx context.Context, db *sql.DB, schema string, keep func(string) bool) ([]Table, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT t.table_name, COALESCE(s.n_live_tup, 0)
		 FROM information_schema.tables t
		 LEFT JOIN pg_stat_user_tables s
		   ON s.schemaname = t.table_schema AND s.relname = t.table_name
		 WHERE t.table_schema = $1 AND t.table_type = 'BASE TABLE'
		 ORDER BY t.table_name`, schema)
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

func pgColumns(ctx context.Context, db *sql.DB, schema string, tables []Table, idx map[string]int, keep func(string) bool) error {
	rows, err := db.QueryContext(ctx,
		`SELECT table_name, column_name, data_type, udt_name, is_nullable, column_default, ordinal_position
		 FROM information_schema.columns
		 WHERE table_schema = $1
		 ORDER BY table_name, ordinal_position`, schema)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var tbl, col, dataType, udtName, nullable string
		var def sql.NullString
		var ord int
		if err := rows.Scan(&tbl, &col, &dataType, &udtName, &nullable, &def, &ord); err != nil {
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
			UDT:      udtName,
			Nullable: nullable,
			Default:  defPtr,
			Ord:      ord,
		})
	}
	return rows.Err()
}

func pgPKs(ctx context.Context, db *sql.DB, schema string, tables []Table, idx map[string]int, keep func(string) bool) error {
	rows, err := db.QueryContext(ctx,
		`SELECT tc.table_name, kcu.column_name
		 FROM information_schema.table_constraints tc
		 JOIN information_schema.key_column_usage kcu
		   ON kcu.constraint_name = tc.constraint_name AND kcu.table_schema = tc.table_schema
		 WHERE tc.constraint_type = 'PRIMARY KEY' AND tc.table_schema = $1
		 ORDER BY tc.table_name, kcu.ordinal_position`, schema)
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

func pgFKs(ctx context.Context, db *sql.DB, schema string, tables []Table, idx map[string]int, keep func(string) bool) error {
	rows, err := db.QueryContext(ctx,
		`SELECT rel.relname AS table_name,
		        a.attname AS col,
		        frel.relname AS ref_table,
		        af.attname AS ref_col,
		        CASE c.confdeltype
		          WHEN 'c' THEN 'CASCADE'
		          WHEN 'n' THEN 'SET NULL'
		          WHEN 'd' THEN 'SET DEFAULT'
		          WHEN 'r' THEN 'RESTRICT'
		          ELSE 'NO ACTION'
		        END AS on_delete,
		        c.conname
		 FROM pg_constraint c
		 JOIN pg_class rel ON rel.oid = c.conrelid
		 JOIN pg_class frel ON frel.oid = c.confrelid
		 JOIN LATERAL unnest(c.conkey) WITH ORDINALITY AS k(attnum, ord) ON true
		 JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = k.attnum
		 JOIN LATERAL unnest(c.confkey) WITH ORDINALITY AS kf(attnum, ford) ON kf.ford = k.ord
		 JOIN pg_attribute af ON af.attrelid = c.confrelid AND af.attnum = kf.attnum
		 WHERE c.contype = 'f'
		   AND c.connamespace = (SELECT oid FROM pg_namespace WHERE nspname = $1)
		 ORDER BY rel.relname, c.conname, k.ord`, schema)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var tbl, col, refTable, refCol, onDelete, constraint string
		if err := rows.Scan(&tbl, &col, &refTable, &refCol, &onDelete, &constraint); err != nil {
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
			OnDelete:   onDelete,
			Constraint: constraint,
			Inferred:   false,
		})
	}
	return rows.Err()
}

func pgIndexes(ctx context.Context, db *sql.DB, schema string, tables []Table, idx map[string]int, keep func(string) bool) error {
	rows, err := db.QueryContext(ctx,
		`SELECT i.tablename, i.indexname, i.indexdef
		 FROM pg_indexes i
		 WHERE i.schemaname = $1
		   AND NOT EXISTS (
		     SELECT 1 FROM pg_constraint c
		     JOIN pg_class rc ON rc.oid = c.conindid
		     WHERE c.contype = 'p' AND rc.relname = i.indexname
		   )
		 ORDER BY i.tablename, i.indexname`, schema)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var tbl, idxName, idxDef string
		if err := rows.Scan(&tbl, &idxName, &idxDef); err != nil {
			return err
		}
		ti, ok := idx[tbl]
		if !ok || !keep(tbl) {
			continue
		}
		tables[ti].Indexes = append(tables[ti].Indexes, Index{
			Name: idxName,
			Def:  idxDef,
		})
	}
	return rows.Err()
}
