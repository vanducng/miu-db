package query

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/vanducng/miu-db/internal/activity"
	"github.com/vanducng/miu-db/internal/adapter"
	"github.com/vanducng/miu-db/internal/config"
	"github.com/vanducng/miu-db/internal/result"
)

type Service struct {
	Registry  *adapter.Registry
	PageStore *result.PageStore
	Logger    *activity.Logger
}

type Outcome struct {
	Result       any    `json:"result"`
	NextCursor   string `json:"next_cursor,omitempty"`
	RowsAffected int64  `json:"rows_affected,omitempty"`
}

func (s *Service) Run(ctx context.Context, conn config.Connection, sql string, limit int, meta activity.CaptureMeta) (Outcome, error) {
	start := time.Now()
	provider, ok := s.Registry.Get(conn.DBType)
	if !ok {
		return Outcome{}, adapter.MissingProvider(conn.DBType)
	}
	session, err := provider.Open(ctx, conn)
	if err != nil {
		return Outcome{}, err
	}
	defer session.Close()

	var outcome Outcome
	var runErr error

	if adapter.IsReturningRows(sql) {
		var qr result.QueryResult
		var remaining [][]any
		if custom, ok := provider.(adapter.Queryer); ok {
			qr, remaining, runErr = custom.Query(ctx, session, sql, limit)
		} else {
			qr, remaining, runErr = adapter.QuerySQL(ctx, session.DB, sql, limit)
		}
		if runErr == nil {
			var cursor string
			cursor, runErr = s.PageStore.Save(qr.Columns, remaining, limit)
			if runErr == nil {
				outcome = Outcome{Result: qr, NextCursor: cursor}
				s.emitEvent(conn, sql, limit, meta, start, len(qr.Rows), 0, cursor != "", runErr)
				return outcome, nil
			}
		}
		s.emitEvent(conn, sql, limit, meta, start, 0, 0, false, runErr)
		return Outcome{}, runErr
	}

	var affected int64
	if custom, ok := provider.(adapter.Execer); ok {
		affected, runErr = custom.Exec(ctx, session, sql)
	} else {
		affected, runErr = adapter.ExecSQL(ctx, session.DB, sql)
	}
	if runErr != nil {
		s.emitEvent(conn, sql, limit, meta, start, 0, 0, false, runErr)
		return Outcome{}, runErr
	}
	outcome = Outcome{Result: map[string]any{"rows_affected": affected}, RowsAffected: affected}
	s.emitEvent(conn, sql, limit, meta, start, 0, affected, false, nil)
	return outcome, nil
}

// emitEvent logs an activity event best-effort; never affects the query result.
func (s *Service) emitEvent(
	conn config.Connection,
	sql string,
	limit int,
	meta activity.CaptureMeta,
	start time.Time,
	rowsReturned int,
	rowsAffected int64,
	nextCursorIssued bool,
	runErr error,
) {
	if s.Logger == nil || meta.SessionID == "" {
		return
	}
	defer func() { recover() }() //nolint:errcheck

	now := time.Now().UTC()
	op := activity.OpQuery
	if !adapter.IsReturningRows(sql) {
		op = activity.OpExec
	}

	rawSQL := sql
	if conn.LogSQL != nil && !*conn.LogSQL {
		rawSQL = ""
	}

	ev := activity.Event{
		EventID:          activity.NewSessionID("ev"),
		SessionID:        meta.SessionID,
		Ts:               now.Format(time.RFC3339Nano),
		Source:           meta.Source,
		MCPClient:        meta.MCPClient,
		Op:               op,
		Connection:       conn.Name,
		Group:            conn.Group,
		DBType:           conn.DBType,
		SQL:              rawSQL,
		SQLShape:         activity.Shape(sql),
		Limit:            limit,
		LatencyMs:        time.Since(start).Milliseconds(),
		RowsReturned:     rowsReturned,
		RowsAffected:     rowsAffected,
		NextCursorIssued: nextCursorIssued,
	}
	if runErr != nil {
		ev.Error = &activity.EventError{
			Class:   errorClass(runErr),
			Message: runErr.Error(),
		}
	}
	s.Logger.Log(ev)
}

func errorClass(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	for _, pair := range []struct{ substr, class string }{
		{"connection refused", "connection_refused"},
		{"timeout", "timeout"},
		{"deadline exceeded", "timeout"},
		{"authentication", "auth"},
		{"access denied", "auth"},
		{"permission denied", "auth"},
		{"no such host", "dns"},
		{"certificate", "tls"},
		{"x509:", "tls"},
	} {
		if strings.Contains(msg, pair.substr) {
			return pair.class
		}
	}
	return "query_error"
}

// RunScript executes a multi-statement script. Strict ScriptRunner opt-in:
// providers that do not implement it are rejected BEFORE opening a connection.
func (s *Service) RunScript(ctx context.Context, conn config.Connection, script string, limit int, opts adapter.ScriptOptions) (result.ScriptResult, error) {
	provider, ok := s.Registry.Get(conn.DBType)
	if !ok {
		return result.ScriptResult{}, adapter.MissingProvider(conn.DBType)
	}
	runner, err := adapter.ResolveScriptRunner(provider)
	if err != nil {
		return result.ScriptResult{}, err
	}
	// The runner owns its connection lifecycle (no pre-open) — rejection above
	// still happens before any connection is touched.
	return runner.RunScript(ctx, conn, script, limit, opts)
}

func (s *Service) FetchPage(cursor string) (result.QueryPage, error) {
	if cursor == "" {
		return result.QueryPage{}, fmt.Errorf("missing cursor")
	}
	return s.PageStore.Fetch(cursor)
}
