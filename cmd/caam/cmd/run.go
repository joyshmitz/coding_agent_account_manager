package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/authfile"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/config"
	caamdb "github.com/Dicklesworthstone/coding_agent_account_manager/internal/db"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/health"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/rotation"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/usage"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/wrap"
	"github.com/spf13/cobra"
)

// runCmd wraps AI CLI execution with automatic rate limit handling.
var runCmd = &cobra.Command{
	Use:   "run <tool> [-- args...]",
	Short: "Run AI CLI with automatic account switching",
	Long: `Wraps AI CLI execution with transparent rate limit detection and automatic
profile switching. This is the "zero friction" mode - just use caam run instead
of calling the CLI directly.

When a rate limit is detected:
1. The current profile is put into cooldown
2. The next best profile is automatically selected
3. The command is re-executed seamlessly

Use --precheck for proactive switching:
  When enabled, caam checks real-time usage levels BEFORE running and
  automatically switches to a healthier profile if current usage is near
  the limit. This prevents rate limit errors before they happen.

Examples:
  caam run claude -- "explain this code"
  caam run codex -- --model gpt-5 "write tests"
  caam run gemini -- "summarize this file"

  # Proactive switching (checks usage before running)
  caam run claude --precheck -- "explain this code"

  # Interactive mode (no auto-retry on rate limit)
  caam run claude

For shell integration, add an alias:
  alias claude='caam run claude --precheck --'

Then you can just use:
  claude "explain this code"

And rate limits will be handled automatically!`,
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: false,
	RunE:               runWrap,
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().Int("max-retries", 1, "maximum retry attempts on rate limit (0 = no retries)")
	runCmd.Flags().Duration("cooldown", 60*time.Minute, "cooldown duration after rate limit")
	runCmd.Flags().Bool("quiet", false, "suppress profile switch notifications")
	runCmd.Flags().String("algorithm", "smart", "rotation algorithm (smart, round_robin, random)")
	runCmd.Flags().Bool("precheck", false, "check usage levels before running and switch if near limit")
	runCmd.Flags().Float64("precheck-threshold", 0.8, "usage threshold for precheck switching (0-1)")
}

func runWrap(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("tool name required")
	}

	tool := strings.ToLower(args[0])

	// Validate tool
	if _, ok := tools[tool]; !ok {
		return fmt.Errorf("unknown tool: %s (supported: codex, claude, gemini)", tool)
	}

	// Parse CLI args (everything after the tool name)
	var cliArgs []string
	if len(args) > 1 {
		cliArgs = args[1:]
	}

	// Get flags
	maxRetries, _ := cmd.Flags().GetInt("max-retries")
	cooldown, _ := cmd.Flags().GetDuration("cooldown")
	quiet, _ := cmd.Flags().GetBool("quiet")
	algorithmStr, _ := cmd.Flags().GetString("algorithm")

	// Parse algorithm
	var algorithm rotation.Algorithm
	switch strings.ToLower(algorithmStr) {
	case "smart":
		algorithm = rotation.AlgorithmSmart
	case "round_robin", "roundrobin":
		algorithm = rotation.AlgorithmRoundRobin
	case "random":
		algorithm = rotation.AlgorithmRandom
	default:
		return fmt.Errorf("unknown algorithm: %s (supported: smart, round_robin, random)", algorithmStr)
	}

	// Initialize vault
	if vault == nil {
		vault = authfile.NewVault(authfile.DefaultVaultPath())
	}

	// Initialize database
	db, err := caamdb.Open()
	if err != nil {
		// Non-fatal: cooldowns won't be recorded but execution can continue
		fmt.Fprintf(os.Stderr, "warning: database unavailable, cooldowns will not be recorded\n")
		db = nil
	}
	if db != nil {
		defer db.Close()
	}

	// Initialize health storage
	healthStore := health.NewStorage("")

	// Load global config and build wrap config with defaults
	globalCfg, err := config.Load()
	if err != nil {
		// Non-fatal: use defaults if config can't be loaded
		globalCfg = config.DefaultConfig()
	}

	// Build config from global settings (includes proper backoff defaults)
	cfg := wrap.ConfigFromGlobal(globalCfg, tool)
	cfg.Args = cliArgs
	cfg.Stdout = os.Stdout
	cfg.Stderr = os.Stderr
	cfg.NotifyOnSwitch = !quiet
	cfg.Algorithm = algorithm

	// Apply CLI flag overrides (only if explicitly set)
	if cmd.Flags().Changed("max-retries") {
		cfg.MaxRetries = maxRetries
	}
	if cmd.Flags().Changed("cooldown") {
		cfg.CooldownDuration = cooldown
	}

	// Get working directory
	cwd, err := os.Getwd()
	if err == nil {
		cfg.WorkDir = cwd
	}

	// Precheck: switch profile if near limit before running
	precheck, _ := cmd.Flags().GetBool("precheck")
	precheckThreshold, _ := cmd.Flags().GetFloat64("precheck-threshold")
	if precheck && (tool == "claude" || tool == "codex") {
		if switched := runPrecheck(tool, precheckThreshold, quiet, db, algorithm); switched && !quiet {
			fmt.Fprintf(os.Stderr, "caam: switched profile before running (usage was near limit)\n")
		}
	}

	// Create wrapper
	wrapper := wrap.NewWrapper(vault, db, healthStore, cfg)

	// Set up context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	// Run wrapped command
	result := wrapper.Run(ctx)

	// Handle result
	if result.Err != nil {
		return result.Err
	}

	// Exit with the same code as the wrapped command
	// Note: os.Exit bypasses defer, so close db explicitly first
	if result.ExitCode != 0 {
		if db != nil {
			db.Close()
		}
		os.Exit(result.ExitCode)
	}

	return nil
}

