package query

import (
	"context"
	"fmt"

	"github.com/vanducng/miu-db/internal/adapter"
	"github.com/vanducng/miu-db/internal/config"
	"github.com/vanducng/miu-db/internal/result"
)

type Service struct {
	Registry  *adapter.Registry
	PageStore *result.PageStore
}

type Outcome struct {
	Result       any    `json:"result"`
	NextCursor   string `json:"next_cursor,omitempty"`
	RowsAffected int64  `json:"rows_affected,omitempty"`
}

func (s *Service) Run(ctx context.Context, conn config.Connection, sql string, limit int) (Outcome, error) {
	provider, ok := s.Registry.Get(conn.DBType)
	if !ok {
		return Outcome{}, adapter.MissingProvider(conn.DBType)
	}
	session, err := provider.Open(ctx, conn)
	if err != nil {
		return Outcome{}, err
	}
	defer session.Close()

	if adapter.IsReturningRows(sql) {
		var qr result.QueryResult
		var remaining [][]any
		var err error
		if custom, ok := provider.(adapter.Queryer); ok {
			qr, remaining, err = custom.Query(ctx, session, sql, limit)
		} else {
			qr, remaining, err = adapter.QuerySQL(ctx, session.DB, sql, limit)
		}
		if err != nil {
			return Outcome{}, err
		}
		cursor, err := s.PageStore.Save(qr.Columns, remaining, limit)
		if err != nil {
			return Outcome{}, err
		}
		return Outcome{Result: qr, NextCursor: cursor}, nil
	}
	var affected int64
	if custom, ok := provider.(adapter.Execer); ok {
		affected, err = custom.Exec(ctx, session, sql)
	} else {
		affected, err = adapter.ExecSQL(ctx, session.DB, sql)
	}
	if err != nil {
		return Outcome{}, err
	}
	return Outcome{Result: map[string]any{"rows_affected": affected}, RowsAffected: affected}, nil
}

func (s *Service) FetchPage(cursor string) (result.QueryPage, error) {
	if cursor == "" {
		return result.QueryPage{}, fmt.Errorf("missing cursor")
	}
	return s.PageStore.Fetch(cursor)
}
