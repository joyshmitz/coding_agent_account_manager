// Package cmd implements the CLI commands for caam.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/provider"
)

// AuthDetectResult represents the result of auth detection for one provider.
type AuthDetectResult struct {
	Provider  string                    `json:"provider"`
	Found     bool                      `json:"found"`
	Locations []AuthDetectLocation      `json:"locations"`
	Primary   *AuthDetectLocation       `json:"primary,omitempty"`
	Warning   string                    `json:"warning,omitempty"`
	Error     string                    `json:"error,omitempty"`
}

// AuthDetectLocation represents a detected auth file location.
type AuthDetectLocation struct {
	Path            string `json:"path"`
	Exists          bool   `json:"exists"`
	LastModified    string `json:"last_modified,omitempty"`
	FileSize        int64  `json:"file_size,omitempty"`
	IsValid         bool   `json:"is_valid"`
	ValidationError string `json:"validation_error,omitempty"`
	Description     string `json:"description"`
}

// AuthDetectReport contains the results of auth detection for all providers.
type AuthDetectReport struct {
	Timestamp string             `json:"timestamp"`
	Results   []AuthDetectResult `json:"results"`
	Summary   AuthDetectSummary  `json:"summary"`
}

// AuthDetectSummary provides a summary of detected auth.
type AuthDetectSummary struct {
	TotalProviders int `json:"total_providers"`
	FoundCount     int `json:"found_count"`
	NotFoundCount  int `json:"not_found_count"`
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication credentials",
	Long: `Commands for managing authentication credentials.

Subcommands:
  detect  - Detect existing auth files in system locations
  import  - Import detected auth into a caam profile (coming soon)`,
}

