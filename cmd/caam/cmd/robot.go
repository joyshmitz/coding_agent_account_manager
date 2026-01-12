// Package cmd implements the CLI commands for caam.
package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/authfile"
	caamdb "github.com/Dicklesworthstone/coding_agent_account_manager/internal/db"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/health"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/version"
	"github.com/spf13/cobra"
)

// ============================================================================
// Robot Mode - Agent-Optimized CLI Interface
// ============================================================================
//
// Designed for coding agents (Claude, Codex, etc.) that need programmatic access
// to caam functionality. All output is JSON. No interactive prompts.
//
// Design principles:
// - JSON output by default (no --json flag needed)
// - Structured errors with error_code field
// - Actionable suggestions in output
// - Exit codes: 0=success, 1=error, 2=partial success
// - Compact but complete information

// RobotOutput is the standard response wrapper for all robot commands.
type RobotOutput struct {
	Success     bool        `json:"success"`
	Command     string      `json:"command"`
	Timestamp   string      `json:"timestamp"`
	Data        interface{} `json:"data,omitempty"`
	Error       *RobotError `json:"error,omitempty"`
	Suggestions []string    `json:"suggestions,omitempty"`
	Timing      *RobotTiming `json:"timing,omitempty"`
}

// RobotError provides structured error information.
type RobotError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// RobotTiming tracks execution time for performance monitoring.
type RobotTiming struct {
	StartedAt  string `json:"started_at"`
	DurationMs int64  `json:"duration_ms"`
}

// RobotStatusData contains the full status overview.
type RobotStatusData struct {
	Version      string              `json:"version"`
	OS           string              `json:"os"`
	Arch         string              `json:"arch"`
	VaultPath    string              `json:"vault_path"`
	ConfigPath   string              `json:"config_path"`
	Providers    []RobotProviderInfo `json:"providers"`
	Summary      RobotStatusSummary  `json:"summary"`
	Coordinators []RobotCoordinator  `json:"coordinators,omitempty"`
}

// RobotStatusSummary provides quick counts.
type RobotStatusSummary struct {
	TotalProfiles      int  `json:"total_profiles"`
	ActiveProfiles     int  `json:"active_profiles"`
	HealthyProfiles    int  `json:"healthy_profiles"`
	CooldownProfiles   int  `json:"cooldown_profiles"`
	ExpiringSoon       int  `json:"expiring_soon"` // < 24h
	AllProfilesBlocked bool `json:"all_profiles_blocked"`
}

// RobotProviderInfo contains provider-specific status.
type RobotProviderInfo struct {
	ID            string               `json:"id"`
	DisplayName   string               `json:"display_name"`
	LoggedIn      bool                 `json:"logged_in"`
	ActiveProfile string               `json:"active_profile,omitempty"`
	Profiles      []RobotProfileInfo   `json:"profiles"`
	AuthPaths     []RobotAuthPath      `json:"auth_paths"`
}

// RobotProfileInfo contains profile details optimized for agents.
type RobotProfileInfo struct {
	Name           string            `json:"name"`
	Active         bool              `json:"active"`
	System         bool              `json:"system"`
	Email          string            `json:"email,omitempty"`
	PlanType       string            `json:"plan_type,omitempty"`
	Health         RobotHealthInfo   `json:"health"`
	Cooldown       *RobotCooldown    `json:"cooldown,omitempty"`
	Recommendation string            `json:"recommendation,omitempty"`
}

