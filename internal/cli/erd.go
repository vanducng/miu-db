package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/vanducng/miu-db/internal/auth"
	"github.com/vanducng/miu-db/internal/erd"
)

func erdCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "erd", Short: "Entity-relationship diagram tools"}

	cmd.AddCommand(erdGenerateCommand(opts))
	cmd.AddCommand(erdServeCommand(opts))
	cmd.AddCommand(erdMetaCommand(opts))
	return cmd
}

func erdGenerateCommand(opts *options) *cobra.Command {
	var connName, schemaName, tablesFlag, metaPath, outputDir, formatFlag, title string
	var cdn bool

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate an interactive offline ERD (HTML) + schema.json/DBML from a connection",
		RunE: func(cmd *cobra.Command, args []string) error {
			if connName == "" {
				return &CLIError{Code: "erd.missing_connection", Message: "--connection is required", Exit: 2}
			}

			var tables []string
			if tablesFlag != "" {
				tables = splitCSV(tablesFlag)
			}

			var formats []string
			for _, f := range splitCSV(formatFlag) {
				formats = append(formats, strings.ToLower(f))
			}

			var metaPtr *erd.Meta
			if metaPath != "" {
				b, err := os.ReadFile(metaPath)
				if err != nil {
					return &CLIError{Code: "erd.meta_unreadable", Message: fmt.Sprintf("cannot read meta file: %v", err), Exit: 2}
				}
				var m erd.Meta
				if err := json.Unmarshal(b, &m); err != nil {
					return &CLIError{Code: "erd.meta_invalid", Message: fmt.Sprintf("invalid meta JSON: %v", err), Exit: 2}
				}
				metaPtr = &m
			}

			if title != "" {
				if metaPtr == nil {
					metaPtr = &erd.Meta{}
				}
				if metaPtr.Title == "" {
					metaPtr.Title = title
				}
			}

			services, err := loadServices(opts)
			if err != nil {
				return err
			}

			if outputDir == "" {
				outputDir = fmt.Sprintf(".diagrams/%s-erd", connName)
			}

			defaultTitle := schemaName
			if defaultTitle == "" {
				defaultTitle = connName
			}
			result, err := services.GenerateERD(cmd.Context(), connName, erd.GenerateOpts{
				OutputDir:    outputDir,
				Formats:      formats,
				Meta:         metaPtr,
				Schema:       schemaName,
				Tables:       tables,
				CDN:          cdn,
				DefaultTitle: defaultTitle,
			}, opts.captureMeta())
			if err != nil {
				if strings.Contains(err.Error(), "not found") {
					return &CLIError{Code: "connection.not_found", Message: "connection not found", Exit: 2}
				}
				if strings.Contains(err.Error(), "unsupported database type") {
					return &CLIError{Code: "erd.unsupported_db", Message: err.Error(), Exit: 2}
				}
				return err
			}

			artifacts := make([]any, len(result.Files))
			for i, f := range result.Files {
				artifacts[i] = f
			}

			return writeJSON(cmd.OutOrStdout(), Envelope{
				OK:      true,
				Kind:    "erd.generate",
				Command: "erd generate",
				Summary: map[string]any{
					"connection": connName,
					"output":     result.OutputDir,
					"tables":     result.TableCount,
					"formats":    formats,
				},
				Data: map[string]any{
					"output":  result.OutputDir,
					"tables":  result.TableCount,
					"formats": formats,
				},
				Artifacts: artifacts,
				Warnings:  []any{},
			})
		},
	}

	cmd.Flags().StringVar(&connName, "connection", "", "Connection name (required)")
	cmd.Flags().StringVar(&schemaName, "schema", "", "Database/schema name; defaults to the connection's default schema")
	cmd.Flags().StringVar(&tablesFlag, "tables", "", "Comma-separated table names to include; default all")
	cmd.Flags().StringVar(&metaPath, "meta", "", "Path to meta.json for agentic polish layer")
	cmd.Flags().StringVar(&outputDir, "out-dir", "", "Output directory (default .diagrams/<connection>-erd/)")
	cmd.Flags().StringVar(&formatFlag, "format", "html", "Comma-separated output formats: html,json,dbml")
	cmd.Flags().BoolVar(&cdn, "cdn", false, "Link renderer libs from CDN instead of inlining (smaller file, needs network)")
	cmd.Flags().StringVar(&title, "title", "", "Diagram title (overrides meta.title when meta.title is empty)")

	return cmd
}