var authDetectCmd = &cobra.Command{
	Use:   "detect [tool]",
	Short: "Detect existing auth files",
	Long: `Detect existing authentication files in standard system locations.

This scans for existing auth files from direct CLI tool usage:
  - Claude: ~/.claude.json, ~/.config/claude-code/auth.json
  - Codex: ~/.codex/auth.json
  - Gemini: ~/.gemini/settings.json, ~/.gemini/.env, gcloud ADC

If a tool argument is provided, only that tool is checked.
Otherwise, all supported tools are scanned.

Examples:
  caam auth detect           # Detect all providers
  caam auth detect claude    # Detect Claude auth only
  caam auth detect --json    # Output as JSON

This is useful for first-run experience to discover and import existing credentials.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOutput, _ := cmd.Flags().GetBool("json")

		var providersToCheck []provider.Provider

		if len(args) > 0 {
			// Check specific provider
			p, ok := registry.Get(args[0])
			if !ok {
				return fmt.Errorf("unknown tool: %s (supported: claude, codex, gemini)", args[0])
			}
			providersToCheck = append(providersToCheck, p)
		} else {
			// Check all providers
			providersToCheck = registry.All()
		}

		report := runAuthDetection(providersToCheck)

		if jsonOutput {
			data, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		printAuthDetectReport(report)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authDetectCmd)
	authDetectCmd.Flags().Bool("json", false, "output in JSON format")
}

func runAuthDetection(providers []provider.Provider) *AuthDetectReport {
	report := &AuthDetectReport{
		Timestamp: time.Now().Format(time.RFC3339),
		Results:   make([]AuthDetectResult, 0, len(providers)),
	}

	for _, p := range providers {
		result := AuthDetectResult{
			Provider:  p.ID(),
			Locations: []AuthDetectLocation{},
		}

		detection, err := p.DetectExistingAuth()
		if err != nil {
			result.Error = err.Error()
			report.Results = append(report.Results, result)
			continue
		}

		result.Found = detection.Found
		result.Warning = detection.Warning

		for _, loc := range detection.Locations {
			detectLoc := AuthDetectLocation{
				Path:            loc.Path,
				Exists:          loc.Exists,
				FileSize:        loc.FileSize,
				IsValid:         loc.IsValid,
				ValidationError: loc.ValidationError,
				Description:     loc.Description,
			}
			if !loc.LastModified.IsZero() {
				detectLoc.LastModified = loc.LastModified.Format(time.RFC3339)
			}
			result.Locations = append(result.Locations, detectLoc)
		}

		if detection.Primary != nil {
			primary := AuthDetectLocation{
				Path:            detection.Primary.Path,
				Exists:          detection.Primary.Exists,
				FileSize:        detection.Primary.FileSize,
				IsValid:         detection.Primary.IsValid,
				ValidationError: detection.Primary.ValidationError,
				Description:     detection.Primary.Description,
			}
			if !detection.Primary.LastModified.IsZero() {
				primary.LastModified = detection.Primary.LastModified.Format(time.RFC3339)
			}
			result.Primary = &primary
		}

		report.Results = append(report.Results, result)

		if detection.Found {
			report.Summary.FoundCount++
		} else {
			report.Summary.NotFoundCount++
		}
	}

	report.Summary.TotalProviders = len(providers)

	return report
}

func printAuthDetectReport(report *AuthDetectReport) {
	fmt.Println("Detecting existing auth credentials...")
	fmt.Println()

	for _, result := range report.Results {
		displayName := getProviderDisplayName(result.Provider)
		fmt.Printf("%s:\n", displayName)

		if result.Error != "" {
			fmt.Printf("  ✗ Error: %s\n", result.Error)
			fmt.Println()
			continue
		}

		if !result.Found {
			fmt.Println("  ✗ No existing auth detected")
			// Show checked locations
			if len(result.Locations) > 0 {
				fmt.Println("    Checked:")
				for _, loc := range result.Locations {
					fmt.Printf("    - %s\n", shortenPath(loc.Path))
				}
			}
			fmt.Println()
			continue
		}

		// Show found auth files
		for _, loc := range result.Locations {
			if !loc.Exists {
				continue
			}

			statusIcon := "✓"
			status := "Valid"
			if !loc.IsValid {
				statusIcon = "⚠"
				status = loc.ValidationError
			}

			fmt.Printf("  %s %s\n", statusIcon, shortenPath(loc.Path))
			if loc.LastModified != "" {
				t, err := time.Parse(time.RFC3339, loc.LastModified)
				if err == nil {
					fmt.Printf("    Last modified: %s\n", t.Format("2006-01-02 15:04:05"))
				}
			}
			fmt.Printf("    Size: %s\n", formatFileSize(loc.FileSize))
			fmt.Printf("    Status: %s\n", status)
		}

		if result.Warning != "" {
			fmt.Printf("  ⚠ %s\n", result.Warning)
		}

		fmt.Println()
	}

	// Summary
	fmt.Printf("Summary: %d provider(s) checked, %d with auth, %d without\n",
		report.Summary.TotalProviders,
		report.Summary.FoundCount,
		report.Summary.NotFoundCount)

	if report.Summary.FoundCount > 0 {
		fmt.Println("\nRun 'caam auth import <tool>' to import detected credentials into a profile.")
	}
}

func getProviderDisplayName(id string) string {
	meta, ok := provider.GetProviderMeta(id)
	if ok {
		return meta.DisplayName
	}
	return strings.Title(id)
}

func shortenPath(path string) string {
	// Replace home directory with ~
	homeDir := ""
	if h, err := getHomeDir(); err == nil {
		homeDir = h
	}
	if homeDir != "" && strings.HasPrefix(path, homeDir) {
		return "~" + path[len(homeDir):]
	}
	return path
}

func getHomeDir() (string, error) {
	// Use os.UserHomeDir - this is a wrapper to avoid importing os here
	// since we already have access to it via provider implementations
	// For now, just try to get it from environment
	if home := getEnv("HOME"); home != "" {
		return home, nil
	}
	if userProfile := getEnv("USERPROFILE"); userProfile != "" {
		return userProfile, nil
	}
	return "", fmt.Errorf("home directory not found")
}

func getEnv(key string) string {
	// Simple wrapper - we can use os.Getenv directly but this keeps the code clean
	// and allows for future abstraction if needed
	return envLookup(key)
}

// envLookup is a variable so it can be mocked in tests
var envLookup = func(key string) string {
	return os.Getenv(key)
}

func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