// RobotHealthInfo contains health status.
type RobotHealthInfo struct {
	Status       string `json:"status"` // healthy, warning, critical, unknown
	Reason       string `json:"reason,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	ExpiresIn    string `json:"expires_in,omitempty"` // human-readable
	ErrorCount1h int    `json:"error_count_1h"`
}

// RobotCooldown contains cooldown information.
type RobotCooldown struct {
	Active       bool   `json:"active"`
	Until        string `json:"until,omitempty"`
	RemainingMs  int64  `json:"remaining_ms,omitempty"`
	RemainingStr string `json:"remaining_str,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

// RobotAuthPath shows auth file locations.
type RobotAuthPath struct {
	Path        string `json:"path"`
	Exists      bool   `json:"exists"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// RobotCoordinator contains coordinator status.
type RobotCoordinator struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Healthy  bool   `json:"healthy"`
	Latency  int64  `json:"latency_ms,omitempty"`
	Error    string `json:"error,omitempty"`
	Pending  int    `json:"pending_auth_requests"`
}

// RobotNextData contains recommended next action.
type RobotNextData struct {
	Provider        string            `json:"provider"`
	Profile         string            `json:"profile"`
	Score           float64           `json:"score"`
	Reasons         []string          `json:"reasons"`
	Command         string            `json:"command"`
	AlternateChoice *RobotNextProfile `json:"alternate,omitempty"`
}

// RobotNextProfile is an alternate profile option.
type RobotNextProfile struct {
	Provider string  `json:"provider"`
	Profile  string  `json:"profile"`
	Score    float64 `json:"score"`
	Reason   string  `json:"reason"`
}

// RobotActResult is the result of an action.
type RobotActResult struct {
	Action      string `json:"action"`
	Provider    string `json:"provider"`
	Profile     string `json:"profile"`
	OldProfile  string `json:"old_profile,omitempty"`
	Success     bool   `json:"success"`
	Message     string `json:"message"`
}

var robotCmd = &cobra.Command{
	Use:   "robot",
	Short: "Agent-optimized commands (JSON output)",
	Long: `Robot mode provides CLI commands optimized for coding agents.

All commands output JSON to stdout. Errors are structured with error codes.
No interactive prompts - designed for programmatic use.

Available subcommands:
  status   - Full system status overview
  next     - Suggest best profile to use
  act      - Execute an action (activate, cooldown, etc.)
  health   - Quick health check
  watch    - Stream status changes

Examples:
  caam robot status                    # Full status
  caam robot status --provider claude  # Claude only
  caam robot next claude               # Suggest next Claude profile
  caam robot act activate claude work  # Activate a profile
  caam robot health                    # Quick health check`,
}

var robotStatusCmd = &cobra.Command{
	Use:   "status [provider]",
	Short: "Full system status overview",
	Long: `Returns comprehensive status information in JSON format.

Includes:
- All profiles with health status
- Active profiles
- Cooldown information
- Token expiry status
- Coordinator status (if configured)
- Actionable suggestions

Use --provider to filter to a specific provider.
Use --compact for minimal output (IDs and status only).`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRobotStatus,
}

var robotNextCmd = &cobra.Command{
	Use:   "next <provider>",
	Short: "Suggest best profile to use",
	Long: `Analyzes all profiles for a provider and suggests the best one to use.

Scoring factors:
- Health status (healthy > warning > critical)
- Cooldown status (not in cooldown preferred)
- Token expiry (longer expiry preferred)
- Recent error count (fewer errors preferred)
- Last used time (LRU by default)

Returns the recommended profile with activation command.`,
	Args: cobra.ExactArgs(1),
	RunE: runRobotNext,
}

var robotActCmd = &cobra.Command{
	Use:   "act <action> <provider> [profile] [args...]",
	Short: "Execute an action",
	Long: `Execute an action and return the result.

Supported actions:
  activate <provider> <profile>  - Activate a profile
  cooldown <provider> <profile> [duration]  - Start cooldown
  uncooldown <provider> <profile>  - Clear cooldown
  refresh <provider> <profile>  - Refresh token
  backup <provider> <profile>   - Backup current auth

All actions return structured results with success/failure status.`,
	Args: cobra.MinimumNArgs(2),
	RunE: runRobotAct,
}

var robotHealthCmd = &cobra.Command{
	Use:   "health",
	Short: "Quick health check",
	Long: `Returns a quick health check suitable for monitoring.

Checks:
- Vault accessibility
- Profile store accessibility
- Token expiry status
- Cooldown status
- Coordinator connectivity (if configured)

Exit codes:
  0 - All healthy
  1 - Error running check
  2 - Health issues detected`,
	RunE: runRobotHealth,
}

var robotWatchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Stream status changes",
	Long: `Streams status updates as newline-delimited JSON.

Each line is a complete JSON object with the current status.
Updates are emitted on changes or at the poll interval.

Use --interval to set poll interval (default 5s).
Use --provider to filter to a specific provider.`,
	RunE: runRobotWatch,
}

