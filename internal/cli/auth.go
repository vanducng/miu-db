package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/vanducng/miu-db/internal/auth"
	"github.com/vanducng/miu-db/internal/config"
)

func authCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage OAuth tokens for connections",
	}
	cmd.AddCommand(authLoginCommand(opts))
	cmd.AddCommand(authStatusCommand(opts))
	cmd.AddCommand(authLogoutCommand(opts))
	return cmd
}

func authLoginCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "login <conn>",
		Short: "Acquire and store an OAuth token for a connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			connName := args[0]
			store, err := loadStore(opts)
			if err != nil {
				return err
			}
			raw, ok := store.FindRaw(connName)
			if !ok {
				return &CLIError{Code: "connection.not_found", Message: fmt.Sprintf("connection %q not found", connName), Exit: 2}
			}
			if !isOAuthConn(raw) {
				return &CLIError{
					Code:    "auth.not_oauth",
					Message: fmt.Sprintf("connection %q does not use oauth authenticator", connName),
					Hint:    "set options.authenticator=oauth to enable OAuth for this connection",
					Exit:    2,
				}
			}
			cfg := config.OAuthConfigFromOptions(raw)
			tok, err := auth.Login(cmd.Context(), cfg)
			if err != nil {
				return fmt.Errorf("auth login: %w", err)
			}
			svc := opts.keyringService
			if svc == "" {
				svc = "miudb"
			}
			if err := config.StoreOAuthToken(svc, connName, tok); err != nil {
				return fmt.Errorf("store oauth token: %w", err)
			}
			data := map[string]any{
				"name":              connName,
				"has_refresh_token": tok.RefreshToken != "",
			}
			if !tok.Expiry.IsZero() {
				data["expires_at"] = tok.Expiry.Format(time.RFC3339)
			}
			return writeSuccess(cmd.OutOrStdout(), "auth login", "auth.login", data, nil)
		},
	}
}

func authStatusCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "status <conn>",
		Short: "Show OAuth token status for a connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			connName := args[0]
			svc := opts.keyringService
			if svc == "" {
				svc = "miudb"
			}
			tok, ok, err := config.LoadOAuthToken(svc, connName)
			if err != nil {
				return fmt.Errorf("load oauth token: %w", err)
			}
			data := map[string]any{
				"name":      connName,
				"logged_in": ok,
			}
			if ok && tok != nil {
				expired := !tok.Expiry.IsZero() && tok.Expiry.Before(time.Now())
				data["expired"] = expired
				if !tok.Expiry.IsZero() {
					data["expires_at"] = tok.Expiry.Format(time.RFC3339)
				}
			}
			return writeSuccess(cmd.OutOrStdout(), "auth status", "auth.status", data, nil)
		},
	}
}

func authLogoutCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "logout <conn>",
		Short: "Remove the stored OAuth token for a connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			connName := args[0]
			svc := opts.keyringService
			if svc == "" {
				svc = "miudb"
			}
			if err := config.DeleteOAuthToken(svc, connName); err != nil {
				return fmt.Errorf("delete oauth token: %w", err)
			}
			return writeSuccess(cmd.OutOrStdout(), "auth logout", "auth.logout", map[string]any{
				"name":    connName,
				"cleared": true,
			}, nil)
		},
	}
}

// isOAuthConn reports whether conn is configured for OAuth authentication.
func isOAuthConn(conn config.Connection) bool {
	auth, _ := conn.Options["authenticator"].(string)
	return auth == "oauth" || auth == "OAUTH"
}

// isInteractive reports true when stdout is a character device (real TTY).
func isInteractive() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// maybeLazyLogin handles ErrOAuthLoginRequired. On a real TTY it runs the
// browser login flow and signals the caller to retry by returning nil and
// setting *retried=true. On non-TTY or mcp/serve contexts it returns a
// typed CLIError with a copy-paste hint so agents can take action.
func maybeLazyLogin(ctx context.Context, opts *options, connName string, origErr error, retried *bool, isMCPOrServe bool) error {
	if !errors.Is(origErr, config.ErrOAuthLoginRequired) {
		return origErr
	}
	if *retried || isMCPOrServe || !isInteractive() {
		return &CLIError{
			Code:    "auth.login_required",
			Message: fmt.Sprintf("oauth login required for connection %q", connName),
			Hint:    fmt.Sprintf("run: miudb auth login %s", connName),
			Exit:    2,
		}
	}

	store, err := loadStore(opts)
	if err != nil {
		return err
	}
	raw, ok := store.FindRaw(connName)
	if !ok {
		return &CLIError{Code: "connection.not_found", Message: fmt.Sprintf("connection %q not found", connName), Exit: 2}
	}
	cfg := config.OAuthConfigFromOptions(raw)
	tok, err := auth.Login(ctx, cfg)
	if err != nil {
		return fmt.Errorf("auth login: %w", err)
	}
	svc := opts.keyringService
	if svc == "" {
		svc = "miudb"
	}
	if err := config.StoreOAuthToken(svc, connName, tok); err != nil {
		return fmt.Errorf("store oauth token: %w", err)
	}
	*retried = true
	return nil
}
