package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/vanducng/miu-db/internal/adapter"
	"github.com/vanducng/miu-db/internal/adapters/bigquery"
	"github.com/vanducng/miu-db/internal/adapters/mysql"
	"github.com/vanducng/miu-db/internal/adapters/postgres"
	"github.com/vanducng/miu-db/internal/adapters/snowflake"
	"github.com/vanducng/miu-db/internal/adapters/sqlite"
	"github.com/vanducng/miu-db/internal/config"
	"github.com/vanducng/miu-db/internal/protocol"
	"github.com/vanducng/miu-db/internal/query"
	"github.com/vanducng/miu-db/internal/result"
)

type options struct {
	output          string
	configDir       string
	credentialsPath string
	limit           int
	timeout         time.Duration
}

type commandInfo struct {
	Name        string   `json:"name"`
	Summary     string   `json:"summary"`
	Description string   `json:"description,omitempty"`
	Stability   string   `json:"stability"`
	Mutates     bool     `json:"mutates"`
	SideEffects []string `json:"side_effects"`
	Examples    []string `json:"examples"`
}

var version = "v0.2.0-go.3-dev"

func Execute(args []string) error {
	opts := &options{output: "json", limit: 100, timeout: 30 * time.Second}
	root := rootCommand(opts)
	root.SetArgs(args)
	root.SilenceUsage = true
	root.SilenceErrors = true
	if err := root.Execute(); err != nil {
		_ = writeError(os.Stdout, commandPath(args), err)
		return err
	}
	return nil
}

func rootCommand(opts *options) *cobra.Command {
	root := &cobra.Command{
		Use:   "miudb",
		Short: "Fast local database CLI for agents and Neovim",
	}
	root.PersistentFlags().StringVar(&opts.output, "output", "json", "Output format: json")
	root.PersistentFlags().StringVar(&opts.configDir, "config-dir", config.DefaultConfigDir(), "Config directory")
	root.PersistentFlags().StringVar(&opts.credentialsPath, "credentials-export", "", "Credentials export path")
	root.PersistentFlags().IntVar(&opts.limit, "limit", 100, "Maximum rows returned inline")
	root.PersistentFlags().DurationVar(&opts.timeout, "timeout", 30*time.Second, "Connection/query timeout")
	root.AddCommand(versionCommand())
	root.AddCommand(commandsCommand(opts))
	root.AddCommand(describeCommand(opts))
	root.AddCommand(connectionsCommand(opts))
	root.AddCommand(queryCommand(opts))
	root.AddCommand(schemaCommand(opts))
	root.AddCommand(serveCommand(opts))
	return root
}

func versionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeSuccess(cmd.OutOrStdout(), "version", "version", map[string]any{"version": versionString()}, nil)
		},
	}
}

func versionString() string {
	info, ok := debug.ReadBuildInfo()
	if ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

func commandsCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "commands",
		Short: "List command catalog",
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeSuccess(cmd.OutOrStdout(), "commands", "command.catalog", map[string]any{"commands": catalog()}, map[string]any{"count": len(catalog())})
		},
	}
}

func describeCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "describe <command>",
		Short: "Describe one command for agents",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.Join(args, " ")
			for _, info := range catalog() {
				if info.Name == name {
					return writeSuccess(cmd.OutOrStdout(), "describe", "command.description", info, nil)
				}
			}
			return &CLIError{Code: "command.not_found", Message: "command not found", Exit: 2}
		},
	}
}

func connectionsCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "connections", Short: "Manage connections"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List saved connections",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := loadStore(opts)
			if err != nil {
				return err
			}
			items := []any{}
			byType := map[string]int{}
			for _, conn := range store.Connections() {
				items = append(items, config.RedactedConnection(conn))
				byType[conn.DBType]++
			}
			return writeSuccess(cmd.OutOrStdout(), "connections list", "connection.list", map[string]any{"connections": items}, map[string]any{"count": len(items), "by_type": byType})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "test <name>",
		Short: "Test a connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := loadStore(opts)
			if err != nil {
				return err
			}
			conn, ok := store.Find(args[0])
			if !ok {
				return &CLIError{Code: "connection.not_found", Message: "connection not found", Exit: 2}
			}
			ctx, cancel := context.WithTimeout(context.Background(), opts.timeout)
			defer cancel()
			provider, ok := registry().Get(conn.DBType)
			if !ok {
				return &CLIError{Code: "adapter.unsupported", Message: adapter.MissingProvider(conn.DBType).Error(), Exit: 2}
			}
			session, err := provider.Open(ctx, conn)
			if err != nil {
				return err
			}
			defer session.Close()
			return writeSuccess(cmd.OutOrStdout(), "connections test", "connection.test", map[string]any{"name": conn.Name, "ok": true}, map[string]any{"db_type": conn.DBType})
		},
	})
	cmd.AddCommand(connectionsSmokeCommand(opts))
	return cmd
}