func init() {
	rootCmd.AddCommand(robotCmd)
	robotCmd.AddCommand(robotStatusCmd)
	robotCmd.AddCommand(robotNextCmd)
	robotCmd.AddCommand(robotActCmd)
	robotCmd.AddCommand(robotHealthCmd)
	robotCmd.AddCommand(robotWatchCmd)

	// Status flags
	robotStatusCmd.Flags().String("provider", "", "filter to specific provider")
	robotStatusCmd.Flags().Bool("compact", false, "minimal output")
	robotStatusCmd.Flags().Bool("include-coordinators", false, "check coordinator status")

	// Next flags
	robotNextCmd.Flags().String("strategy", "smart", "selection strategy: smart, lru, random")
	robotNextCmd.Flags().Bool("include-cooldown", false, "include profiles in cooldown")

	// Watch flags
	robotWatchCmd.Flags().Int("interval", 5, "poll interval in seconds")
	robotWatchCmd.Flags().String("provider", "", "filter to specific provider")
}

// robotOutput writes a RobotOutput to stdout.
func robotOutput(cmd *cobra.Command, output RobotOutput) error {
	output.Timestamp = time.Now().UTC().Format(time.RFC3339)
	enc := json.NewEncoder(cmd.OutOrStdout())
	return enc.Encode(output)
}

// robotError creates an error output.
func robotError(cmd *cobra.Command, command string, code, message string, details string, suggestions []string) error {
	output := RobotOutput{
		Success: false,
		Command: command,
		Error: &RobotError{
			Code:    code,
			Message: message,
			Details: details,
		},
		Suggestions: suggestions,
	}
	robotOutput(cmd, output)
	return fmt.Errorf("%s: %s", code, message)
}

func runRobotStatus(cmd *cobra.Command, args []string) error {
	start := time.Now()
	providerFilter, _ := cmd.Flags().GetString("provider")
	compact, _ := cmd.Flags().GetBool("compact")
	includeCoords, _ := cmd.Flags().GetBool("include-coordinators")

	// Determine which providers to check
	providersToCheck := []string{"codex", "claude", "gemini"}
	if len(args) > 0 {
		providerFilter = strings.ToLower(args[0])
	}
	if providerFilter != "" {
		if _, ok := tools[providerFilter]; !ok {
			return robotError(cmd, "status", "INVALID_PROVIDER",
				fmt.Sprintf("unknown provider: %s", providerFilter),
				"valid providers: codex, claude, gemini",
				[]string{"caam robot status claude", "caam robot status codex", "caam robot status gemini"})
		}
		providersToCheck = []string{providerFilter}
	}

	data := RobotStatusData{
		Version:   version.Version,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		VaultPath: authfile.DefaultVaultPath(),
		Providers: make([]RobotProviderInfo, 0, len(providersToCheck)),
	}

	if configDir, err := os.UserConfigDir(); err == nil {
		data.ConfigPath = filepath.Join(configDir, "caam")
	}

	var suggestions []string

	for _, tool := range providersToCheck {
		provInfo := buildProviderInfo(tool, compact)
		data.Providers = append(data.Providers, provInfo)

		// Update summary
		data.Summary.TotalProfiles += len(provInfo.Profiles)
		if provInfo.ActiveProfile != "" {
			data.Summary.ActiveProfiles++
		}
		for _, p := range provInfo.Profiles {
			if p.Health.Status == "healthy" {
				data.Summary.HealthyProfiles++
			}
			if p.Cooldown != nil && p.Cooldown.Active {
				data.Summary.CooldownProfiles++
			}
			if p.Health.ExpiresAt != "" {
				if exp, err := time.Parse(time.RFC3339, p.Health.ExpiresAt); err == nil {
					if time.Until(exp) < 24*time.Hour {
						data.Summary.ExpiringSoon++
					}
				}
			}
		}
	}

	// Check if all profiles are blocked (cooldown or unhealthy)
	usableProfiles := data.Summary.HealthyProfiles - data.Summary.CooldownProfiles
	if data.Summary.TotalProfiles > 0 && usableProfiles == 0 {
		data.Summary.AllProfilesBlocked = true
		suggestions = append(suggestions, "All profiles are in cooldown or unhealthy. Consider adding a new profile or waiting for cooldown to expire.")
	}

	// Add suggestions based on status
	if data.Summary.ExpiringSoon > 0 {
		suggestions = append(suggestions, fmt.Sprintf("%d profile(s) expiring within 24h. Consider refreshing tokens.", data.Summary.ExpiringSoon))
	}

	// Check coordinators if requested
	if includeCoords {
		data.Coordinators = checkCoordinators()
	}

	duration := time.Since(start)
	output := RobotOutput{
		Success:     true,
		Command:     "status",
		Data:        data,
		Suggestions: suggestions,
		Timing: &RobotTiming{
			StartedAt:  start.UTC().Format(time.RFC3339),
			DurationMs: duration.Milliseconds(),
		},
	}

	return robotOutput(cmd, output)
}

