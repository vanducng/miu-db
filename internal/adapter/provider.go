package adapter

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/vanducng/miu-db/internal/config"
	"github.com/vanducng/miu-db/internal/result"
)

type Provider interface {
	Type() string
	Open(ctx context.Context, conn config.Connection) (*Session, error)
	Schema(ctx context.Context, session *Session) (any, error)
	BuildSelect(table string, limit int) string
}

type Queryer interface {
	Query(ctx context.Context, session *Session, query string, limit int) (result.QueryResult, [][]any, error)
}

type Execer interface {
	Exec(ctx context.Context, session *Session, query string) (int64, error)
}

// SessionConfigurable is implemented by providers that accept per-call session
// context keys (e.g. Snowflake role/warehouse). Returned keys must match the
// conn.Options key names the provider's Open/Query already reads.
type SessionConfigurable interface {
	SessionKeys() []string
}

type Registry struct {
	providers map[string]Provider
}

func NewRegistry() *Registry {
	return &Registry{providers: map[string]Provider{}}
}

func (r *Registry) Register(provider Provider) {
	r.providers[provider.Type()] = provider
}

func (r *Registry) Get(dbType string) (Provider, bool) {
	provider, ok := r.providers[dbType]
	return provider, ok
}

type Session struct {
	DB       *sql.DB
	Closer   func() error
	Provider Provider
	Config   config.Connection
}

func (s *Session) Close() error {
	var first error
	if s.DB != nil {
		first = s.DB.Close()
	}
	if s.Closer != nil {
		if err := s.Closer(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func QuerySQL(ctx context.Context, db *sql.DB, query string, limit int) (result.QueryResult, [][]any, error) {
	if limit <= 0 {
		limit = 100
	}
	queryCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	rows, err := db.QueryContext(queryCtx, query)
	if err != nil {
		return result.QueryResult{}, nil, err
	}
	defer rows.Close()

	colNames, err := rows.Columns()
	if err != nil {
		return result.QueryResult{}, nil, err
	}
	colTypes, _ := rows.ColumnTypes()
	columns := make([]result.Column, len(colNames))
	for i, name := range colNames {
		typ := ""
		nullable := false
		if i < len(colTypes) {
			typ = colTypes[i].DatabaseTypeName()
			if n, ok := colTypes[i].Nullable(); ok {
				nullable = n
			}
		}
		columns[i] = result.Column{Name: name, Type: typ, Nullable: nullable}
	}

	outRows := make([][]any, 0, limit)
	remaining := [][]any{}
	count := 0
	for rows.Next() {
		values := make([]any, len(columns))
		scan := make([]any, len(columns))
		for i := range values {
			scan[i] = &values[i]
		}
		if err := rows.Scan(scan...); err != nil {
			return result.QueryResult{}, nil, err
		}
		normalized := normalizeRow(values)
		if count < limit {
			outRows = append(outRows, normalized)
		} else {
			remaining = append(remaining, normalized)
			break
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return result.QueryResult{}, nil, err
	}
	return result.QueryResult{
		Columns:   columns,
		Rows:      outRows,
		Truncated: len(remaining) > 0,
	}, remaining, nil
}

func ExecSQL(ctx context.Context, db *sql.DB, query string) (int64, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	res, err := db.ExecContext(queryCtx, query)
	if err != nil {
		return 0, err
	}
	affected, _ := res.RowsAffected()
	return affected, nil
}

func normalizeRow(values []any) []any {
	out := make([]any, len(values))
	for i, value := range values {
		switch v := value.(type) {
		case []byte:
			out[i] = string(v)
		case time.Time:
			out[i] = v.Format(time.RFC3339Nano)
		default:
			out[i] = v
		}
	}
	return out
}

func IsReturningRows(query string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(query))
	if trimmed == "" {
		return false
	}
	for _, prefix := range []string{"select", "with", "show", "describe", "desc", "explain", "pragma"} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

func MissingProvider(dbType string) error {
	return fmt.Errorf("unsupported database type %q", dbType)
}