func erdServeCommand(opts *options) *cobra.Command {
	var connName, fromPath, schemaName, tablesFlag, metaPath, title string
	var port int
	var noOpen, cdn bool

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Render an ERD and serve it on a local browser",
		Long:  "Render an ERD and serve it on a loopback HTTP server, opening the browser automatically. Use --from to load an existing export directory or schema.json; use --connection for a live introspection.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if fromPath == "" && connName == "" {
				return &CLIError{Code: "erd.missing_source", Message: "--connection or --from is required", Exit: 2}
			}
			if fromPath != "" && connName != "" {
				return &CLIError{Code: "erd.ambiguous_source", Message: "--connection and --from are mutually exclusive", Exit: 2}
			}

			var payload erd.Payload

			if fromPath != "" {
				p, err := erd.LoadPayloadFromPath(fromPath)
				if err != nil {
					return &CLIError{Code: "erd.from_unreadable", Message: fmt.Sprintf("cannot load ERD from %q: %v", fromPath, err), Exit: 2}
				}
				payload = p
			} else {
				var tables []string
				if tablesFlag != "" {
					tables = splitCSV(tablesFlag)
				}

				var metaPtr *erd.Meta
				if metaPath != "" {
					b, err := os.ReadFile(metaPath)
					if err != nil {
						return &CLIError{Code: "erd.meta_unreadable", Message: fmt.Sprintf("cannot read meta file: %v", err), Exit: 2}
					}
					var m erd.Meta
					if err := json.Unmarshal(b, &m); err != nil {
						return &CLIError{Code: "erd.meta_invalid", Message: fmt.Sprintf("invalid meta JSON: %v", err), Exit: 2}
					}
					metaPtr = &m
				}

				if title != "" {
					if metaPtr == nil {
						metaPtr = &erd.Meta{}
					}
					if metaPtr.Title == "" {
						metaPtr.Title = title
					}
				}

				services, err := loadServices(opts)
				if err != nil {
					return err
				}

				introspected, err := services.IntrospectERD(cmd.Context(), connName, erd.GenerateOpts{
					Schema: schemaName,
					Tables: tables,
					Meta:   metaPtr,
				}, opts.captureMeta())
				if err != nil {
					if strings.Contains(err.Error(), "not found") {
						return &CLIError{Code: "connection.not_found", Message: "connection not found", Exit: 2}
					}
					if strings.Contains(err.Error(), "unsupported database type") {
						return &CLIError{Code: "erd.unsupported_db", Message: err.Error(), Exit: 2}
					}
					return err
				}

				meta := erd.Meta{}
				if metaPtr != nil {
					meta = *metaPtr
				}
				payload = erd.Payload{Schema: introspected, Meta: meta}
			}

			defaultTitle := schemaName
			if defaultTitle == "" {
				defaultTitle = connName
			}
			if fromPath != "" && defaultTitle == "" {
				defaultTitle = fromPath
			}

			html, err := erd.RenderHTML(payload, erd.RenderOpts{CDN: cdn, DefaultTitle: defaultTitle})
			if err != nil {
				return fmt.Errorf("erd serve: render: %w", err)
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			suppressBrowser := noOpen || isMCPOrServeContext(cmd)

			return erd.Serve(ctx, html, erd.ServeOpts{
				Port: port,
				OnReady: func(url string) {
					fmt.Fprintf(cmd.ErrOrStderr(), "Listening at %s (press Ctrl-C to stop)\n", url)

					_ = writeJSON(cmd.OutOrStdout(), Envelope{
						OK:      true,
						Kind:    "erd.serve",
						Command: "erd serve",
						Summary: map[string]any{
							"url":        url,
							"connection": connName,
							"from":       fromPath,
							"tables":     len(payload.Schema),
						},
						Data: map[string]any{
							"url":    url,
							"tables": len(payload.Schema),
						},
						Artifacts: []any{},
						Warnings:  []any{},
					})

					if !suppressBrowser {
						auth.Open(url) //nolint:errcheck
					}
				},
			})
		},
	}

	cmd.Flags().StringVar(&connName, "connection", "", "Connection name (live introspect)")
	cmd.Flags().StringVar(&fromPath, "from", "", "Load from an existing export directory or schema.json")
	cmd.Flags().StringVar(&schemaName, "schema", "", "Database/schema name; defaults to the connection's default schema")
	cmd.Flags().StringVar(&tablesFlag, "tables", "", "Comma-separated table names to include; default all")
	cmd.Flags().StringVar(&metaPath, "meta", "", "Path to meta.json for agentic polish layer")
	cmd.Flags().IntVar(&port, "port", 0, "Port to listen on (0 = auto-pick)")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "Do not open the browser automatically")
	cmd.Flags().BoolVar(&cdn, "cdn", false, "Link renderer libs from CDN instead of inlining")
	cmd.Flags().StringVar(&title, "title", "", "Diagram title (overrides meta.title when meta.title is empty)")

	return cmd
}

