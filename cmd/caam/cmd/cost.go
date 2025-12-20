package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	caamdb "github.com/Dicklesworthstone/coding_agent_account_manager/internal/db"
	"github.com/spf13/cobra"
)

var costCmd = &cobra.Command{
	Use:   "cost",
	Short: "View and manage cost tracking for API usage",
	Long: `View estimated costs and manage cost rate configuration.

This command tracks costs based on wrap session durations and configurable rates.
Costs are estimated based on session time, not actual API usage.

Examples:
  caam cost                        # Show cost summary for all providers
  caam cost --provider claude      # Show costs for Claude only
  caam cost --since 7d             # Show costs from last 7 days
  caam cost --json                 # Output as JSON

Subcommands:
  caam cost sessions               # List recent wrap sessions
  caam cost rates                  # Show/set cost rate configuration`,
	Args: cobra.NoArgs,
	RunE: runCostSummary,
}

var costSessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "List recent wrap sessions",
	Long: `Show recent wrap sessions with cost estimates.

Examples:
  caam cost sessions                # Show last 20 sessions
  caam cost sessions --limit 50     # Show last 50 sessions
  caam cost sessions --provider claude
  caam cost sessions --json`,
	Args: cobra.NoArgs,
	RunE: runCostSessions,
}

var costRatesCmd = &cobra.Command{
	Use:   "rates",
	Short: "View or set cost rates",
	Long: `View or configure cost rates per provider.

Rates are specified in cents. Costs are calculated as:
  estimated_cost = cents_per_session + (cents_per_minute * session_minutes)

Examples:
  caam cost rates                   # Show current rates
  caam cost rates --json            # Show rates as JSON
  caam cost rates --set claude --per-minute 5 --per-session 0
  caam cost rates --set codex --per-minute 3`,
	Args: cobra.NoArgs,
	RunE: runCostRates,
}

func init() {
	rootCmd.AddCommand(costCmd)
	costCmd.AddCommand(costSessionsCmd)
	costCmd.AddCommand(costRatesCmd)

	// Cost summary flags
	costCmd.Flags().String("provider", "", "filter by provider (claude, codex, gemini)")
	costCmd.Flags().String("since", "", "filter by time range (e.g., '24h', '7d', '30d')")
	costCmd.Flags().Bool("json", false, "output as JSON")

	// Sessions flags
	costSessionsCmd.Flags().IntP("limit", "n", 20, "maximum number of sessions to show")
	costSessionsCmd.Flags().String("provider", "", "filter by provider")
	costSessionsCmd.Flags().String("since", "", "filter sessions newer than duration")
	costSessionsCmd.Flags().Bool("json", false, "output as JSON")

	// Rates flags
	costRatesCmd.Flags().Bool("json", false, "output as JSON")
	costRatesCmd.Flags().String("set", "", "set rates for provider (requires --per-minute or --per-session)")
	costRatesCmd.Flags().Int("per-minute", -1, "cents per minute (use with --set)")
	costRatesCmd.Flags().Int("per-session", -1, "cents per session (use with --set)")
}

func runCostSummary(cmd *cobra.Command, args []string) error {
	provider, _ := cmd.Flags().GetString("provider")
	sinceStr, _ := cmd.Flags().GetString("since")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	var sinceTime time.Time
	if sinceStr != "" {
		duration, err := parseDuration(sinceStr)
		if err != nil {
			return fmt.Errorf("invalid --since duration: %w", err)
		}
		sinceTime = time.Now().Add(-duration)
	}

	db, err := caamdb.Open()
	if err != nil {
		return err
	}
	defer db.Close()

	summaries, err := db.GetCostSummary(provider, sinceTime)
	if err != nil {
		return fmt.Errorf("get cost summary: %w", err)
	}

	if jsonOutput {
		return renderCostSummaryJSON(cmd.OutOrStdout(), summaries, sinceTime)
	}
	return renderCostSummary(cmd.OutOrStdout(), summaries, sinceTime)
}