func buildProviderInfo(tool string, compact bool) RobotProviderInfo {
	info := RobotProviderInfo{
		ID:          tool,
		DisplayName: getProviderDisplayName(tool),
		Profiles:    []RobotProfileInfo{},
		AuthPaths:   []RobotAuthPath{},
	}

	// Check if logged in
	fileSet := tools[tool]()
	info.LoggedIn = authfile.HasAuthFiles(fileSet)

	// Get active profile
	if info.LoggedIn {
		if activeProfile, err := vault.ActiveProfile(fileSet); err == nil && activeProfile != "" {
			info.ActiveProfile = activeProfile
		}
	}

	// Get auth paths (unless compact)
	if !compact {
		for _, spec := range fileSet.Files {
			_, err := os.Stat(spec.Path)
			info.AuthPaths = append(info.AuthPaths, RobotAuthPath{
				Path:        spec.Path,
				Exists:      err == nil,
				Required:    spec.Required,
				Description: spec.Description,
			})
		}
	}

	// List profiles
	profiles, err := vault.List(tool)
	if err != nil {
		return info
	}

	db, _ := caamdb.Open()
	defer func() {
		if db != nil {
			db.Close()
		}
	}()

	for _, profileName := range profiles {
		pInfo := buildProfileInfo(tool, profileName, info.ActiveProfile, db, compact)
		info.Profiles = append(info.Profiles, pInfo)
	}

	return info
}

func buildProfileInfo(tool, profileName, activeProfile string, db *caamdb.DB, compact bool) RobotProfileInfo {
	pInfo := RobotProfileInfo{
		Name:   profileName,
		Active: profileName == activeProfile,
		System: authfile.IsSystemProfile(profileName),
	}

	// Get health info
	ph, id := getProfileHealthWithIdentity(tool, profileName)
	status := health.CalculateStatus(ph)

	pInfo.Health = RobotHealthInfo{
		Status:       status.String(),
		ErrorCount1h: ph.ErrorCount1h,
	}

	if !compact {
		if !ph.TokenExpiresAt.IsZero() {
			pInfo.Health.ExpiresAt = ph.TokenExpiresAt.Format(time.RFC3339)
			remaining := time.Until(ph.TokenExpiresAt)
			if remaining > 0 {
				pInfo.Health.ExpiresIn = robotFormatDuration(remaining)
			} else {
				pInfo.Health.ExpiresIn = "expired"
				pInfo.Health.Reason = "token expired"
			}
		}

		// Set reason based on status
		if status == health.StatusWarning || status == health.StatusCritical {
			pInfo.Health.Reason = getHealthReason(ph, status)
		}
	}

	// Get identity info
	if id != nil {
		if compact {
			pInfo.Email = id.Email
		} else {
			pInfo.Email = id.Email
			pInfo.PlanType = id.PlanType
		}
	}

	// Check cooldown
	if db != nil {
		now := time.Now()
		if cooldown, err := db.ActiveCooldown(tool, profileName, now); err == nil && cooldown != nil {
			remaining := cooldown.CooldownUntil.Sub(now)
			if remaining > 0 {
				pInfo.Cooldown = &RobotCooldown{
					Active:       true,
					Until:        cooldown.CooldownUntil.Format(time.RFC3339),
					RemainingMs:  remaining.Milliseconds(),
					RemainingStr: robotFormatDuration(remaining),
					Reason:       cooldown.Notes,
				}
			}
		}
	}

	// Generate recommendation (unless compact)
	if !compact {
		pInfo.Recommendation = generateRecommendation(pInfo)
	}

	return pInfo
}