func erdMetaCommand(opts *options) *cobra.Command {
	var connName, schemaName, outputDir string
	var stub, force bool

	cmd := &cobra.Command{
		Use:   "meta",
		Short: "Scaffold a meta.json stub for agentic ERD polish",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stub {
				return &CLIError{Code: "erd.meta_missing_mode", Message: "only --stub is supported", Exit: 2}
			}
			if connName == "" {
				return &CLIError{Code: "erd.missing_connection", Message: "--connection is required", Exit: 2}
			}

			services, err := loadServices(opts)
			if err != nil {
				return err
			}

			conn, ok, err := services.FindConnection(connName)
			if err != nil {
				return err
			}
			if !ok {
				return &CLIError{Code: "connection.not_found", Message: "connection not found", Exit: 2}
			}

			if outputDir == "" {
				outputDir = fmt.Sprintf(".diagrams/%s-erd", connName)
			}

			outPath := filepath.Join(outputDir, "meta.json")
			if !force {
				if _, statErr := os.Stat(outPath); statErr == nil {
					return &CLIError{
						Code:    "erd.meta_exists",
						Message: fmt.Sprintf("meta.json already exists at %s; use --force to overwrite", outPath),
						Exit:    2,
					}
				}
			}

			tables, err := services.IntrospectERD(cmd.Context(), connName, erd.GenerateOpts{
				Schema: schemaName,
			}, opts.captureMeta())
			if err != nil {
				if strings.Contains(err.Error(), "not found") {
					return &CLIError{Code: "connection.not_found", Message: "connection not found", Exit: 2}
				}
				if strings.Contains(err.Error(), "unsupported database type") {
					return &CLIError{Code: "erd.unsupported_db", Message: err.Error(), Exit: 2}
				}
				return err
			}

			meta := erd.BuildMetaStub(tables, conn.DBType)

			if err := os.MkdirAll(outputDir, 0o755); err != nil {
				return fmt.Errorf("erd meta: create output dir: %w", err)
			}

			b, err := json.MarshalIndent(meta, "", "  ")
			if err != nil {
				return fmt.Errorf("erd meta: marshal: %w", err)
			}

			if err := os.WriteFile(outPath, b, 0o644); err != nil {
				return fmt.Errorf("erd meta: write %s: %w", outPath, err)
			}

			return writeJSON(cmd.OutOrStdout(), Envelope{
				OK:        true,
				Kind:      "erd.meta",
				Command:   "erd meta",
				Summary:   map[string]any{"connection": connName, "output": outPath},
				Data:      map[string]any{"output": outPath},
				Artifacts: []any{outPath},
				Warnings:  []any{},
			})
		},
	}

	cmd.Flags().BoolVar(&stub, "stub", false, "Generate a meta.json stub (required)")
	cmd.Flags().StringVar(&connName, "connection", "", "Connection name (required)")
	cmd.Flags().StringVar(&schemaName, "schema", "", "Database/schema name; defaults to the connection's default schema")
	cmd.Flags().StringVar(&outputDir, "out-dir", "", "Output directory (default .diagrams/<connection>-erd/)")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing meta.json")

	return cmd
}
