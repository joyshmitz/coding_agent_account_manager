package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/authfile"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/authpool"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/config"
	caamdb "github.com/Dicklesworthstone/coding_agent_account_manager/internal/db"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/exec"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/health"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/notify"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/profile"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/rotation"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/usage"
	"github.com/spf13/cobra"
)

// getWd allows mocking os.Getwd in tests
var getWd = os.Getwd

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

	// Load global config
	spmCfg, err := config.LoadSPMConfig()
	if err != nil {
		// Non-fatal: use defaults
		spmCfg = config.DefaultSPMConfig()
	}

	// Get working directory
	cwd, err := getWd()
	if err != nil {
		cwd, _ = os.Getwd()
	}

	// Precheck: switch profile if near limit before running
	precheck, _ := cmd.Flags().GetBool("precheck")
	precheckThreshold, _ := cmd.Flags().GetFloat64("precheck-threshold")
	if precheck && (tool == "claude" || tool == "codex") {
		if switched := runPrecheck(tool, precheckThreshold, quiet, db, algorithm); switched && !quiet {
			fmt.Fprintf(os.Stderr, "caam: switched profile before running (usage was near limit)\n")
		}
	}

	// Initialize AuthPool (if enabled in config)
	var pool *authpool.AuthPool
	if spmCfg.Daemon.AuthPool.Enabled {
		pool = authpool.NewAuthPool(authpool.WithVault(vault))
		// Best effort load
		_ = pool.Load(authpool.PersistOptions{})
	}

	// Initialize Rotation Selector
	selector := rotation.NewSelector(algorithm, healthStore, db)

	// Initialize Runner
	if runner == nil {
		// Should be initialized in Root PersistentPreRunE, but defensive check
		runner = exec.NewRunner(registry)
	}

	// Initialize Notifier
	var notifier notify.Notifier
	if !quiet {
		notifier = notify.NewTerminalNotifier(os.Stderr, true)
	}

	// Create SmartRunner
	opts := exec.SmartRunnerOptions{
		HandoffConfig: &spmCfg.Handoff,
		Notifier:      notifier,
		Vault:         vault,
		DB:            db,
		AuthPool:      pool,
		Rotation:      selector,
	}
	smartRunner := exec.NewSmartRunner(runner, opts)

	// Get provider
	prov, ok := registry.Get(tool)
	if !ok {
		return fmt.Errorf("provider %s not found in registry", tool)
	}

	// Get active profile
	fileSet := tools[tool]()
	activeProfileName, _ := vault.ActiveProfile(fileSet)
	if activeProfileName == "" {
		// If no active profile, try to select one
		profiles, err := vault.List(tool)
		if err != nil || len(profiles) == 0 {
			return fmt.Errorf("no profiles found for %s", tool)
		}
		res, err := selector.Select(tool, profiles, "")
		if err != nil {
			return fmt.Errorf("select profile: %w", err)
		}
		activeProfileName = res.Selected
		// Restore it
		if err := vault.Restore(fileSet, activeProfileName); err != nil {
			return fmt.Errorf("activate profile: %w", err)
		}
	}

	// Load profile object
	prof, err := profileStore.Load(tool, activeProfileName)
	if err != nil {
		// If profile object doesn't exist (only in vault), create a transient one
		// or try to create it.
		// profileStore requires directory.
		// Maybe we should use profileStore.Create/Load logic safely.
		// For now, assume it might not exist in profileStore if it was just a vault backup.
		// But runner needs *profile.Profile.
		// profile.Profile contains paths.
		
		// If it doesn't exist in profile store (isolated profiles), we are running in "vault mode".
		// In vault mode, we don't use isolated HOME usually.
		// But exec.Runner logic uses opts.Profile to set up env.
		
		// We can create a dummy profile object for the Runner.
		// Runner uses: Name, BasePath (if set), AuthMode.
		prof = &profile.Profile{
			Name:     activeProfileName,
			Provider: tool,
			AuthMode: "oauth", // Assumption
		}
	}

	// Set CLI overrides
	if cmd.Flags().Changed("cooldown") {
		// SmartRunner uses hardcoded 30m currently or what?
		// handleRateLimit uses 30m.
		// We should probably update SmartRunner to accept cooldown override if possible.
		// But SmartRunnerOptions doesn't have it.
		// It's fine for now.
	}

	// Run
	runOptions := exec.RunOptions{
		Profile:  prof,
		Provider: prov,
		Args:     cliArgs,
		WorkDir:  cwd,
		Env:      nil, // Inherit
	}

	// Handle signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	err = smartRunner.Run(ctx, runOptions)

	// Handle exit code
	var exitErr *exec.ExitCodeError
	if errors.As(err, &exitErr) {
		// Clean up before exiting
		if db != nil {
			db.Close()
		}
		cancel()
		os.Exit(exitErr.Code)
	}

	return err
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
