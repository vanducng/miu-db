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
	"github.com/vanducng/miu-db/internal/config"
	"github.com/vanducng/miu-db/internal/core"
	"github.com/vanducng/miu-db/internal/mcpserver"
	"github.com/vanducng/miu-db/internal/protocol"
	"github.com/vanducng/miu-db/internal/result"
)

type options struct {
	output           string
	connectionSource string
	configDir        string
	connectionsPath  string
	credentialsPath  string
	secretSources    string
	keyringService   string
	gopassPrefix     string
	limit            int
	timeout          time.Duration
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

var version = "v0.2.0-go.9-dev"

func Execute(args []string) error {
	opts := &options{output: "json", limit: 100, timeout: 30 * time.Second}
	root := rootCommand(opts)
	root.SetArgs(args)
	root.SilenceUsage = true
	root.SilenceErrors = true
	if err := root.Execute(); err != nil {
		errorWriter := os.Stdout
		if isMCPServeCommand(args) {
			errorWriter = os.Stderr
		}
		_ = writeError(errorWriter, commandPath(args), err)
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
	root.PersistentFlags().StringVar(&opts.connectionSource, "connection-source", config.SourceAuto, "Connection source: auto or file")
	root.PersistentFlags().StringVar(&opts.configDir, "config-dir", config.DefaultConfigDir(), "Config directory for file source")
	root.PersistentFlags().StringVar(&opts.connectionsPath, "connections-file", "", "Connections JSON file")
	root.PersistentFlags().StringVar(&opts.credentialsPath, "credentials-file", "", "Credential JSON file")
	root.PersistentFlags().StringVar(&opts.credentialsPath, "credentials-export", "", "Deprecated alias for --credentials-file")
	root.PersistentFlags().StringVar(&opts.secretSources, "secret-source", "", "Comma-separated secret sources: file,keyring,gopass,none")
	root.PersistentFlags().StringVar(&opts.keyringService, "keyring-service", "", "OS keyring service name; defaults to miudb")
	root.PersistentFlags().StringVar(&opts.gopassPrefix, "gopass-prefix", "miudb", "gopass path prefix")
	root.PersistentFlags().IntVar(&opts.limit, "limit", 100, "Maximum rows returned inline")
	root.PersistentFlags().DurationVar(&opts.timeout, "timeout", 30*time.Second, "Connection/query timeout")
	root.AddCommand(versionCommand())
	root.AddCommand(commandsCommand(opts))
	root.AddCommand(describeCommand(opts))
	root.AddCommand(connectionsCommand(opts))
	root.AddCommand(queryCommand(opts))
	root.AddCommand(schemaCommand(opts))
	root.AddCommand(mcpCommand(opts))
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
	if version != "" && !strings.HasSuffix(version, "-dev") {
		return version
	}
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
			services, err := loadServices(opts)
			if err != nil {
				return err
			}
			items := []any{}
			byType := map[string]int{}
			for _, conn := range services.Connections() {
				items = append(items, config.RedactedConnection(conn))
				byType[conn.DBType]++
			}
			return writeSuccess(cmd.OutOrStdout(), "connections list", "connection.list", map[string]any{"connections": items}, map[string]any{"count": len(items), "by_type": byType, "store": services.Store.Info()})
		},
	})
	cmd.AddCommand(connectionAddCommand(opts))
	cmd.AddCommand(&cobra.Command{
		Use:   "test <name>",
		Short: "Test a connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			services, err := loadServices(opts)
			if err != nil {
				return err
			}
			conn, err := services.TestConnection(cmd.Context(), args[0])
			if err != nil {
				if strings.Contains(err.Error(), "not found") {
					return &CLIError{Code: "connection.not_found", Message: "connection not found", Exit: 2}
				}
				if strings.Contains(err.Error(), "unsupported database type") {
					return &CLIError{Code: "adapter.unsupported", Message: adapter.MissingProvider(conn.DBType).Error(), Exit: 2}
				}
				return err
			}
			return writeSuccess(cmd.OutOrStdout(), "connections test", "connection.test", map[string]any{"name": conn.Name, "ok": true}, map[string]any{"db_type": conn.DBType})
		},
	})
	cmd.AddCommand(connectionsSmokeCommand(opts))
	return cmd
}