func connectionsSmokeCommand(opts *options) *cobra.Command {
	var sqlText string
	var names []string
	var concurrency int
	smoke := &cobra.Command{
		Use:   "smoke",
		Short: "Run a smoke query across saved connections",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := loadStore(opts)
			if err != nil {
				return err
			}
			conns := selectConnections(store.Connections(), names)
			if len(conns) == 0 {
				return &CLIError{Code: "connection.not_found", Message: "no matching connections", Exit: 2}
			}
			if concurrency <= 0 {
				concurrency = 4
			}
			if concurrency > len(conns) {
				concurrency = len(conns)
			}

			results := make([]smokeResult, len(conns))
			jobs := make(chan int)
			var wg sync.WaitGroup
			reg := registry()
			pageStore := result.NewPageStore("")
			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for idx := range jobs {
						results[idx] = runSmoke(cmd.Context(), reg, pageStore, conns[idx], sqlText, opts.limit, opts.timeout)
					}
				}()
			}
			for idx := range conns {
				jobs <- idx
			}
			close(jobs)
			wg.Wait()

			byType := map[string]map[string]int{}
			passed := 0
			for _, res := range results {
				if byType[res.DBType] == nil {
					byType[res.DBType] = map[string]int{"passed": 0, "failed": 0}
				}
				if res.OK {
					passed++
					byType[res.DBType]["passed"]++
				} else {
					byType[res.DBType]["failed"]++
				}
			}
			failed := len(results) - passed
			return writeJSON(cmd.OutOrStdout(), Envelope{
				OK:      failed == 0,
				Kind:    "connection.smoke",
				Command: "connections smoke",
				Summary: map[string]any{
					"count":       len(results),
					"passed":      passed,
					"failed":      failed,
					"by_type":     byType,
					"timeout":     opts.timeout.String(),
					"concurrency": concurrency,
				},
				Data:      map[string]any{"results": results},
				Artifacts: []any{},
				Warnings:  []any{},
			})
		},
	}
	smoke.Flags().StringArrayVar(&names, "connection", nil, "Connection name to test; repeat to test a subset")
	smoke.Flags().StringVar(&sqlText, "sql", "select 1 as one", "Smoke SQL to run")
	smoke.Flags().IntVar(&concurrency, "concurrency", 4, "Maximum concurrent connection tests")
	return smoke
}

type smokeResult struct {
	Name       string     `json:"name"`
	DBType     string     `json:"db_type"`
	OK         bool       `json:"ok"`
	DurationMS int64      `json:"duration_ms"`
	Rows       int        `json:"rows,omitempty"`
	Error      *ErrorInfo `json:"error,omitempty"`
}

func selectConnections(conns []config.Connection, names []string) []config.Connection {
	if len(names) == 0 {
		return conns
	}
	wanted := map[string]bool{}
	for _, name := range names {
		wanted[name] = true
	}
	out := []config.Connection{}
	for _, conn := range conns {
		if wanted[conn.Name] {
			out = append(out, conn)
		}
	}
	return out
}

func runSmoke(parent context.Context, reg *adapter.Registry, pageStore *result.PageStore, conn config.Connection, sqlText string, limit int, timeout time.Duration) smokeResult {
	start := time.Now()
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	service := query.Service{Registry: reg, PageStore: pageStore}
	outcome, err := service.Run(ctx, conn, sqlText, limit)
	res := smokeResult{Name: conn.Name, DBType: conn.DBType, DurationMS: time.Since(start).Milliseconds()}
	if err != nil {
		info := errorInfo(err)
		if ctx.Err() != nil {
			info.Code = "connection.timeout"
			info.Retryable = true
			info.SafeToRetry = true
		}
		res.Error = &info
		return res
	}
	res.OK = true
	if qr, ok := outcome.Result.(result.QueryResult); ok {
		res.Rows = len(qr.Rows)
	}
	return res
}