func getHealthReason(ph *health.ProfileHealth, status health.HealthStatus) string {
	// Use thresholds from health.DefaultHealthConfig()
	cfg := health.DefaultHealthConfig()

	if status == health.StatusCritical {
		if !ph.TokenExpiresAt.IsZero() && time.Until(ph.TokenExpiresAt) <= 0 {
			return "token expired"
		}
		if ph.ErrorCount1h >= cfg.ErrorCountCritical {
			return fmt.Sprintf("high error rate (%d errors in 1h)", ph.ErrorCount1h)
		}
	}
	if status == health.StatusWarning {
		if !ph.TokenExpiresAt.IsZero() && time.Until(ph.TokenExpiresAt) < 24*time.Hour {
			return "token expiring soon"
		}
		if ph.ErrorCount1h >= cfg.ErrorCountWarning {
			return fmt.Sprintf("elevated error rate (%d errors in 1h)", ph.ErrorCount1h)
		}
	}
	return ""
}

func generateRecommendation(p RobotProfileInfo) string {
	if p.Cooldown != nil && p.Cooldown.Active {
		return fmt.Sprintf("wait for cooldown (%s remaining)", p.Cooldown.RemainingStr)
	}
	if p.Health.Status == "critical" {
		if strings.Contains(p.Health.Reason, "expired") {
			return "refresh token required"
		}
		return "investigate errors before use"
	}
	if p.Health.Status == "warning" {
		if strings.Contains(p.Health.Reason, "expiring") {
			return "consider refreshing token soon"
		}
	}
	if p.Health.Status == "healthy" && !p.Active {
		return "ready to activate"
	}
	return ""
}

func robotFormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if m > 0 {
			return fmt.Sprintf("%dh%dm", h, m)
		}
		return fmt.Sprintf("%dh", h)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	if hours > 0 {
		return fmt.Sprintf("%dd%dh", days, hours)
	}
	return fmt.Sprintf("%dd", days)
}

func checkCoordinators() []RobotCoordinator {
	// Check known coordinator endpoints
	// This is a simplified version - in production, this would read from config
	endpoints := []struct {
		name string
		url  string
	}{
		{"local", "http://localhost:7890"},
	}

	var coords []RobotCoordinator
	client := &http.Client{Timeout: 2 * time.Second}

	for _, ep := range endpoints {
		coord := RobotCoordinator{
			Name: ep.name,
			URL:  ep.url,
		}

		start := time.Now()
		resp, err := client.Get(ep.url + "/status")
		coord.Latency = time.Since(start).Milliseconds()

		if err != nil {
			coord.Error = err.Error()
			coord.Healthy = false
		} else {
			resp.Body.Close()
			coord.Healthy = resp.StatusCode == http.StatusOK

			// Try to get pending count
			if pendResp, err := client.Get(ep.url + "/auth/pending"); err == nil {
				var pending []interface{}
				json.NewDecoder(pendResp.Body).Decode(&pending)
				pendResp.Body.Close()
				coord.Pending = len(pending)
			}
		}

		coords = append(coords, coord)
	}

	return coords
}

