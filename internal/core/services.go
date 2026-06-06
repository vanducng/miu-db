package core

import (
	"context"
	"fmt"
	"time"

	"github.com/vanducng/miu-db/internal/activity"
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
	Logger    *activity.Logger
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
	return s.RunQueryWithMeta(ctx, name, sqlText, limit, nil, activity.CaptureMeta{})
}

// RunQueryWithSession resolves the connection, overlays validated per-call
// session context onto a clone of its Options (saved config untouched), then
// runs the query. A nil/empty session is identical to RunQuery.
func (s *Services) RunQueryWithSession(ctx context.Context, name string, sqlText string, limit int, session map[string]any) (config.Connection, query.Outcome, error) {
	return s.RunQueryWithMeta(ctx, name, sqlText, limit, session, activity.CaptureMeta{})
}

// RunQueryWithMeta is the canonical entry point; session may be nil.
func (s *Services) RunQueryWithMeta(ctx context.Context, name string, sqlText string, limit int, session map[string]any, meta activity.CaptureMeta) (config.Connection, query.Outcome, error) {
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
	outcome, err := s.RunQueryConnectionMeta(ctx, conn, sqlText, limit, meta)
	return conn, outcome, err
}

func (s *Services) RunQueryConnection(ctx context.Context, conn config.Connection, sqlText string, limit int) (query.Outcome, error) {
	return s.RunQueryConnectionMeta(ctx, conn, sqlText, limit, activity.CaptureMeta{})
}

func (s *Services) RunQueryConnectionMeta(ctx context.Context, conn config.Connection, sqlText string, limit int, meta activity.CaptureMeta) (query.Outcome, error) {
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()
	svc := query.Service{Registry: s.registry(), PageStore: s.pageStore(), Logger: s.Logger}
	return svc.Run(ctx, conn, sqlText, limit, meta)
}

func (s *Services) RunScript(ctx context.Context, name string, script string, limit int, opts adapter.ScriptOptions) (config.Connection, result.ScriptResult, error) {
	return s.RunScriptWithSession(ctx, name, script, limit, opts, nil)
}

// RunScriptWithSession resolves the connection, overlays validated per-call
// session context, then runs the multi-statement script. Mirrors
// RunQueryWithSession; rejection (unsupported datasource / session key) happens
// before any connection opens.
func (s *Services) RunScriptWithSession(ctx context.Context, name string, script string, limit int, opts adapter.ScriptOptions, session map[string]any) (config.Connection, result.ScriptResult, error) {
	conn, ok, err := s.FindConnection(name)
	if err != nil {
		return config.Connection{}, result.ScriptResult{}, err
	}
	if !ok {
		return config.Connection{}, result.ScriptResult{}, fmt.Errorf("connection %q not found", name)
	}
	if len(session) > 0 {
		provider, pok := s.registry().Get(conn.DBType)
		if !pok {
			return conn, result.ScriptResult{}, adapter.MissingProvider(conn.DBType)
		}
		conn, err = adapter.ApplySession(provider, conn, session)
		if err != nil {
			return conn, result.ScriptResult{}, err
		}
	}
	sr, err := s.RunScriptConnection(ctx, conn, script, limit, opts)
	return conn, sr, err
}

func (s *Services) RunScriptConnection(ctx context.Context, conn config.Connection, script string, limit int, opts adapter.ScriptOptions) (result.ScriptResult, error) {
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()
	service := query.Service{Registry: s.registry(), PageStore: s.pageStore()}
	return service.RunScript(ctx, conn, script, limit, opts)
}

func (s *Services) FetchPage(cursor string) (result.QueryPage, error) {
	svc := query.Service{Registry: s.registry(), PageStore: s.pageStore()}
	return svc.FetchPage(cursor)
}

func (s *Services) SchemaTree(ctx context.Context, name string) (config.Connection, any, error) {
	return s.SchemaTreeWithMeta(ctx, name, activity.CaptureMeta{})
}

func (s *Services) SchemaTreeWithMeta(ctx context.Context, name string, meta activity.CaptureMeta) (config.Connection, any, error) {
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

	start := time.Now()
	session, err := provider.Open(ctx, conn)
	if err != nil {
		s.emitSchemaEvent(conn, meta, start, err)
		return conn, nil, err
	}
	defer session.Close()
	data, schemaErr := provider.Schema(ctx, session)
	s.emitSchemaEvent(conn, meta, start, schemaErr)
	return conn, data, schemaErr
}

func (s *Services) emitSchemaEvent(conn config.Connection, meta activity.CaptureMeta, start time.Time, runErr error) {
	if s.Logger == nil || meta.SessionID == "" {
		return
	}
	defer func() { recover() }() //nolint:errcheck

	now := time.Now().UTC()
	ev := activity.Event{
		EventID:    activity.NewSessionID("ev"),
		SessionID:  meta.SessionID,
		Ts:         now.Format(time.RFC3339Nano),
		Source:     meta.Source,
		MCPClient:  meta.MCPClient,
		Op:         activity.OpSchema,
		Connection: conn.Name,
		Group:      conn.Group,
		DBType:     conn.DBType,
		LatencyMs:  time.Since(start).Milliseconds(),
	}
	if runErr != nil {
		ev.Error = &activity.EventError{
			Class:   "schema_error",
			Message: runErr.Error(),
		}
	}
	s.Logger.Log(ev)
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
