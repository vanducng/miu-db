package core

import (
	"context"
	"fmt"
	"time"

	"github.com/vanducng/miu-db/internal/adapter"
	"github.com/vanducng/miu-db/internal/adapters/bigquery"
	"github.com/vanducng/miu-db/internal/adapters/mysql"
	"github.com/vanducng/miu-db/internal/adapters/postgres"
	"github.com/vanducng/miu-db/internal/adapters/snowflake"
	"github.com/vanducng/miu-db/internal/adapters/sqlite"
	"github.com/vanducng/miu-db/internal/config"
	"github.com/vanducng/miu-db/internal/query"
	"github.com/vanducng/miu-db/internal/result"
)

type Services struct {
	Store     *config.Store
	Registry  *adapter.Registry
	PageStore *result.PageStore
	Timeout   time.Duration
}

func NewServices(store *config.Store, timeout time.Duration) *Services {
	return &Services{
		Store:     store,
		Registry:  NewRegistry(),
		PageStore: result.NewPageStore(""),
		Timeout:   timeout,
	}
}

func NewRegistry() *adapter.Registry {
	reg := adapter.NewRegistry()
	reg.Register(sqlite.New())
	reg.Register(postgres.New())
	reg.Register(mysql.New())
	reg.Register(snowflake.New())
	reg.Register(bigquery.New())
	return reg
}

func (s *Services) Connections() []config.Connection {
	if s == nil || s.Store == nil {
		return []config.Connection{}
	}
	return s.Store.Connections()
}

func (s *Services) FindConnection(name string) (config.Connection, bool, error) {
	if s == nil || s.Store == nil {
		return config.Connection{}, false, fmt.Errorf("connection store is not configured")
	}
	return s.Store.FindResolved(name)
}

func (s *Services) RunQuery(ctx context.Context, name string, sqlText string, limit int) (config.Connection, query.Outcome, error) {
	return s.RunQueryWithSession(ctx, name, sqlText, limit, nil)
}

// RunQueryWithSession resolves the connection, overlays validated per-call
// session context onto a clone of its Options (saved config untouched), then
// runs the query. A nil/empty session is identical to RunQuery.
func (s *Services) RunQueryWithSession(ctx context.Context, name string, sqlText string, limit int, session map[string]any) (config.Connection, query.Outcome, error) {
	conn, ok, err := s.FindConnection(name)
	if err != nil {
		return config.Connection{}, query.Outcome{}, err
	}
	if !ok {
		return config.Connection{}, query.Outcome{}, fmt.Errorf("connection %q not found", name)
	}
	if len(session) > 0 {
		provider, pok := s.registry().Get(conn.DBType)
		if !pok {
			return conn, query.Outcome{}, adapter.MissingProvider(conn.DBType)
		}
		conn, err = adapter.ApplySession(provider, conn, session)
		if err != nil {
			return conn, query.Outcome{}, err
		}
	}
	outcome, err := s.RunQueryConnection(ctx, conn, sqlText, limit)
	return conn, outcome, err
}

func (s *Services) RunQueryConnection(ctx context.Context, conn config.Connection, sqlText string, limit int) (query.Outcome, error) {
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()
	service := query.Service{Registry: s.registry(), PageStore: s.pageStore()}
	return service.Run(ctx, conn, sqlText, limit)
}

func (s *Services) FetchPage(cursor string) (result.QueryPage, error) {
	service := query.Service{Registry: s.registry(), PageStore: s.pageStore()}
	return service.FetchPage(cursor)
}

func (s *Services) SchemaTree(ctx context.Context, name string) (config.Connection, any, error) {
	conn, ok, err := s.FindConnection(name)
	if err != nil {
		return config.Connection{}, nil, err
	}
	if !ok {
		return config.Connection{}, nil, fmt.Errorf("connection %q not found", name)
	}
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()
	provider, ok := s.registry().Get(conn.DBType)
	if !ok {
		return conn, nil, adapter.MissingProvider(conn.DBType)
	}
	session, err := provider.Open(ctx, conn)
	if err != nil {
		return conn, nil, err
	}
	defer session.Close()
	data, err := provider.Schema(ctx, session)
	return conn, data, err
}

func (s *Services) TestConnection(ctx context.Context, name string) (config.Connection, error) {
	conn, ok, err := s.FindConnection(name)
	if err != nil {
		return config.Connection{}, err
	}
	if !ok {
		return config.Connection{}, fmt.Errorf("connection %q not found", name)
	}
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()
	provider, ok := s.registry().Get(conn.DBType)
	if !ok {
		return conn, adapter.MissingProvider(conn.DBType)
	}
	session, err := provider.Open(ctx, conn)
	if err != nil {
		return conn, err
	}
	defer session.Close()
	return conn, nil
}

func (s *Services) registry() *adapter.Registry {
	if s != nil && s.Registry != nil {
		return s.Registry
	}
	return NewRegistry()
}

func (s *Services) pageStore() *result.PageStore {
	if s != nil && s.PageStore != nil {
		return s.PageStore
	}
	return result.NewPageStore("")
}

func (s *Services) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if s == nil || s.Timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, s.Timeout)
}