// runPrecheck checks current usage levels and switches profile if near limit.
// Returns true if a switch was performed.
func runPrecheck(tool string, threshold float64, quiet bool, db *caamdb.DB, algorithm rotation.Algorithm) bool {
	// Get current profile's access token
	vaultDir := authfile.DefaultVaultPath()

	// Get the currently active profile
	fileSet := tools[tool]()
	currentProfile, _ := vault.ActiveProfile(fileSet)
	if currentProfile == "" {
		return false // No active profile
	}

	// Load credentials for current profile
	credentials, err := usage.LoadProfileCredentials(vaultDir, tool)
	if err != nil || len(credentials) == 0 {
		return false
	}

	token, ok := credentials[currentProfile]
	if !ok {
		return false
	}

	// Fetch current usage
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	fetcher := usage.NewMultiProfileFetcher()
	results := fetcher.FetchAllProfiles(ctx, tool, map[string]string{currentProfile: token})

	if len(results) == 0 || results[0].Usage == nil {
		return false
	}

	currentUsage := results[0].Usage

	// Check if near limit
	if !currentUsage.IsNearLimit(threshold) {
		return false // All good, no switch needed
	}

	// Need to switch - find all profiles
	allProfiles, err := vault.List(tool)
	if err != nil || len(allProfiles) <= 1 {
		return false // Can't switch
	}

	// Find best alternative using usage-aware selection
	allCredentials, err := usage.LoadProfileCredentials(vaultDir, tool)
	if err != nil || len(allCredentials) == 0 {
		return false
	}

	// Fetch usage for all profiles
	allResults := fetcher.FetchAllProfiles(ctx, tool, allCredentials)

	// Convert to rotation.UsageInfo format
	usageData := make(map[string]*rotation.UsageInfo)
	for _, r := range allResults {
		if r.Usage == nil {
			continue
		}
		info := &rotation.UsageInfo{
			ProfileName: r.ProfileName,
			AvailScore:  r.Usage.AvailabilityScore(),
			Error:       r.Usage.Error,
		}
		if r.Usage.PrimaryWindow != nil {
			info.PrimaryPercent = r.Usage.PrimaryWindow.UsedPercent
		}
		if r.Usage.SecondaryWindow != nil {
			info.SecondaryPercent = r.Usage.SecondaryWindow.UsedPercent
		}
		usageData[r.ProfileName] = info
	}

	// Use rotation selector with usage data
	selector := rotation.NewSelector(algorithm, nil, db)
	selector.SetUsageData(usageData)

	result, err := selector.Select(tool, allProfiles, currentProfile)
	if err != nil || result.Selected == currentProfile {
		return false // Couldn't find better alternative
	}

	// Switch to the better profile
	if err := vault.Restore(fileSet, result.Selected); err != nil {
		return false
	}

	if !quiet {
		fmt.Fprintf(os.Stderr, "caam: precheck switched %s/%s -> %s/%s\n",
			tool, currentProfile, tool, result.Selected)
	}

	return true
}
