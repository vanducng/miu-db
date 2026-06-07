package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vanducng/miu-db/internal/erd"
)

func erdCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "erd", Short: "Entity-relationship diagram tools"}

	cmd.AddCommand(erdGenerateCommand(opts))
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
