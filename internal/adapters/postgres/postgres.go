package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/vanducng/miu-db/internal/adapter"
	"github.com/vanducng/miu-db/internal/config"
	"github.com/vanducng/miu-db/internal/schema"
	"github.com/vanducng/miu-db/internal/tunnel"
)

type Provider struct{}

func New() Provider { return Provider{} }

func (Provider) Type() string { return "postgresql" }

func (p Provider) Open(ctx context.Context, conn config.Connection) (*adapter.Session, error) {
	targetHost, targetPort := conn.Endpoint.Host, defaultString(conn.Endpoint.Port, "5432")
	closer := func() error { return nil }
	if conn.Tunnel != nil && conn.Tunnel.Enabled {
		forward, err := tunnel.Open(ctx, *conn.Tunnel, targetHost, targetPort)
		if err != nil {
			return nil, err
		}
		targetHost, targetPort = forward.Host, forward.Port
		closer = forward.Close
	}
	dsn := conn.ConnectionURL
	if dsn == "" || conn.Endpoint.Password != "" || conn.Tunnel != nil && conn.Tunnel.Enabled {
		u := &url.URL{
			Scheme: "postgres",
			User:   url.UserPassword(conn.Endpoint.Username, conn.Endpoint.Password),
			Host:   net.JoinHostPort(targetHost, targetPort),
			Path:   conn.Endpoint.Database,
		}
		q := u.Query()
		sslMode := "prefer"
		if v, ok := conn.Options["tls_mode"].(string); ok && v != "" {
			sslMode = v
		}
		if conn.ExtraOptions != nil && conn.ExtraOptions["sslmode"] != "" {
			sslMode = conn.ExtraOptions["sslmode"]
		}
		q.Set("sslmode", sslMode)
		u.RawQuery = q.Encode()
		dsn = u.String()
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		_ = closer()
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		_ = closer()
		return nil, err
	}
	return &adapter.Session{DB: db, Closer: closer, Provider: p, Config: conn}, nil
}

func (Provider) BuildSelect(table string, limit int) string {
	return fmt.Sprintf("SELECT * FROM %s LIMIT %d", quotePGName(table), limit)
}

func (Provider) Schema(ctx context.Context, session *adapter.Session) (any, error) {
	const q = `
SELECT table_schema, table_name, table_type
FROM information_schema.tables
WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
ORDER BY table_schema, table_name`
	rows, err := session.DB.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tree := schema.Tree{Tables: []schema.Table{}}
	for rows.Next() {
		var sch, name, typ string
		if err := rows.Scan(&sch, &name, &typ); err != nil {
			return nil, err
		}
		tree.Tables = append(tree.Tables, schema.Table{Schema: sch, Name: name, Type: typ})
	}
	return tree, rows.Err()
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func quotePGName(name string) string {
	parts := strings.Split(name, ".")
	for i, part := range parts {
		parts[i] = `"` + strings.ReplaceAll(part, `"`, `""`) + `"`
	}
	return strings.Join(parts, ".")
}
