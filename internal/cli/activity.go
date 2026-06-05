package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/vanducng/miu-db/internal/activity"
	"github.com/vanducng/miu-db/internal/config"
)

func activityCommand(opts *options) *cobra.Command {
	var connFlag, groupFlag, sessionFlag, sinceFlag string
	var failedOnly bool
	var limit int

	cmd := &cobra.Command{
		Use:   "activity",
		Short: "Query captured activity events",
		RunE: func(cmd *cobra.Command, args []string) error {
			since, err := activity.ParseSince(sinceFlag)
			if err != nil {
				return &CLIError{Code: "activity.invalid_since", Message: fmt.Sprintf("invalid --since value %q: %v", sinceFlag, err), Exit: 2}
			}

			root := activityRoot(opts)
			f := activity.Filter{
				Connection: connFlag,
				Group:      groupFlag,
				Session:    sessionFlag,
				Since:      since,
				FailedOnly: failedOnly,
				Limit:      limit,
			}
			events, err := activity.Query(root, f)
			if err != nil {
				return fmt.Errorf("activity query: %w", err)
			}

			items := make([]any, len(events))
			for i, e := range events {
				items[i] = e
			}
			return writeJSON(cmd.OutOrStdout(), Envelope{
				OK:      true,
				Kind:    "activity.events",
				Command: "activity",
				Summary: map[string]any{"count": len(events), "root": root},
				Data:    map[string]any{"events": items},
			})
		},
	}
	cmd.Flags().StringVar(&connFlag, "connection", "", "Filter by connection name (bare or group/connection)")
	cmd.Flags().StringVar(&groupFlag, "group", "", "Filter by group")
	cmd.Flags().StringVar(&sessionFlag, "session", "", "Filter by session id (reconstructs trace across date dirs)")
	cmd.Flags().StringVar(&sinceFlag, "since", "", "Return events newer than duration (e.g. 1h, 24h, 7d, 2w)")
	cmd.Flags().BoolVar(&failedOnly, "failed", false, "Show only events with errors")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum events returned (0 = no cap)")

	cmd.AddCommand(activityPruneCommand(opts))
	return cmd
}

func activityPruneCommand(opts *options) *cobra.Command {
	var olderThan string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove stale activity date directories",
		RunE: func(cmd *cobra.Command, args []string) error {
			dur, err := activity.ParseSince(olderThan)
			if err != nil || dur == 0 {
				return &CLIError{Code: "activity.invalid_older_than", Message: fmt.Sprintf("invalid --older-than value %q", olderThan), Hint: "use e.g. 30d, 7d, 2w", Exit: 2}
			}

			root := activityRoot(opts)
			removed, dirs, err := activity.Prune(root, dur, dryRun)
			if err != nil {
				return fmt.Errorf("activity prune: %w", err)
			}

			dirList := make([]any, len(dirs))
			for i, d := range dirs {
				dirList[i] = d
			}
			return writeJSON(cmd.OutOrStdout(), Envelope{
				OK:      true,
				Kind:    "activity.pruned",
				Command: "activity prune",
				Summary: map[string]any{"removed": removed, "dry_run": dryRun, "older_than": olderThan},
				Data:    map[string]any{"dirs": dirList},
			})
		},
	}
	cmd.Flags().StringVar(&olderThan, "older-than", "", "Remove date dirs older than this duration (e.g. 30d, 7d, 2w)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "List dirs that would be removed without deleting them")
	return cmd
}

// activityRoot resolves the activity directory: respects MIUDB_CONFIG_DIR / --config-dir override.
func activityRoot(opts *options) string {
	dir := opts.configDir
	if dir == "" {
		envDir := os.Getenv("MIUDB_CONFIG_DIR")
		if envDir != "" {
			dir = envDir
		} else {
			dir = config.DefaultConfigDir()
		}
	}
	return dir + "/activity"
}
