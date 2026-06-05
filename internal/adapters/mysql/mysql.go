package mysql

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"net"
	"strings"
	"sync"

	_ "github.com/go-sql-driver/mysql"
	driver "github.com/go-sql-driver/mysql"

	"github.com/vanducng/miu-db/internal/adapter"
	"github.com/vanducng/miu-db/internal/config"
	"github.com/vanducng/miu-db/internal/schema"
	"github.com/vanducng/miu-db/internal/tunnel"
)

type Provider struct{}

var registerCompatTLSOnce sync.Once

func init() {
	driver.SetLogger(discardLogger{})
}

func New() Provider { return Provider{} }

func (Provider) Type() string { return "mysql" }

func (p Provider) Open(ctx context.Context, conn config.Connection) (*adapter.Session, error) {
	targetHost, targetPort := conn.Endpoint.Host, defaultString(conn.Endpoint.Port, "3306")
	closer := func() error { return nil }
	if conn.Tunnel != nil && conn.Tunnel.Enabled {
		forward, err := tunnel.Open(ctx, *conn.Tunnel, targetHost, targetPort)
		if err != nil {
			return nil, err
		}
		targetHost, targetPort = forward.Host, forward.Port
		closer = forward.Close
	}
	cfg := buildConfig(conn, targetHost, targetPort)
	db, err := sql.Open("mysql", cfg.FormatDSN())
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

// SessionKeys are the per-call --session keys MySQL accepts. They map to
// connection-time system variables via the driver's DSN params. Values are
// passed verbatim — quote them if MySQL requires it (e.g. time_zone='+00:00').
func (Provider) SessionKeys() []string {
	return []string{"sql_mode", "time_zone", "max_execution_time"}
}

func buildConfig(conn config.Connection, host, port string) *driver.Config {
	cfg := driver.NewConfig()
	cfg.User = conn.Endpoint.Username
	cfg.Passwd = conn.Endpoint.Password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(host, port)
	cfg.DBName = conn.Endpoint.Database
	cfg.ParseTime = true
	tlsMode := ""
	if v, ok := conn.Options["tls_mode"].(string); ok {
		tlsMode = v
	}
	if v := conn.ExtraOptions["ssl-mode"]; v != "" {
		tlsMode = v
	}
	if v := conn.ExtraOptions["sslmode"]; v != "" {
		tlsMode = v
	}
	if strings.EqualFold(tlsMode, "insecure") || strings.EqualFold(tlsMode, "skip-verify") {
		registerCompatTLSOnce.Do(func() {
			_ = driver.RegisterTLSConfig("miudb-compat", &tls.Config{InsecureSkipVerify: true})
		})
		cfg.TLSConfig = "miudb-compat"
	} else if strings.EqualFold(tlsMode, "verify-full") || strings.EqualFold(tlsMode, "verify_identity") || strings.EqualFold(tlsMode, "verify-ca") {
		cfg.TLSConfig = "true"
	} else if strings.EqualFold(tlsMode, "require") || strings.EqualFold(tlsMode, "required") {
		cfg.TLSConfig = "preferred"
	}
	applyMySQLSessionOptions(cfg, conn.Options)
	return cfg
}

func applyMySQLSessionOptions(cfg *driver.Config, opts map[string]any) {
	for _, key := range []string{"sql_mode", "time_zone", "max_execution_time"} {
		v, ok := opts[key].(string)
		if !ok || v == "" {
			continue
		}
		if cfg.Params == nil {
			cfg.Params = map[string]string{}
		}
		cfg.Params[key] = v
	}
}

func (Provider) BuildSelect(table string, limit int) string {
	return fmt.Sprintf("SELECT * FROM %s LIMIT %d", quoteMySQLName(table), limit)
}

func (Provider) Schema(ctx context.Context, session *adapter.Session) (any, error) {
	const q = `
SELECT table_schema, table_name, table_type
FROM information_schema.tables
WHERE table_schema NOT IN ('mysql', 'performance_schema', 'information_schema', 'sys')
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

func quoteMySQLName(name string) string {
	parts := strings.Split(name, ".")
	for i, part := range parts {
		parts[i] = "`" + strings.ReplaceAll(part, "`", "``") + "`"
	}
	return strings.Join(parts, ".")
}

type discardLogger struct{}

func (discardLogger) Print(v ...any) {}
