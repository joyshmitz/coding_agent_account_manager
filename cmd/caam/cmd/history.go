package cmd

import (
	"fmt"
	"io"
	"text/tabwriter"

	caamdb "github.com/Dicklesworthstone/coding_agent_account_manager/internal/db"
	"github.com/spf13/cobra"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "List recent activity events",
	Long: `Show recent account activity from the event log.

Examples:
  caam history           # Show last 20 events
  caam history --limit 50  # Show last 50 events`,
	Args: cobra.NoArgs,
	RunE: runHistory,
}

func init() {
	rootCmd.AddCommand(historyCmd)
	historyCmd.Flags().IntP("limit", "n", 20, "maximum number of events to show")
}

func runHistory(cmd *cobra.Command, args []string) error {
	limit, _ := cmd.Flags().GetInt("limit")

	db, err := caamdb.Open()
	if err != nil {
		return err
	}
	defer db.Close()

	events, err := db.ListRecentEvents(limit)
	if err != nil {
		return fmt.Errorf("get events: %w", err)
	}

	if len(events) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No events recorded.")
		return nil
	}

	return renderEventList(cmd.OutOrStdout(), events)
}

func renderEventList(w io.Writer, events []caamdb.Event) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "TIMESTAMP\tTYPE\tPROVIDER\tPROFILE")
	for _, ev := range events {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			ev.Timestamp.Local().Format("2006-01-02 15:04:05"),
			ev.Type,
			ev.Provider,
			ev.ProfileName,
		)
	}
	return tw.Flush()
}