func errorInfo(err error) ErrorInfo {
	message := err.Error()
	lower := strings.ToLower(message)
	info := ErrorInfo{Code: "internal.error", Message: message, Retryable: false}
	var cliErr *CLIError
	if strings.Contains(lower, "connection refused") {
		info.Code = "connection.refused"
		info.Retryable = true
		info.SafeToRetry = true
	}
	if strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline exceeded") {
		info.Code = "connection.timeout"
		info.Retryable = true
		info.SafeToRetry = true
	}
	if strings.Contains(lower, "unsupported database type") {
		info.Code = "adapter.unsupported"
	}
	if strings.Contains(lower, "invalid connection") {
		info.Code = "connection.invalid"
		info.Retryable = true
	}
	if strings.Contains(lower, "lost connection") {
		info.Code = "connection.lost"
		info.Retryable = true
	}
	if strings.Contains(lower, "x509:") {
		info.Code = "connection.tls"
	}
	if strings.Contains(lower, "ssh") {
		info.Code = "tunnel.error"
		info.Retryable = true
	}
	if strings.Contains(lower, "context canceled") {
		info.Code = "connection.cancelled"
		info.Retryable = true
		info.SafeToRetry = true
	}
	if strings.Contains(lower, "context deadline exceeded") {
		info.Code = "connection.timeout"
		info.Retryable = true
		info.SafeToRetry = true
	}
	if strings.Contains(lower, "no such host") {
		info.Code = "connection.dns"
		info.Retryable = true
	}
	if strings.Contains(lower, "access denied") {
		info.Code = "connection.auth"
	}
	if strings.Contains(lower, "authentication") {
		info.Code = "connection.auth"
	}
	if strings.Contains(lower, "permission denied") {
		info.Code = "connection.auth"
	}
	if strings.Contains(lower, "certificate") {
		info.Code = "connection.tls"
	}
	if strings.Contains(lower, "handshake") {
		info.Code = "connection.handshake"
		info.Retryable = true
	}
	if errors.As(err, &cliErr) {
		info.Code = cliErr.Code
		info.Message = cliErr.Message
		info.Hint = cliErr.Hint
		info.Details = cliErr.Details
		info.Retryable = cliErr.Retry
		info.SafeToRetry = cliErr.SafeRetry
	}
	return info
}

func queryCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "query", Short: "Run queries"}
	var connectionName, sqlText, cursor string
	run := &cobra.Command{
		Use:   "run",
		Short: "Run SQL",
		RunE: func(cmd *cobra.Command, args []string) error {
			if connectionName == "" || sqlText == "" {
				return &CLIError{Code: "query.missing_input", Message: "connection and sql are required", Exit: 2}
			}
			store, err := loadStore(opts)
			if err != nil {
				return err
			}
			conn, ok := store.Find(connectionName)
			if !ok {
				return &CLIError{Code: "connection.not_found", Message: "connection not found", Exit: 2}
			}
			service := query.Service{Registry: registry(), PageStore: result.NewPageStore("")}
			ctx, cancel := context.WithTimeout(context.Background(), opts.timeout)
			defer cancel()
			outcome, err := service.Run(ctx, conn, sqlText, opts.limit)
			if err != nil {
				return err
			}
			envData := map[string]any{"connection": conn.Name, "result": outcome.Result}
			page := map[string]any{"limit": opts.limit}
			if outcome.NextCursor != "" {
				page["next_cursor"] = outcome.NextCursor
			}
			return writeJSON(cmd.OutOrStdout(), Envelope{
				OK:        true,
				Kind:      "query.result",
				Command:   "query run",
				Summary:   map[string]any{"connection": conn.Name, "db_type": conn.DBType},
				Data:      envData,
				Page:      page,
				Artifacts: []any{},
				Warnings:  []any{},
			})
		},
	}
	run.Flags().StringVar(&connectionName, "connection", "", "Connection name")
	run.Flags().StringVar(&sqlText, "sql", "", "SQL to run")
	fetch := &cobra.Command{
		Use:   "fetch-page",
		Short: "Fetch a query result page",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cursor == "" {
				return &CLIError{Code: "query.missing_cursor", Message: "cursor is required", Exit: 2}
			}
			page, err := result.NewPageStore("").Fetch(cursor)
			if err != nil {
				return err
			}
			return writeSuccess(cmd.OutOrStdout(), "query fetch-page", "query.page", page, map[string]any{"has_next": page.NextCursor != ""})
		},
	}
	fetch.Flags().StringVar(&cursor, "cursor", "", "Cursor returned by query run")
	cmd.AddCommand(run, fetch)
	return cmd
}

func schemaCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "schema", Short: "Inspect schemas"}
	var connectionName string
	tree := &cobra.Command{
		Use:   "tree",
		Short: "Return schema tree",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := loadStore(opts)
			if err != nil {
				return err
			}
			conn, ok := store.Find(connectionName)
			if !ok {
				return &CLIError{Code: "connection.not_found", Message: "connection not found", Exit: 2}
			}
			provider, ok := registry().Get(conn.DBType)
			if !ok {
				return &CLIError{Code: "adapter.unsupported", Message: adapter.MissingProvider(conn.DBType).Error(), Exit: 2}
			}
			ctx, cancel := context.WithTimeout(context.Background(), opts.timeout)
			defer cancel()
			session, err := provider.Open(ctx, conn)
			if err != nil {
				return err
			}
			defer session.Close()
			data, err := provider.Schema(ctx, session)
			if err != nil {
				return err
			}
			return writeSuccess(cmd.OutOrStdout(), "schema tree", "schema.tree", data, map[string]any{"connection": conn.Name})
		},
	}
	tree.Flags().StringVar(&connectionName, "connection", "", "Connection name")
	cmd.AddCommand(tree)
	return cmd
}

func serveCommand(opts *options) *cobra.Command {
	var protocolName string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve protocol over stdio",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := loadStore(opts)
			if err != nil {
				return err
			}
			if protocolName != "jsonrpc" && protocolName != "ndjson" {
				return &CLIError{Code: "protocol.invalid", Message: "protocol must be jsonrpc or ndjson", Exit: 2}
			}
			server := protocol.Server{Store: store, Registry: registry(), PageStore: result.NewPageStore(""), Protocol: protocolName}
			return server.Serve(context.Background(), os.Stdin, os.Stdout)
		},
	}
	cmd.Flags().StringVar(&protocolName, "protocol", "jsonrpc", "Protocol: jsonrpc or ndjson")
	return cmd
}

func loadStore(opts *options) (*config.Store, error) {
	return config.NewStore(opts.configDir, opts.credentialsPath)
}

func registry() *adapter.Registry {
	reg := adapter.NewRegistry()
	reg.Register(sqlite.New())
	reg.Register(postgres.New())
	reg.Register(mysql.New())
	reg.Register(snowflake.New())
	reg.Register(bigquery.New())
	return reg
}

func catalog() []commandInfo {
	return []commandInfo{
		{Name: "commands", Summary: "List command catalog", Stability: "stable", Mutates: false, Examples: []string{"miudb commands --output json"}},
		{Name: "describe", Summary: "Describe a command", Stability: "stable", Mutates: false, Examples: []string{"miudb describe query run --output json"}},
		{Name: "connections list", Summary: "List saved connections with secrets redacted", Stability: "stable", Mutates: false, Examples: []string{"miudb connections list --output json"}},
		{Name: "connections test", Summary: "Test one connection", Stability: "experimental", Mutates: false, SideEffects: []string{"opens_connection", "may_create_tunnel"}},
		{Name: "connections smoke", Summary: "Run a bounded smoke query across saved connections", Stability: "experimental", Mutates: false, SideEffects: []string{"opens_connections", "may_create_tunnels"}, Examples: []string{"miudb connections smoke --timeout 12s --concurrency 4 --output json"}},
		{Name: "query run", Summary: "Run SQL against a saved connection", Stability: "experimental", Mutates: false, SideEffects: []string{"opens_connection", "may_create_tunnel", "may_write_page_store"}},
		{Name: "query fetch-page", Summary: "Fetch a continued result page", Stability: "experimental", Mutates: false},
		{Name: "schema tree", Summary: "Inspect schema objects", Stability: "experimental", Mutates: false, SideEffects: []string{"opens_connection", "may_create_tunnel"}},
		{Name: "serve", Summary: "Serve JSON-RPC or NDJSON over stdio", Stability: "experimental", Mutates: false},
	}
}

func commandPath(args []string) string {
	if len(args) == 0 {
		return "miudb"
	}
	if len(args) > 2 {
		args = args[:2]
	}
	return strings.Join(args, " ")
}

func init() {
	cobra.EnableCommandSorting = false
}

func printfErr(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
}
