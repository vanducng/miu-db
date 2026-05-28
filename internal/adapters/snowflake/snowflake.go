package snowflake

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"fmt"
	"os"
	"strings"

	"github.com/snowflakedb/gosnowflake"
	_ "github.com/snowflakedb/gosnowflake"

	"github.com/vanducng/miu-db/internal/adapter"
	"github.com/vanducng/miu-db/internal/config"
	"github.com/vanducng/miu-db/internal/schema"
)

type Provider struct{}

func New() Provider { return Provider{} }

func (Provider) Type() string { return "snowflake" }

func (p Provider) Open(ctx context.Context, conn config.Connection) (*adapter.Session, error) {
	cfg := &gosnowflake.Config{
		Account:          conn.Endpoint.Host,
		User:             conn.Endpoint.Username,
		Password:         conn.Endpoint.Password,
		Database:         conn.Endpoint.Database,
		Application:      "miudb",
		DisableTelemetry: true,
	}
	if v, ok := conn.Options["warehouse"].(string); ok {
		cfg.Warehouse = v
	}
	if v, ok := conn.Options["schema"].(string); ok {
		cfg.Schema = v
	}
	if v, ok := conn.Options["role"].(string); ok {
		cfg.Role = v
	}
	if auth, ok := conn.Options["authenticator"].(string); ok && strings.EqualFold(auth, "snowflake_jwt") {
		cfg.Authenticator = gosnowflake.AuthTypeJwt
		keyPath, _ := conn.Options["private_key_file"].(string)
		key, err := loadPrivateKey(keyPath)
		if err != nil {
			return nil, err
		}
		cfg.PrivateKey = key
	}
	dsn, err := gosnowflake.DSN(cfg)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("snowflake", dsn)
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
	return fmt.Sprintf("SELECT * FROM %s LIMIT %d", quoteSnowflakeName(table), limit)
}

func (Provider) Schema(ctx context.Context, session *adapter.Session) (any, error) {
	rows, err := session.DB.QueryContext(ctx, "SHOW TABLES")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns, _ := rows.Columns()
	tree := schema.Tree{Tables: []schema.Table{}}
	for rows.Next() {
		values := make([]any, len(columns))
		scan := make([]any, len(columns))
		for i := range values {
			scan[i] = &values[i]
		}
		if err := rows.Scan(scan...); err != nil {
			return nil, err
		}
		row := map[string]string{}
		for i, col := range columns {
			if b, ok := values[i].([]byte); ok {
				row[strings.ToLower(col)] = string(b)
			} else if values[i] != nil {
				row[strings.ToLower(col)] = fmt.Sprint(values[i])
			}
		}
		name := row["name"]
		if name != "" {
			tree.Tables = append(tree.Tables, schema.Table{Schema: row["schema_name"], Name: name, Type: "table"})
		}
	}
	return tree, rows.Err()
}

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	if path == "" {
		return nil, fmt.Errorf("snowflake_jwt requires private_key_file")
	}
	data, err := os.ReadFile(expandPath(path))
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("private key file is not PEM")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not RSA")
	}
	return rsaKey, nil
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return home + "/" + strings.TrimPrefix(path, "~/")
		}
	}
	return path
}

func quoteSnowflakeName(name string) string {
	parts := strings.Split(name, ".")
	for i, part := range parts {
		parts[i] = `"` + strings.ReplaceAll(part, `"`, `""`) + `"`
	}
	return strings.Join(parts, ".")
}