func runCostSessions(cmd *cobra.Command, args []string) error {
	limit, _ := cmd.Flags().GetInt("limit")
	provider, _ := cmd.Flags().GetString("provider")
	sinceStr, _ := cmd.Flags().GetString("since")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	var sinceTime time.Time
	if sinceStr != "" {
		duration, err := parseDuration(sinceStr)
		if err != nil {
			return fmt.Errorf("invalid --since duration: %w", err)
		}
		sinceTime = time.Now().Add(-duration)
	}

	db, err := caamdb.Open()
	if err != nil {
		return err
	}
	defer db.Close()

	sessions, err := db.GetWrapSessions(provider, sinceTime, limit)
	if err != nil {
		return fmt.Errorf("get sessions: %w", err)
	}

	if len(sessions) == 0 {
		if jsonOutput {
			return renderSessionsJSON(cmd.OutOrStdout(), nil)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "No wrap sessions found.")
		fmt.Fprintln(cmd.OutOrStdout(), "\nSessions are recorded when using: caam wrap <provider> <command>")
		return nil
	}

	if jsonOutput {
		return renderSessionsJSON(cmd.OutOrStdout(), sessions)
	}
	return renderSessions(cmd.OutOrStdout(), sessions)
}

func runCostRates(cmd *cobra.Command, args []string) error {
	jsonOutput, _ := cmd.Flags().GetBool("json")
	setProvider, _ := cmd.Flags().GetString("set")
	perMinute, _ := cmd.Flags().GetInt("per-minute")
	perSession, _ := cmd.Flags().GetInt("per-session")

	db, err := caamdb.Open()
	if err != nil {
		return err
	}
	defer db.Close()

	// Handle setting rates
	if setProvider != "" {
		// Get current rates to preserve values not being set
		currentRate, _ := db.GetCostRate(setProvider)
		newPerMinute := 0
		newPerSession := 0
		if currentRate != nil {
			newPerMinute = currentRate.CentsPerMinute
			newPerSession = currentRate.CentsPerSession
		}

		if perMinute >= 0 {
			newPerMinute = perMinute
		}
		if perSession >= 0 {
			newPerSession = perSession
		}

		if err := db.SetCostRate(setProvider, newPerMinute, newPerSession); err != nil {
			return fmt.Errorf("set rate: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Updated %s: %d¢/min, %d¢/session\n", setProvider, newPerMinute, newPerSession)
		return nil
	}

	// Show current rates
	rates, err := db.GetAllCostRates()
	if err != nil {
		return fmt.Errorf("get rates: %w", err)
	}

	if jsonOutput {
		return renderRatesJSON(cmd.OutOrStdout(), rates)
	}
	return renderRates(cmd.OutOrStdout(), rates)
}

// JSON output types
type costSummaryOutput struct {
	Summaries   []costSummaryItem `json:"summaries"`
	TotalCents  int               `json:"total_cents"`
	TotalDollar string            `json:"total_dollars"`
	Since       string            `json:"since,omitempty"`
}

type costSummaryItem struct {
	Provider        string  `json:"provider"`
	TotalSessions   int     `json:"total_sessions"`
	TotalMinutes    float64 `json:"total_minutes"`
	TotalCostCents  int     `json:"total_cost_cents"`
	TotalCostDollar string  `json:"total_cost_dollars"`
	RateLimitHits   int     `json:"rate_limit_hits"`
	AvgMinutes      float64 `json:"avg_session_minutes"`
}

type sessionsOutput struct {
	Sessions []sessionItem `json:"sessions"`
	Count    int           `json:"count"`
}

type sessionItem struct {
	ID             int     `json:"id"`
	Provider       string  `json:"provider"`
	Profile        string  `json:"profile"`
	StartedAt      string  `json:"started_at"`
	DurationSecs   int     `json:"duration_seconds"`
	DurationMins   float64 `json:"duration_minutes"`
	ExitCode       int     `json:"exit_code"`
	RateLimitHit   bool    `json:"rate_limit_hit"`
	EstimatedCents int     `json:"estimated_cost_cents"`
}

type ratesOutput struct {
	Rates []rateItem `json:"rates"`
}

type rateItem struct {
	Provider        string `json:"provider"`
	CentsPerMinute  int    `json:"cents_per_minute"`
	CentsPerSession int    `json:"cents_per_session"`
	UpdatedAt       string `json:"updated_at"`
}

func renderCostSummaryJSON(w io.Writer, summaries []caamdb.CostSummary, since time.Time) error {
	var totalCents int
	items := make([]costSummaryItem, len(summaries))
	for i, s := range summaries {
		totalCents += s.TotalCostCents
		items[i] = costSummaryItem{
			Provider:        s.Provider,
			TotalSessions:   s.TotalSessions,
			TotalMinutes:    float64(s.TotalDurationSecs) / 60.0,
			TotalCostCents:  s.TotalCostCents,
			TotalCostDollar: formatDollars(s.TotalCostCents),
			RateLimitHits:   s.RateLimitHits,
			AvgMinutes:      s.AverageDurationSec / 60.0,
		}
	}

	output := costSummaryOutput{
		Summaries:   items,
		TotalCents:  totalCents,
		TotalDollar: formatDollars(totalCents),
	}
	if !since.IsZero() {
		output.Since = since.UTC().Format(time.RFC3339)
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func renderCostSummary(w io.Writer, summaries []caamdb.CostSummary, since time.Time) error {
	if len(summaries) == 0 {
		fmt.Fprintln(w, "No cost data available.")
		fmt.Fprintln(w, "\nCosts are tracked when using: caam wrap <provider> <command>")
		return nil
	}

	if !since.IsZero() {
		fmt.Fprintf(w, "Cost Summary (since %s)\n", since.Local().Format("2006-01-02 15:04"))
	} else {
		fmt.Fprintln(w, "Cost Summary (all time)")
	}
	fmt.Fprintln(w)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "PROVIDER\tSESSIONS\tTIME\tEST. COST\tRATE LIMITS")

	var totalCents int
	for _, s := range summaries {
		totalCents += s.TotalCostCents
		_, _ = fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%d\n",
			s.Provider,
			s.TotalSessions,
			formatDurationShort(time.Duration(s.TotalDurationSecs)*time.Second),
			formatDollars(s.TotalCostCents),
			s.RateLimitHits,
		)
	}
	_ = tw.Flush()

	fmt.Fprintln(w)
	fmt.Fprintf(w, "Total Estimated Cost: %s\n", formatDollars(totalCents))
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Note: Costs are estimates based on session duration, not actual API usage.")

	return nil
}

func renderSessionsJSON(w io.Writer, sessions []caamdb.WrapSession) error {
	items := make([]sessionItem, len(sessions))
	for i, s := range sessions {
		items[i] = sessionItem{
			ID:             s.ID,
			Provider:       s.Provider,
			Profile:        s.ProfileName,
			StartedAt:      s.StartedAt.UTC().Format(time.RFC3339),
			DurationSecs:   s.DurationSeconds,
			DurationMins:   float64(s.DurationSeconds) / 60.0,
			ExitCode:       s.ExitCode,
			RateLimitHit:   s.RateLimitHit,
			EstimatedCents: s.EstimatedCostCents,
		}
	}

	output := sessionsOutput{
		Sessions: items,
		Count:    len(items),
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func renderSessions(w io.Writer, sessions []caamdb.WrapSession) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "TIME\tPROVIDER\tPROFILE\tDURATION\tCOST\tSTATUS")

	for _, s := range sessions {
		status := fmt.Sprintf("exit %d", s.ExitCode)
		if s.RateLimitHit {
			status = "rate-limited"
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			s.StartedAt.Local().Format("01-02 15:04"),
			s.Provider,
			s.ProfileName,
			formatDurationShort(time.Duration(s.DurationSeconds)*time.Second),
			formatDollars(s.EstimatedCostCents),
			status,
		)
	}
	return tw.Flush()
}

func renderRatesJSON(w io.Writer, rates []caamdb.CostRate) error {
	items := make([]rateItem, len(rates))
	for i, r := range rates {
		items[i] = rateItem{
			Provider:        r.Provider,
			CentsPerMinute:  r.CentsPerMinute,
			CentsPerSession: r.CentsPerSession,
			UpdatedAt:       r.UpdatedAt.UTC().Format(time.RFC3339),
		}
	}

	output := ratesOutput{Rates: items}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func renderRates(w io.Writer, rates []caamdb.CostRate) error {
	if len(rates) == 0 {
		fmt.Fprintln(w, "No cost rates configured.")
		return nil
	}

	fmt.Fprintln(w, "Cost Rates:")
	fmt.Fprintln(w)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "PROVIDER\tPER MINUTE\tPER SESSION\tLAST UPDATED")

	for _, r := range rates {
		_, _ = fmt.Fprintf(tw, "%s\t%d¢\t%d¢\t%s\n",
			r.Provider,
			r.CentsPerMinute,
			r.CentsPerSession,
			r.UpdatedAt.Local().Format("2006-01-02"),
		)
	}
	_ = tw.Flush()

	fmt.Fprintln(w)
	fmt.Fprintln(w, "To update rates: caam cost rates --set <provider> --per-minute <cents>")

	return nil
}

// formatDollars converts cents to a dollar string
func formatDollars(cents int) string {
	if cents == 0 {
		return "$0.00"
	}
	dollars := float64(cents) / 100.0
	return fmt.Sprintf("$%.2f", dollars)
}
