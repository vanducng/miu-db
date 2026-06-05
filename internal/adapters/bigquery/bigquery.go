package bigquery

import (
	"context"
	"fmt"
	"os"
	"strconv"
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

// SessionKeys are the per-call --session keys BigQuery accepts.
func (Provider) SessionKeys() []string {
	return []string{"bigquery_location", "bigquery_maximum_bytes_billed"}
}

// clientOptions builds the google API client options from conn.
// Honors auth_method (adc|service_account) and bigquery_quota_project.
// Used by Open and openClient only; Query operates on an existing session.
func clientOptions(conn config.Connection) ([]option.ClientOption, error) {
	authMethod, _ := conn.Options["auth_method"].(string)
	credPath, _ := conn.Options["bigquery_credentials_path"].(string)

	var opts []option.ClientOption

	switch authMethod {
	case "", "adc":
		if credPath != "" {
			opts = append(opts, option.WithAuthCredentialsFile(option.ServiceAccount, expandPath(credPath)))
		}
		// else: rely on ADC (GOOGLE_APPLICATION_CREDENTIALS or gcloud default)
	case "service_account":
		if credPath == "" {
			return nil, fmt.Errorf("bigquery auth_method=service_account requires bigquery_credentials_path")
		}
		opts = append(opts, option.WithAuthCredentialsFile(option.ServiceAccount, expandPath(credPath)))
	default:
		return nil, fmt.Errorf("bigquery auth_method %q unknown; use \"adc\" or \"service_account\"", authMethod)
	}

	if qp, _ := conn.Options["bigquery_quota_project"].(string); qp != "" {
		opts = append(opts, option.WithQuotaProject(qp))
	}

	return opts, nil
}

// wrapCredentialError attaches an actionable hint when BigQuery returns a
// missing-ADC error so the user knows exactly how to fix it.
func wrapCredentialError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "could not find default credentials") ||
		strings.Contains(msg, "application default credentials") ||
		strings.Contains(msg, "Application Default Credentials") {
		return fmt.Errorf("BigQuery: no credentials found. run `gcloud auth application-default login`, or set bigquery_credentials_path / GOOGLE_APPLICATION_CREDENTIALS: %w", err)
	}
	return err
}

func applySession(q *gcbq.Query, opts map[string]any) error {
	if loc, ok := opts["bigquery_location"].(string); ok && loc != "" {
		q.Location = loc
	}
	if raw, ok := opts["bigquery_maximum_bytes_billed"]; ok {
		n, err := toInt64(raw)
		if err != nil {
			return fmt.Errorf("bigquery_maximum_bytes_billed: %w", err)
		}
		q.MaxBytesBilled = n
	}
	return nil
}

func toInt64(v any) (int64, error) {
	switch t := v.(type) {
	case string:
		return strconv.ParseInt(strings.TrimSpace(t), 10, 64)
	case int:
		return int64(t), nil
	case int64:
		return t, nil
	case float64:
		return int64(t), nil
	default:
		return 0, fmt.Errorf("expected integer, got %T", v)
	}
}

func (p Provider) Open(ctx context.Context, conn config.Connection) (*adapter.Session, error) {
	project := conn.Endpoint.Host
	if project == "" {
		return nil, fmt.Errorf("bigquery project is required")
	}
	opts, err := clientOptions(conn)
	if err != nil {
		return nil, err
	}
	client, err := gcbq.NewClient(ctx, project, opts...)
	if err != nil {
		return nil, wrapCredentialError(err)
	}
	q := client.Query("SELECT 1")
	if err := applySession(q, conn.Options); err != nil {
		_ = client.Close()
		return nil, err
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
	if err := applySession(q, session.Config.Options); err != nil {
		return result.QueryResult{}, nil, err
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
	opts, err := clientOptions(conn)
	if err != nil {
		return nil, err
	}
	client, err := gcbq.NewClient(ctx, project, opts...)
	if err != nil {
		return nil, wrapCredentialError(err)
	}
	return client, nil
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