func runRobotNext(cmd *cobra.Command, args []string) error {
	start := time.Now()
	provider := strings.ToLower(args[0])

	if _, ok := tools[provider]; !ok {
		return robotError(cmd, "next", "INVALID_PROVIDER",
			fmt.Sprintf("unknown provider: %s", provider),
			"valid providers: codex, claude, gemini",
			nil)
	}

	strategy, _ := cmd.Flags().GetString("strategy")
	includeCooldown, _ := cmd.Flags().GetBool("include-cooldown")

	// Get all profiles for this provider
	profiles, err := vault.List(provider)
	if err != nil {
		return robotError(cmd, "next", "VAULT_ERROR",
			"failed to list profiles",
			err.Error(),
			[]string{"caam robot status " + provider})
	}

	if len(profiles) == 0 {
		return robotError(cmd, "next", "NO_PROFILES",
			fmt.Sprintf("no profiles found for %s", provider),
			"",
			[]string{
				"caam backup " + provider + " <profile-name>",
				"caam auth import " + provider,
			})
	}

	db, _ := caamdb.Open()
	defer func() {
		if db != nil {
			db.Close()
		}
	}()

	// Score each profile
	type scoredProfile struct {
		name    string
		score   float64
		reasons []string
		info    RobotProfileInfo
	}

	var scored []scoredProfile
	now := time.Now()

	for _, profileName := range profiles {
		pInfo := buildProfileInfo(provider, profileName, "", db, false)

		// Skip profiles in cooldown unless requested
		if !includeCooldown && pInfo.Cooldown != nil && pInfo.Cooldown.Active {
			continue
		}

		sp := scoredProfile{
			name:    profileName,
			info:    pInfo,
			reasons: []string{},
		}

		// Calculate score (higher is better)
		switch pInfo.Health.Status {
		case "healthy":
			sp.score += 100
			sp.reasons = append(sp.reasons, "healthy status")
		case "warning":
			sp.score += 50
			sp.reasons = append(sp.reasons, "warning status")
		case "critical":
			sp.score += 10
			sp.reasons = append(sp.reasons, "critical status (not recommended)")
		default:
			sp.score += 30
		}

		// Cooldown penalty
		if pInfo.Cooldown != nil && pInfo.Cooldown.Active {
			sp.score -= 200
			sp.reasons = append(sp.reasons, fmt.Sprintf("in cooldown (%s remaining)", pInfo.Cooldown.RemainingStr))
		}

		// Error penalty
		if pInfo.Health.ErrorCount1h > 0 {
			sp.score -= float64(pInfo.Health.ErrorCount1h * 10)
			sp.reasons = append(sp.reasons, fmt.Sprintf("%d recent errors", pInfo.Health.ErrorCount1h))
		}

		// Token expiry consideration
		if pInfo.Health.ExpiresAt != "" {
			if exp, err := time.Parse(time.RFC3339, pInfo.Health.ExpiresAt); err == nil {
				remaining := exp.Sub(now)
				if remaining > 7*24*time.Hour {
					sp.score += 20
					sp.reasons = append(sp.reasons, "token valid for >7d")
				} else if remaining > 24*time.Hour {
					sp.score += 10
					sp.reasons = append(sp.reasons, fmt.Sprintf("token expires in %s", robotFormatDuration(remaining)))
				} else if remaining > 0 {
					sp.score -= 20
					sp.reasons = append(sp.reasons, fmt.Sprintf("token expiring soon (%s)", robotFormatDuration(remaining)))
				} else {
					sp.score -= 100
					sp.reasons = append(sp.reasons, "token expired")
				}
			}
		}

		// LRU bonus (strategy-dependent)
		if strategy == "lru" || strategy == "smart" {
			// Could check last used time here
			// For now, just slightly favor non-active profiles
			if !pInfo.Active {
				sp.score += 5
			}
		}

		scored = append(scored, sp)
	}

	if len(scored) == 0 {
		suggestions := []string{
			fmt.Sprintf("caam robot status %s", provider),
		}
		if !includeCooldown {
			suggestions = append(suggestions, "caam robot next "+provider+" --include-cooldown")
		}
		return robotError(cmd, "next", "ALL_BLOCKED",
			"all profiles are blocked or in cooldown",
			"",
			suggestions)
	}

	// Sort by score (descending)
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	best := scored[0]
	data := RobotNextData{
		Provider: provider,
		Profile:  best.name,
		Score:    best.score,
		Reasons:  best.reasons,
		Command:  fmt.Sprintf("caam activate %s %s", provider, best.name),
	}

	// Include alternate if available
	if len(scored) > 1 {
		alt := scored[1]
		data.AlternateChoice = &RobotNextProfile{
			Provider: provider,
			Profile:  alt.name,
			Score:    alt.score,
			Reason:   strings.Join(alt.reasons, "; "),
		}
	}

	duration := time.Since(start)
	output := RobotOutput{
		Success: true,
		Command: "next",
		Data:    data,
		Timing: &RobotTiming{
			StartedAt:  start.UTC().Format(time.RFC3339),
			DurationMs: duration.Milliseconds(),
		},
	}

	return robotOutput(cmd, output)
}

