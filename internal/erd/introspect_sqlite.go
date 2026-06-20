package erd

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

func introspectSQLite(ctx context.Context, db *sql.DB, _ string, filter []string) ([]Table, error) {
	filterSet := make(map[string]bool, len(filter))
	for _, t := range filter {
		filterSet[t] = true
	}
	keep := func(name string) bool {
		return len(filterSet) == 0 || filterSet[name]
	}

	tables, err := sqliteTables(ctx, db, keep)
	if err != nil {
		return nil, err
	}
	for i := range tables {
		name := tables[i].Name
		if err := sqliteColumns(ctx, db, name, &tables[i]); err != nil {
			return nil, fmt.Errorf("introspect table %q columns: %w", name, err)
		}
		if err := sqliteFKs(ctx, db, name, &tables[i]); err != nil {
			return nil, fmt.Errorf("introspect table %q foreign keys: %w", name, err)
		}
		if err := sqliteIndexes(ctx, db, name, &tables[i]); err != nil {
			return nil, fmt.Errorf("introspect table %q indexes: %w", name, err)
		}
	}
	return tables, nil
}

// quoteIdent escapes a SQLite identifier for interpolation where a bound
// parameter is not allowed (PRAGMA args, FROM targets). %q is wrong here: it
// backslash-escapes embedded quotes, but SQLite doubles them.
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func sqliteTables(ctx context.Context, db *sql.DB, keep func(string) bool) ([]Table, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []Table
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if !keep(name) {
			continue
		}
		var rowCount int64
		if err := db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s`, quoteIdent(name))).Scan(&rowCount); err != nil {
			return nil, fmt.Errorf("count rows for %q: %w", name, err)
		}
		tables = append(tables, Table{Name: name, Rows: rowCount, PK: []string{}, Columns: []Column{}, FKs: []FK{}, Indexes: []Index{}})
	}
	return tables, rows.Err()
}

func sqliteColumns(ctx context.Context, db *sql.DB, tableName string, t *Table) error {
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s)`, quoteIdent(tableName)))
	if err != nil {
		return err
	}
	defer rows.Close()

	type pkEntry struct {
		col   string
		pkPos int
	}
	var pks []pkEntry

	for rows.Next() {
		var cid, notNull, pk int
		var name, colType string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return err
		}
		nullable := "YES"
		if notNull != 0 {
			nullable = "NO"
		}
		var defPtr *string
		if dflt.Valid {
			s := dflt.String
			defPtr = &s
		}
		t.Columns = append(t.Columns, Column{
			Name:     name,
			Type:     colType,
			UDT:      colType,
			Nullable: nullable,
			Default:  defPtr,
			Ord:      cid + 1,
		})
		if pk > 0 {
			pks = append(pks, pkEntry{col: name, pkPos: pk})
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// pk values are 1-based position; sort ascending to preserve composite-key order.
	sort.Slice(pks, func(i, j int) bool { return pks[i].pkPos < pks[j].pkPos })
	for _, p := range pks {
		t.PK = append(t.PK, p.col)
	}
	return nil
}

func sqliteFKs(ctx context.Context, db *sql.DB, tableName string, t *Table) error {
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`PRAGMA foreign_key_list(%s)`, quoteIdent(tableName)))
	if err != nil {
		return err
	}
	defer rows.Close()

	// foreign_key_list returns one row per FK column; rows sharing an id form one
	// (possibly composite) FK, with seq ordering the columns. Group by id so a
	// composite FK keeps a single constraint name and renders as one DBML ref.
	type fkRow struct {
		id, seq                            int
		refTable, fromCol, toCol, onDelete string
	}
	var fks []fkRow
	for rows.Next() {
		var r fkRow
		var onUpdate, match string
		if err := rows.Scan(&r.id, &r.seq, &r.refTable, &r.fromCol, &r.toCol, &onUpdate, &r.onDelete, &match); err != nil {
			return err
		}
		fks = append(fks, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	sort.Slice(fks, func(i, j int) bool {
		if fks[i].id != fks[j].id {
			return fks[i].id < fks[j].id
		}
		return fks[i].seq < fks[j].seq
	})
	for _, r := range fks {
		t.FKs = append(t.FKs, FK{
			Column:     r.fromCol,
			RefTable:   r.refTable,
			RefColumn:  r.toCol,
			OnDelete:   r.onDelete,
			Constraint: fmt.Sprintf("fk_%s_%d", tableName, r.id),
			Inferred:   false,
		})
	}
	return nil
}

func sqliteIndexes(ctx context.Context, db *sql.DB, tableName string, t *Table) error {
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`PRAGMA index_list(%s)`, quoteIdent(tableName)))
	if err != nil {
		return err
	}
	defer rows.Close()

	type idxMeta struct {
		name   string
		unique int
	}
	var metas []idxMeta

	for rows.Next() {
		var seq, unique int
		var name, origin string
		var partial int
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return err
		}
		// Skip implicit PK indexes and SQLite-managed auto-indexes.
		if origin == "pk" || strings.HasPrefix(name, "sqlite_autoindex") {
			continue
		}
		metas = append(metas, idxMeta{name: name, unique: unique})
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, m := range metas {
		def, err := sqliteIndexDef(ctx, db, m.name, m.unique)
		if err != nil {
			return err
		}
		t.Indexes = append(t.Indexes, Index{Name: m.name, Def: def})
	}
	return nil
}

func sqliteIndexDef(ctx context.Context, db *sql.DB, idxName string, unique int) (string, error) {
	var sqlText sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT sql FROM sqlite_master WHERE type='index' AND name=?`, idxName).Scan(&sqlText); err != nil {
		return "", err
	}
	if sqlText.Valid && sqlText.String != "" {
		return sqlText.String, nil
	}

	// Synthesize def from PRAGMA index_info when sql is NULL (e.g. auto-created unique indexes).
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`PRAGMA index_info(%s)`, quoteIdent(idxName)))
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var seqno, cid int
		var name string
		if err := rows.Scan(&seqno, &cid, &name); err != nil {
			return "", err
		}
		cols = append(cols, name)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	prefix := "INDEX "
	if unique != 0 {
		prefix = "UNIQUE INDEX "
	}
	return prefix + idxName + " (" + strings.Join(cols, ", ") + ")", nil
}
