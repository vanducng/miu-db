package erd

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// openTestDB creates a file-backed SQLite DB so all pool connections share state.
// :memory: gives each connection its own ephemeral database, breaking PRAGMA queries.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func seedSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	stmts := []string{
		`CREATE TABLE authors (
			id   INTEGER NOT NULL,
			name TEXT    NOT NULL,
			bio  TEXT,
			PRIMARY KEY (id)
		)`,
		`CREATE TABLE books (
			id        INTEGER NOT NULL,
			author_id INTEGER NOT NULL,
			title     TEXT    NOT NULL,
			PRIMARY KEY (id),
			FOREIGN KEY (author_id) REFERENCES authors(id) ON DELETE CASCADE
		)`,
		`CREATE UNIQUE INDEX idx_books_title ON books (title)`,
		`INSERT INTO authors (id, name, bio) VALUES (1, 'Alice', NULL)`,
		`INSERT INTO books  (id, author_id, title) VALUES (1, 1, 'Go Handbook')`,
		`INSERT INTO books  (id, author_id, title) VALUES (2, 1, 'SQL Mastery')`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("seed %q: %v", s, err)
		}
	}
}

func TestIntrospectSQLite_FullSchema(t *testing.T) {
	db := openTestDB(t)
	seedSchema(t, db)

	tables, err := introspectSQLite(context.Background(), db, "", nil)
	if err != nil {
		t.Fatalf("introspect: %v", err)
	}
	Normalize(tables)

	if len(tables) != 2 {
		t.Fatalf("want 2 tables, got %d", len(tables))
	}

	// Tables are sorted alphabetically after Normalize: authors, books.
	authors := tables[0]
	books := tables[1]

	if authors.Name != "authors" {
		t.Errorf("tables[0].Name = %q, want authors", authors.Name)
	}
	if books.Name != "books" {
		t.Errorf("tables[1].Name = %q, want books", books.Name)
	}

	// Row counts.
	if authors.Rows != 1 {
		t.Errorf("authors.Rows = %d, want 1", authors.Rows)
	}
	if books.Rows != 2 {
		t.Errorf("books.Rows = %d, want 2", books.Rows)
	}

	// authors PK.
	if len(authors.PK) != 1 || authors.PK[0] != "id" {
		t.Errorf("authors.PK = %v, want [id]", authors.PK)
	}

	// Column nullability and ordinal order.
	if len(authors.Columns) != 3 {
		t.Fatalf("authors column count = %d, want 3", len(authors.Columns))
	}
	checkCol(t, authors.Columns[0], "id", "NO", 1)
	checkCol(t, authors.Columns[1], "name", "NO", 2)
	checkCol(t, authors.Columns[2], "bio", "YES", 3)

	// books FK.
	if len(books.FKs) != 1 {
		t.Fatalf("books FK count = %d, want 1", len(books.FKs))
	}
	fk := books.FKs[0]
	if fk.Column != "author_id" {
		t.Errorf("FK.Column = %q, want author_id", fk.Column)
	}
	if fk.RefTable != "authors" {
		t.Errorf("FK.RefTable = %q, want authors", fk.RefTable)
	}
	if fk.RefColumn != "id" {
		t.Errorf("FK.RefColumn = %q, want id", fk.RefColumn)
	}
	if fk.OnDelete != "CASCADE" {
		t.Errorf("FK.OnDelete = %q, want CASCADE", fk.OnDelete)
	}
	if fk.Constraint == "" {
		t.Error("FK.Constraint is empty")
	}

	// books secondary index captured; auto PK index excluded.
	if len(books.Indexes) != 1 {
		t.Fatalf("books index count = %d, want 1", len(books.Indexes))
	}
	if books.Indexes[0].Name != "idx_books_title" {
		t.Errorf("index name = %q, want idx_books_title", books.Indexes[0].Name)
	}
	if books.Indexes[0].Def == "" {
		t.Error("index Def is empty")
	}

	// authors has no secondary indexes.
	if len(authors.Indexes) != 0 {
		t.Errorf("authors index count = %d, want 0", len(authors.Indexes))
	}
}

func TestIntrospectSQLite_Filter(t *testing.T) {
	db := openTestDB(t)
	seedSchema(t, db)

	tables, err := introspectSQLite(context.Background(), db, "", []string{"authors"})
	if err != nil {
		t.Fatalf("introspect: %v", err)
	}
	if len(tables) != 1 {
		t.Fatalf("filter: want 1 table, got %d", len(tables))
	}
	if tables[0].Name != "authors" {
		t.Errorf("filter: got %q, want authors", tables[0].Name)
	}
}