func runRobotAct(cmd *cobra.Command, args []string) error {
	start := time.Now()
	action := strings.ToLower(args[0])
	provider := strings.ToLower(args[1])

	if _, ok := tools[provider]; !ok {
		return robotError(cmd, "act", "INVALID_PROVIDER",
			fmt.Sprintf("unknown provider: %s", provider),
			"valid providers: codex, claude, gemini",
			nil)
	}

	var result RobotActResult
	result.Action = action
	result.Provider = provider

	switch action {
	case "activate":
		if len(args) < 3 {
			return robotError(cmd, "act", "MISSING_PROFILE",
				"profile name required for activate",
				"usage: caam robot act activate <provider> <profile>",
				nil)
		}
		profile := args[2]
		result.Profile = profile

		// Get current active profile
		fileSet := tools[provider]()
		if oldProfile, err := vault.ActiveProfile(fileSet); err == nil {
			result.OldProfile = oldProfile
		}

		// Activate the profile
		if err := vault.Restore(fileSet, profile); err != nil {
			return robotError(cmd, "act", "ACTIVATE_FAILED",
				fmt.Sprintf("failed to activate %s/%s", provider, profile),
				err.Error(),
				[]string{fmt.Sprintf("caam robot status %s", provider)})
		}

		result.Success = true
		result.Message = fmt.Sprintf("activated %s/%s", provider, profile)

	case "cooldown":
		if len(args) < 3 {
			return robotError(cmd, "act", "MISSING_PROFILE",
				"profile name required for cooldown",
				"usage: caam robot act cooldown <provider> <profile> [duration]",
				nil)
		}
		profile := args[2]
		result.Profile = profile

		duration := 4 * time.Hour // default
		if len(args) >= 4 {
			if d, err := time.ParseDuration(args[3]); err == nil {
				duration = d
			}
		}

		db, err := caamdb.Open()
		if err != nil {
			return robotError(cmd, "act", "DB_ERROR",
				"failed to open database",
				err.Error(),
				nil)
		}
		defer db.Close()

		hitAt := time.Now()
		cooldownEvent, err := db.SetCooldown(provider, profile, hitAt, duration, "manual via robot act")
		if err != nil {
			return robotError(cmd, "act", "COOLDOWN_FAILED",
				"failed to set cooldown",
				err.Error(),
				nil)
		}

		result.Success = true
		result.Message = fmt.Sprintf("cooldown set until %s (%s)", cooldownEvent.CooldownUntil.Format(time.RFC3339), robotFormatDuration(duration))

	case "uncooldown":
		if len(args) < 3 {
			return robotError(cmd, "act", "MISSING_PROFILE",
				"profile name required for uncooldown",
				"usage: caam robot act uncooldown <provider> <profile>",
				nil)
		}
		profile := args[2]
		result.Profile = profile

		db, err := caamdb.Open()
		if err != nil {
			return robotError(cmd, "act", "DB_ERROR",
				"failed to open database",
				err.Error(),
				nil)
		}
		defer db.Close()

		if _, err := db.ClearCooldown(provider, profile); err != nil {
			return robotError(cmd, "act", "UNCOOLDOWN_FAILED",
				"failed to clear cooldown",
				err.Error(),
				nil)
		}

		result.Success = true
		result.Message = fmt.Sprintf("cleared cooldown for %s/%s", provider, profile)

	case "backup":
		fileSet := tools[provider]()
		if !authfile.HasAuthFiles(fileSet) {
			return robotError(cmd, "act", "NO_AUTH",
				fmt.Sprintf("no auth files found for %s", provider),
				"login first using the tool's login command",
				nil)
		}

		profile := "backup-" + time.Now().Format("20060102-150405")
		if len(args) >= 3 {
			profile = args[2]
		}
		result.Profile = profile

		if err := vault.Backup(fileSet, profile); err != nil {
			return robotError(cmd, "act", "BACKUP_FAILED",
				"backup failed",
				err.Error(),
				nil)
		}

		result.Success = true
		result.Message = fmt.Sprintf("backed up to %s/%s", provider, profile)

	default:
		return robotError(cmd, "act", "INVALID_ACTION",
			fmt.Sprintf("unknown action: %s", action),
			"valid actions: activate, cooldown, uncooldown, backup",
			[]string{
				"caam robot act activate <provider> <profile>",
				"caam robot act cooldown <provider> <profile> [duration]",
				"caam robot act uncooldown <provider> <profile>",
				"caam robot act backup <provider> [profile]",
			})
	}

	duration := time.Since(start)
	output := RobotOutput{
		Success: result.Success,
		Command: "act",
		Data:    result,
		Timing: &RobotTiming{
			StartedAt:  start.UTC().Format(time.RFC3339),
			DurationMs: duration.Milliseconds(),
		},
	}

	return robotOutput(cmd, output)
}

