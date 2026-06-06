package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"

	"github.com/vanducng/miu-db/internal/adapter"
	"github.com/vanducng/miu-db/internal/config"
	"github.com/vanducng/miu-db/internal/schema"
)

type Provider struct{}

func New() Provider { return Provider{} }

func (Provider) Type() string { return "sqlite" }

// ScriptUnsupportedReason rejects multi-statement scripts: the modernc driver's
// Query keeps only the LAST result set (silently dropping earlier ones).
func (Provider) ScriptUnsupportedReason() string {
	return "sqlite scripts are not supported (only the final result set would be returned); run statements individually with 'query run'"
}

func (p Provider) Open(ctx context.Context, conn config.Connection) (*adapter.Session, error) {
	path := conn.Endpoint.Path
	if path == "" {
		return nil, fmt.Errorf("sqlite path is required")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &adapter.Session{DB: db, Provider: p, Config: conn}, nil
}

func (Provider) BuildSelect(table string, limit int) string {
	return fmt.Sprintf("SELECT * FROM %q LIMIT %d", table, limit)
}

func (Provider) Schema(ctx context.Context, session *adapter.Session) (any, error) {
	rows, err := session.DB.QueryContext(ctx, "SELECT name, type FROM sqlite_master WHERE type IN ('table','view') AND name NOT LIKE 'sqlite_%' ORDER BY type, name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tree := schema.Tree{Tables: []schema.Table{}}
	for rows.Next() {
		var name, typ string
		if err := rows.Scan(&name, &typ); err != nil {
			return nil, err
		}
		tree.Tables = append(tree.Tables, schema.Table{Name: name, Type: typ})
		colRows, err := session.DB.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%q)", name))
		if err != nil {
			continue
		}
		for colRows.Next() {
			var cid int
			var colName, colType string
			var notNull, pk int
			var dflt any
			if err := colRows.Scan(&cid, &colName, &colType, &notNull, &dflt, &pk); err == nil {
				tree.Columns = append(tree.Columns, schema.Column{Table: name, Name: colName, Type: colType})
			}
		}
		_ = colRows.Close()
	}
	return tree, rows.Err()
}