func TestIntrospectSQLite_SchemaArgIgnored(t *testing.T) {
	db := openTestDB(t)
	seedSchema(t, db)

	// "main" and "" must both work identically.
	t1, err := introspectSQLite(context.Background(), db, "", nil)
	if err != nil {
		t.Fatalf("empty schema: %v", err)
	}
	t2, err := introspectSQLite(context.Background(), db, "main", nil)
	if err != nil {
		t.Fatalf("main schema: %v", err)
	}
	if len(t1) != len(t2) {
		t.Errorf("schema arg mismatch: %d vs %d tables", len(t1), len(t2))
	}
}

func seedCompositeSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	stmts := []string{
		`CREATE TABLE parent (
			a_id  INTEGER NOT NULL,
			b_id  INTEGER NOT NULL,
			label TEXT,
			PRIMARY KEY (a_id, b_id)
		)`,
		`CREATE TABLE child (
			id INTEGER PRIMARY KEY,
			pa INTEGER NOT NULL,
			pb INTEGER NOT NULL,
			FOREIGN KEY (pa, pb) REFERENCES parent(a_id, b_id) ON DELETE CASCADE
		)`,
		// PK declared y-before-x to exercise pk-position ordering vs column order.
		`CREATE TABLE composite_pk (
			x INTEGER NOT NULL,
			y INTEGER NOT NULL,
			z TEXT,
			PRIMARY KEY (y, x)
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("seed %q: %v", s, err)
		}
	}
}

func TestIntrospectSQLite_Composite(t *testing.T) {
	db := openTestDB(t)
	seedCompositeSchema(t, db)

	tables, err := introspectSQLite(context.Background(), db, "", nil)
	if err != nil {
		t.Fatalf("introspect: %v", err)
	}
	Normalize(tables)

	byName := map[string]Table{}
	for _, tbl := range tables {
		byName[tbl.Name] = tbl
	}

	// Composite PK preserves declaration order (y, x), not column order.
	if got := byName["composite_pk"].PK; len(got) != 2 || got[0] != "y" || got[1] != "x" {
		t.Errorf("composite_pk.PK = %v, want [y x]", got)
	}

	// Composite FK: two columns, ONE shared constraint, seq order preserved.
	child := byName["child"]
	if len(child.FKs) != 2 {
		t.Fatalf("child FK rows = %d, want 2", len(child.FKs))
	}
	if child.FKs[0].Constraint != child.FKs[1].Constraint {
		t.Errorf("composite FK split across constraints: %q vs %q", child.FKs[0].Constraint, child.FKs[1].Constraint)
	}
	if child.FKs[0].Column != "pa" || child.FKs[1].Column != "pb" {
		t.Errorf("composite FK columns = [%q %q], want [pa pb]", child.FKs[0].Column, child.FKs[1].Column)
	}
	if child.FKs[0].RefColumn != "a_id" || child.FKs[1].RefColumn != "b_id" {
		t.Errorf("composite FK ref columns = [%q %q], want [a_id b_id]", child.FKs[0].RefColumn, child.FKs[1].RefColumn)
	}

	// Regression: composite FK must render as ONE grouped DBML ref, not two.
	dbml := EmitDBML(tables, Meta{DatabaseType: "SQLite"})
	if !strings.Contains(dbml, "Ref: child.(pa, pb) > parent.(a_id, b_id)") {
		t.Errorf("composite FK not grouped in DBML:\n%s", dbml)
	}
	if n := strings.Count(dbml, "Ref: child."); n != 1 {
		t.Errorf("want exactly 1 child Ref line, got %d:\n%s", n, dbml)
	}
}

func checkCol(t *testing.T, c Column, wantName, wantNullable string, wantOrd int) {
	t.Helper()
	if c.Name != wantName {
		t.Errorf("column name = %q, want %q", c.Name, wantName)
	}
	if c.Nullable != wantNullable {
		t.Errorf("column %q nullable = %q, want %q", c.Name, c.Nullable, wantNullable)
	}
	if c.Ord != wantOrd {
		t.Errorf("column %q ord = %d, want %d", c.Name, c.Ord, wantOrd)
	}
}