func connectionAddCommand(opts *options) *cobra.Command {
	var conn config.Connection
	var secretStore string
	var tunnelEnabled bool
	var sshHost, sshPort, sshUser, sshPassword, sshKeyPath, sshConfigAlias string
	var optionFlags []string
	var extraOptionFlags []string
	add := &cobra.Command{
		Use:   "add",
		Short: "Add a native miudb connection",
		RunE: func(cmd *cobra.Command, args []string) error {
			if conn.Name == "" || conn.DBType == "" {
				return &CLIError{Code: "connection.missing_input", Message: "name and db-type are required", Exit: 2}
			}
			options, err := parseOptionFlags(optionFlags)
			if err != nil {
				return err
			}
			extraOptions, err := parseExtraOptionFlags(extraOptionFlags)
			if err != nil {
				return err
			}
			conn.Options = options
			conn.ExtraOptions = extraOptions
			if conn.Endpoint.Kind == "" {
				if conn.Endpoint.Path != "" {
					conn.Endpoint.Kind = "file"
				} else {
					conn.Endpoint.Kind = "tcp"
				}
			}
			if tunnelEnabled || sshHost != "" || sshConfigAlias != "" {
				conn.Tunnel = &config.Tunnel{
					Enabled:     true,
					Source:      "manual",
					ConfigAlias: sshConfigAlias,
					Host:        sshHost,
					Port:        sshPort,
					Username:    sshUser,
					Password:    sshPassword,
					KeyPath:     sshKeyPath,
				}
				if sshConfigAlias != "" {
					conn.Tunnel.Source = "config"
				}
				if conn.Tunnel.Port == "" {
					conn.Tunnel.Port = "22"
				}
				if conn.Tunnel.KeyPath != "" {
					conn.Tunnel.AuthType = "key"
				}
				if conn.Tunnel.Password != "" {
					conn.Tunnel.AuthType = "password"
				}
			}
			store, err := loadStoreAllowMissing(opts)
			if err != nil {
				return err
			}
			saved, err := store.Add(conn, config.AddOptions{SecretStore: secretStore})
			if err != nil {
				return err
			}
			return writeSuccess(cmd.OutOrStdout(), "connections add", "connection.added", config.RedactedConnection(saved), map[string]any{"store": store.Info(), "sensitive_targets": config.SensitiveTargets(saved)})
		},
	}
	add.Flags().StringVar(&conn.Name, "name", "", "Connection name")
	add.Flags().StringVar(&conn.DBType, "db-type", "", "Database type")
	add.Flags().StringVar(&conn.FolderPath, "folder", "", "Connection folder")
	add.Flags().StringVar(&conn.ConnectionURL, "url", "", "Connection URL")
	add.Flags().StringVar(&conn.Endpoint.Kind, "kind", "", "Endpoint kind: tcp or file")
	add.Flags().StringVar(&conn.Endpoint.Host, "host", "", "Database host")
	add.Flags().StringVar(&conn.Endpoint.Port, "port", "", "Database port")
	add.Flags().StringVar(&conn.Endpoint.Database, "database", "", "Database name")
	add.Flags().StringVar(&conn.Endpoint.Username, "username", "", "Database username")
	add.Flags().StringVar(&conn.Endpoint.Password, "password", "", "Database password; stored outside connections.json by default")
	add.Flags().StringVar(&conn.Endpoint.PasswordCommand, "password-command", "", "Command to resolve database password")
	add.Flags().StringVar(&conn.Endpoint.Path, "path", "", "File path for SQLite-like databases")
	add.Flags().StringArrayVar(&optionFlags, "option", nil, "Provider option as key=value; repeat for multiple options")
	add.Flags().StringArrayVar(&extraOptionFlags, "extra-option", nil, "Driver extra option as key=value; repeat for multiple options")
	add.Flags().StringVar(&secretStore, "secret-store", "keyring", "Secret storage for sensitive fields: keyring, file, inline, none")
	add.Flags().BoolVar(&tunnelEnabled, "tunnel", false, "Enable SSH tunnel")
	add.Flags().StringVar(&sshHost, "ssh-host", "", "SSH tunnel host")
	add.Flags().StringVar(&sshPort, "ssh-port", "22", "SSH tunnel port")
	add.Flags().StringVar(&sshUser, "ssh-username", "", "SSH tunnel username")
	add.Flags().StringVar(&sshPassword, "ssh-password", "", "SSH password; stored outside connections.json by default")
	add.Flags().StringVar(&sshKeyPath, "ssh-key-path", "", "SSH private key path")
	add.Flags().StringVar(&sshConfigAlias, "ssh-config-alias", "", "SSH config alias")
	return add
}