func runRobotHealth(cmd *cobra.Command, args []string) error {
	start := time.Now()

	type HealthCheck struct {
		Name    string `json:"name"`
		Status  string `json:"status"` // ok, warning, error
		Message string `json:"message,omitempty"`
	}

	type HealthCheckResult struct {
		Overall     string        `json:"overall"` // healthy, degraded, unhealthy
		Checks      []HealthCheck `json:"checks"`
		Issues      []string      `json:"issues,omitempty"`
		Suggestions []string      `json:"suggestions,omitempty"`
	}

	result := HealthCheckResult{
		Overall: "healthy",
		Checks:  []HealthCheck{},
	}

	// Check vault
	vaultPath := authfile.DefaultVaultPath()
	if _, err := os.Stat(vaultPath); err == nil {
		result.Checks = append(result.Checks, HealthCheck{
			Name:   "vault",
			Status: "ok",
		})
	} else {
		result.Checks = append(result.Checks, HealthCheck{
			Name:    "vault",
			Status:  "error",
			Message: "vault directory not found",
		})
		result.Issues = append(result.Issues, "vault directory not accessible")
		result.Overall = "unhealthy"
	}

	// Check database
	if db, err := caamdb.Open(); err == nil {
		db.Close()
		result.Checks = append(result.Checks, HealthCheck{
			Name:   "database",
			Status: "ok",
		})
	} else {
		result.Checks = append(result.Checks, HealthCheck{
			Name:    "database",
			Status:  "error",
			Message: err.Error(),
		})
		result.Issues = append(result.Issues, "database not accessible")
		if result.Overall == "healthy" {
			result.Overall = "degraded"
		}
	}

	// Check each provider
	for _, tool := range []string{"codex", "claude", "gemini"} {
		profiles, err := vault.List(tool)
		if err != nil {
			continue
		}

		healthyCount := 0
		totalCount := len(profiles)

		for _, profileName := range profiles {
			ph := buildProfileHealth(tool, profileName)
			status := health.CalculateStatus(ph)
			if status == health.StatusHealthy {
				healthyCount++
			}
		}

		if totalCount > 0 {
			if healthyCount == 0 {
				result.Checks = append(result.Checks, HealthCheck{
					Name:    tool,
					Status:  "warning",
					Message: fmt.Sprintf("0/%d profiles healthy", totalCount),
				})
				result.Issues = append(result.Issues, fmt.Sprintf("%s: no healthy profiles", tool))
				if result.Overall == "healthy" {
					result.Overall = "degraded"
				}
			} else if healthyCount < totalCount {
				result.Checks = append(result.Checks, HealthCheck{
					Name:    tool,
					Status:  "ok",
					Message: fmt.Sprintf("%d/%d profiles healthy", healthyCount, totalCount),
				})
			} else {
				result.Checks = append(result.Checks, HealthCheck{
					Name:   tool,
					Status: "ok",
				})
			}
		}
	}

	// Generate suggestions
	if len(result.Issues) > 0 {
		result.Suggestions = append(result.Suggestions, "caam robot status --include-coordinators")
		result.Suggestions = append(result.Suggestions, "caam doctor")
	}

	duration := time.Since(start)
	output := RobotOutput{
		Success: result.Overall != "unhealthy",
		Command: "health",
		Data:    result,
		Timing: &RobotTiming{
			StartedAt:  start.UTC().Format(time.RFC3339),
			DurationMs: duration.Milliseconds(),
		},
	}

	return robotOutput(cmd, output)
}

func runRobotWatch(cmd *cobra.Command, args []string) error {
	interval, _ := cmd.Flags().GetInt("interval")
	providerFilter, _ := cmd.Flags().GetString("provider")

	// Validate provider filter if specified
	if providerFilter != "" {
		providerFilter = strings.ToLower(providerFilter)
		if _, ok := tools[providerFilter]; !ok {
			return robotError(cmd, "watch", "INVALID_PROVIDER",
				fmt.Sprintf("unknown provider: %s", providerFilter),
				"valid providers: codex, claude, gemini",
				nil)
		}
	}

	if interval < 1 {
		interval = 1
	}

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	ctx := cmd.Context()

	// Emit initial status
	emitWatchStatus(cmd, providerFilter)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			emitWatchStatus(cmd, providerFilter)
		}
	}
}

func emitWatchStatus(cmd *cobra.Command, providerFilter string) {
	providersToCheck := []string{"codex", "claude", "gemini"}
	if providerFilter != "" {
		providersToCheck = []string{providerFilter}
	}

	type WatchEvent struct {
		Timestamp string              `json:"timestamp"`
		Providers []RobotProviderInfo `json:"providers"`
	}

	event := WatchEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Providers: make([]RobotProviderInfo, 0),
	}

	for _, tool := range providersToCheck {
		event.Providers = append(event.Providers, buildProviderInfo(tool, true))
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.Encode(event)
}
