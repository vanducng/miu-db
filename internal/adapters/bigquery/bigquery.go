package bigquery

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	gcbq "cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/vanducng/miu-db/internal/adapter"
	"github.com/vanducng/miu-db/internal/config"
	"github.com/vanducng/miu-db/internal/result"
	"github.com/vanducng/miu-db/internal/schema"
)

type Provider struct{}

func New() Provider { return Provider{} }

func (Provider) Type() string { return "bigquery" }

func (p Provider) Open(ctx context.Context, conn config.Connection) (*adapter.Session, error) {
	project := conn.Endpoint.Host
	if project == "" {
		return nil, fmt.Errorf("bigquery project is required")
	}
	opts := []option.ClientOption{}
	if path, ok := conn.Options["bigquery_credentials_path"].(string); ok && path != "" {
		opts = append(opts, option.WithCredentialsFile(expandPath(path)))
	}
	client, err := gcbq.NewClient(ctx, project, opts...)
	if err != nil {
		return nil, err
	}
	q := client.Query("SELECT 1")
	if loc, ok := conn.Options["bigquery_location"].(string); ok {
		q.Location = loc
	}
	it, err := q.Read(ctx)
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	var values []gcbq.Value
	if err := it.Next(&values); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &adapter.Session{
		Provider: p,
		Config:   conn,
		Closer:   client.Close,
	}, nil
}

func (Provider) BuildSelect(table string, limit int) string {
	return fmt.Sprintf("SELECT * FROM `%s` LIMIT %d", strings.Trim(table, "`"), limit)
}

func (Provider) Query(ctx context.Context, session *adapter.Session, query string, limit int) (result.QueryResult, [][]any, error) {
	client, err := openClient(ctx, session.Config)
	if err != nil {
		return result.QueryResult{}, nil, err
	}
	defer client.Close()
	q := client.Query(query)
	if loc, ok := session.Config.Options["bigquery_location"].(string); ok {
		q.Location = loc
	}
	it, err := q.Read(ctx)
	if err != nil {
		return result.QueryResult{}, nil, err
	}
	columns := make([]result.Column, len(it.Schema))
	for i, field := range it.Schema {
		columns[i] = result.Column{Name: field.Name, Type: string(field.Type), Nullable: !field.Required}
	}
	rows := [][]any{}
	remaining := [][]any{}
	count := 0
	for {
		var values []gcbq.Value
		err := it.Next(&values)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return result.QueryResult{}, nil, err
		}
		row := make([]any, len(values))
		for i, value := range values {
			if t, ok := value.(time.Time); ok {
				row[i] = t.Format(time.RFC3339Nano)
			} else {
				row[i] = value
			}
		}
		if count < limit {
			rows = append(rows, row)
		} else {
			remaining = append(remaining, row)
			break
		}
		count++
	}
	return result.QueryResult{Columns: columns, Rows: rows, Truncated: len(remaining) > 0}, remaining, nil
}

func (Provider) Exec(ctx context.Context, session *adapter.Session, query string) (int64, error) {
	_, _, err := New().Query(ctx, session, query, 1)
	return 0, err
}

func (Provider) Schema(ctx context.Context, session *adapter.Session) (any, error) {
	client, err := openClient(ctx, session.Config)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	tree := schema.Tree{Tables: []schema.Table{}}
	it := client.Datasets(ctx)
	for {
		ds, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		tables := ds.Tables(ctx)
		for {
			tbl, err := tables.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return nil, err
			}
			tree.Tables = append(tree.Tables, schema.Table{Schema: ds.DatasetID, Name: tbl.TableID, Type: "table"})
		}
	}
	return tree, nil
}

func openClient(ctx context.Context, conn config.Connection) (*gcbq.Client, error) {
	project := conn.Endpoint.Host
	opts := []option.ClientOption{}
	if path, ok := conn.Options["bigquery_credentials_path"].(string); ok && path != "" {
		opts = append(opts, option.WithCredentialsFile(expandPath(path)))
	}
	return gcbq.NewClient(ctx, project, opts...)
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