func parseOptionFlags(values []string) (map[string]any, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := map[string]any{}
	for _, value := range values {
		key, raw, ok := strings.Cut(value, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, &CLIError{Code: "connection.invalid_option", Message: "option must be key=value", Exit: 2}
		}
		out[key] = parseOptionValue(strings.TrimSpace(raw))
	}
	return out, nil
}

func parseExtraOptionFlags(values []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := map[string]string{}
	for _, value := range values {
		key, raw, ok := strings.Cut(value, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, &CLIError{Code: "connection.invalid_extra_option", Message: "extra-option must be key=value", Exit: 2}
		}
		out[key] = strings.TrimSpace(raw)
	}
	return out, nil
}

func parseOptionValue(value string) any {
	switch strings.ToLower(value) {
	case "true":
		return true
	case "false":
		return false
	default:
		return value
	}
}

func connectionsSmokeCommand(opts *options) *cobra.Command {
	var sqlText string
	var names []string
	var concurrency int
	smoke := &cobra.Command{
		Use:   "smoke",
		Short: "Run a smoke query across saved connections",
		RunE: func(cmd *cobra.Command, args []string) error {
			services, err := loadServices(opts)
			if err != nil {
				return err
			}
			conns := selectConnections(services.Connections(), names)
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
			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for idx := range jobs {
						conn, err := services.Store.Resolve(conns[idx])
						if err != nil {
							results[idx] = smokeResolveError(conns[idx], err)
							continue
						}
						results[idx] = runSmoke(cmd.Context(), services, conn, sqlText, opts.limit)
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

func runSmoke(parent context.Context, services *core.Services, conn config.Connection, sqlText string, limit int) smokeResult {
	start := time.Now()
	ctx := parent
	cancel := func() {}
	if services.Timeout > 0 {
		ctx, cancel = context.WithTimeout(parent, services.Timeout)
	}
	defer cancel()
	outcome, err := services.RunQueryConnection(ctx, conn, sqlText, limit)
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

func smokeResolveError(conn config.Connection, err error) smokeResult {
	info := errorInfo(err)
	if errors.Is(err, context.DeadlineExceeded) {
		info.Code = "secret.timeout"
		info.Retryable = true
		info.SafeToRetry = true
	}
	return smokeResult{
		Name:   conn.Name,
		DBType: conn.DBType,
		Error:  &info,
	}
}

func errorInfo(err error) ErrorInfo {
	message := config.RedactString(err.Error())
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
		info.Message = config.RedactString(cliErr.Message)
		info.Hint = config.RedactString(cliErr.Hint)
		info.Details = redactDetails(cliErr.Details)
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
			services, err := loadServices(opts)
			if err != nil {
				return err
			}
			conn, outcome, err := services.RunQuery(cmd.Context(), connectionName, sqlText, opts.limit)
			if err != nil {
				if strings.Contains(err.Error(), "not found") {
					return &CLIError{Code: "connection.not_found", Message: "connection not found", Exit: 2}
				}
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
			page, err := (&core.Services{PageStore: result.NewPageStore("")}).FetchPage(cursor)
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
			services, err := loadServices(opts)
			if err != nil {
				return err
			}
			conn, data, err := services.SchemaTree(cmd.Context(), connectionName)
			if err != nil {
				if strings.Contains(err.Error(), "not found") {
					return &CLIError{Code: "connection.not_found", Message: "connection not found", Exit: 2}
				}
				if strings.Contains(err.Error(), "unsupported database type") {
					return &CLIError{Code: "adapter.unsupported", Message: adapter.MissingProvider(conn.DBType).Error(), Exit: 2}
				}
				return err
			}
			return writeSuccess(cmd.OutOrStdout(), "schema tree", "schema.tree", data, map[string]any{"connection": conn.Name})
		},
	}
	tree.Flags().StringVar(&connectionName, "connection", "", "Connection name")
	cmd.AddCommand(tree)
	return cmd
}

func mcpCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "mcp", Short: "Serve Model Context Protocol"}
	var transport string
	var allowedConnections []string
	var defaultLimit int
	var maxLimit int
	var maxBytes int
	var allowMutations bool
	serve := &cobra.Command{
		Use:   "serve",
		Short: "Serve MCP over stdio",
		RunE: func(cmd *cobra.Command, args []string) error {
			normalizedTransport := strings.ToLower(strings.TrimSpace(transport))
			if normalizedTransport == "" {
				normalizedTransport = mcpserver.TransportStdio
			}
			if normalizedTransport != mcpserver.TransportStdio {
				return &CLIError{
					Code:    "mcp.unsupported_transport",
					Message: (&mcpserver.UnsupportedTransportError{Transport: transport}).Error(),
					Exit:    2,
					Details: map[string]any{"transport": transport},
				}
			}
			services, err := loadServices(opts)
			if err != nil {
				return err
			}
			err = mcpserver.Serve(cmd.Context(), services, mcpserver.Options{
				Transport:             normalizedTransport,
				ImplementationName:    "miudb",
				ImplementationVersion: versionString(),
				AllowedConnections:    allowedConnections,
				Timeout:               opts.timeout,
				DefaultLimit:          defaultLimit,
				MaxLimit:              maxLimit,
				MaxBytes:              maxBytes,
				AllowMutations:        allowMutations,
			}, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
			var unsupported *mcpserver.UnsupportedTransportError
			if errors.As(err, &unsupported) {
				return &CLIError{
					Code:    "mcp.unsupported_transport",
					Message: unsupported.Error(),
					Exit:    2,
					Details: map[string]any{"transport": unsupported.Transport},
				}
			}
			return err
		},
	}
	serve.Flags().StringVar(&transport, "transport", mcpserver.TransportStdio, "MCP transport: stdio")
	serve.Flags().StringArrayVar(&allowedConnections, "connection", nil, "Allowed connection name; repeat to restrict MCP-visible connections")
	serve.Flags().IntVar(&defaultLimit, "limit", opts.limit, "Default row limit for MCP query tools")
	serve.Flags().IntVar(&maxLimit, "max-limit", 1000, "Maximum row limit accepted by MCP query tools")
	serve.Flags().IntVar(&maxBytes, "max-bytes", 1<<20, "Maximum serialized bytes per MCP tool response")
	serve.Flags().BoolVar(&allowMutations, "allow-mutate", false, "Allow mutation SQL through MCP query_run; unsafe")
	cmd.AddCommand(serve)
	return cmd
}

func serveCommand(opts *options) *cobra.Command {
	var protocolName string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve protocol over stdio",
		RunE: func(cmd *cobra.Command, args []string) error {
			services, err := loadServices(opts)
			if err != nil {
				return err
			}
			if protocolName != "jsonrpc" && protocolName != "ndjson" {
				return &CLIError{Code: "protocol.invalid", Message: "protocol must be jsonrpc or ndjson", Exit: 2}
			}
			server := protocol.Server{Services: services, Protocol: protocolName}
			return server.Serve(cmd.Context(), os.Stdin, os.Stdout)
		},
	}
	cmd.Flags().StringVar(&protocolName, "protocol", "jsonrpc", "Protocol: jsonrpc or ndjson")
	return cmd
}

func loadStore(opts *options) (*config.Store, error) {
	return config.NewStoreWithOptions(config.StoreOptions{
		Source:          opts.connectionSource,
		ConfigDir:       opts.configDir,
		ConnectionsPath: opts.connectionsPath,
		CredentialsPath: opts.credentialsPath,
		SecretSources:   splitCSV(opts.secretSources),
		KeyringService:  opts.keyringService,
		GopassPrefix:    opts.gopassPrefix,
		SecretTimeout:   opts.timeout,
	})
}

func loadStoreAllowMissing(opts *options) (*config.Store, error) {
	return config.NewWritableStore(config.StoreOptions{
		Source:          opts.connectionSource,
		ConfigDir:       opts.configDir,
		ConnectionsPath: opts.connectionsPath,
		CredentialsPath: opts.credentialsPath,
		SecretSources:   splitCSV(opts.secretSources),
		KeyringService:  opts.keyringService,
		GopassPrefix:    opts.gopassPrefix,
		SecretTimeout:   opts.timeout,
	})
}

func loadServices(opts *options) (*core.Services, error) {
	store, err := loadStore(opts)
	if err != nil {
		return nil, err
	}
	return core.NewServices(store, opts.timeout), nil
}

func catalog() []commandInfo {
	return []commandInfo{
		{Name: "commands", Summary: "List command catalog", Stability: "stable", Mutates: false, Examples: []string{"miudb commands --output json"}},
		{Name: "describe", Summary: "Describe a command", Stability: "stable", Mutates: false, Examples: []string{"miudb describe query run --output json"}},
		{Name: "connections list", Summary: "List saved connections with secrets redacted", Stability: "stable", Mutates: false, Examples: []string{"miudb connections list --output json"}},
		{Name: "connections add", Summary: "Add a native connection and store sensitive fields safely", Stability: "experimental", Mutates: true, SideEffects: []string{"writes_connections_file", "may_write_keyring", "may_write_credentials_file"}, Examples: []string{"miudb connections add --name local --db-type sqlite --path ./app.db --output json"}},
		{Name: "connections test", Summary: "Test one connection", Stability: "experimental", Mutates: false, SideEffects: []string{"opens_connection", "may_create_tunnel"}},
		{Name: "connections smoke", Summary: "Run a bounded smoke query across saved connections", Stability: "experimental", Mutates: false, SideEffects: []string{"opens_connections", "may_create_tunnels"}, Examples: []string{"miudb connections smoke --timeout 12s --concurrency 4 --output json"}},
		{Name: "query run", Summary: "Run SQL against a saved connection", Stability: "experimental", Mutates: false, SideEffects: []string{"opens_connection", "may_create_tunnel", "may_write_page_store"}},
		{Name: "query fetch-page", Summary: "Fetch a continued result page", Stability: "experimental", Mutates: false},
		{Name: "schema tree", Summary: "Inspect schema objects", Stability: "experimental", Mutates: false, SideEffects: []string{"opens_connection", "may_create_tunnel"}},
		{Name: "mcp serve", Summary: "Serve standard MCP over stdio", Stability: "experimental", Mutates: false, SideEffects: []string{"opens_connections", "may_create_tunnels"}},
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

func isMCPServeCommand(args []string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "mcp" && args[i+1] == "serve" {
			return true
		}
	}
	return false
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := []string{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func init() {
	cobra.EnableCommandSorting = false
}

func printfErr(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
}
