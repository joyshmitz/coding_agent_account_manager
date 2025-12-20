package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	caamdb "github.com/Dicklesworthstone/coding_agent_account_manager/internal/db"
)

func TestCostCommand(t *testing.T) {
	if costCmd.Use != "cost" {
		t.Errorf("Expected Use 'cost', got %q", costCmd.Use)
	}

	if costCmd.Short == "" {
		t.Error("Expected non-empty Short description")
	}
}

func TestCostSubcommands(t *testing.T) {
	// Check sessions subcommand
	if costSessionsCmd.Use != "sessions" {
		t.Errorf("Expected Use 'sessions', got %q", costSessionsCmd.Use)
	}

	// Check rates subcommand
	if costRatesCmd.Use != "rates" {
		t.Errorf("Expected Use 'rates', got %q", costRatesCmd.Use)
	}
}

func TestCostFlags(t *testing.T) {
	// Check main command flags
	flags := []string{"provider", "since", "json"}
	for _, name := range flags {
		flag := costCmd.Flags().Lookup(name)
		if flag == nil {
			t.Errorf("Flag %q not found on cost command", name)
		}
	}

	// Check sessions flags
	sessionsFlags := []string{"limit", "provider", "since", "json"}
	for _, name := range sessionsFlags {
		flag := costSessionsCmd.Flags().Lookup(name)
		if flag == nil {
			t.Errorf("Flag %q not found on cost sessions command", name)
		}
	}

	// Check rates flags
	ratesFlags := []string{"json", "set", "per-minute", "per-session"}
	for _, name := range ratesFlags {
		flag := costRatesCmd.Flags().Lookup(name)
		if flag == nil {
			t.Errorf("Flag %q not found on cost rates command", name)
		}
	}
}

func TestFormatDollars(t *testing.T) {
	tests := []struct {
		cents    int
		expected string
	}{
		{0, "$0.00"},
		{1, "$0.01"},
		{50, "$0.50"},
		{100, "$1.00"},
		{150, "$1.50"},
		{1234, "$12.34"},
		{10000, "$100.00"},
	}

	for _, tc := range tests {
		got := formatDollars(tc.cents)
		if got != tc.expected {
			t.Errorf("formatDollars(%d) = %q, want %q", tc.cents, got, tc.expected)
		}
	}
}

func TestRenderCostSummary_Empty(t *testing.T) {
	var buf bytes.Buffer
	err := renderCostSummary(&buf, nil, time.Time{})
	if err != nil {
		t.Fatalf("renderCostSummary() error = %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("Expected non-empty output for empty summaries")
	}
}

func TestRenderCostSummaryJSON_Empty(t *testing.T) {
	var buf bytes.Buffer
	err := renderCostSummaryJSON(&buf, nil, time.Time{})
	if err != nil {
		t.Fatalf("renderCostSummaryJSON() error = %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("Expected non-empty JSON output")
	}
}

func TestRenderSessions_Empty(t *testing.T) {
	var buf bytes.Buffer
	err := renderSessions(&buf, nil)
	if err != nil {
		t.Fatalf("renderSessions() error = %v", err)
	}

	output := buf.String()
	// Should still have header
	if output == "" {
		t.Error("Expected header row even with empty sessions")
	}
}

func TestRenderSessionsJSON_Empty(t *testing.T) {
	var buf bytes.Buffer
	err := renderSessionsJSON(&buf, nil)
	if err != nil {
		t.Fatalf("renderSessionsJSON() error = %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("Expected non-empty JSON output")
	}
}

func TestRenderRates(t *testing.T) {
	rates := []caamdb.CostRate{
		{Provider: "claude", CentsPerMinute: 5, CentsPerSession: 0, UpdatedAt: time.Now()},
		{Provider: "codex", CentsPerMinute: 3, CentsPerSession: 10, UpdatedAt: time.Now()},
	}

	var buf bytes.Buffer
	err := renderRates(&buf, rates)
	if err != nil {
		t.Fatalf("renderRates() error = %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("Expected non-empty output")
	}
}

func TestRenderRatesJSON(t *testing.T) {
	rates := []caamdb.CostRate{
		{Provider: "claude", CentsPerMinute: 5, CentsPerSession: 0, UpdatedAt: time.Now()},
	}

	var buf bytes.Buffer
	err := renderRatesJSON(&buf, rates)
	if err != nil {
		t.Fatalf("renderRatesJSON() error = %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("Expected non-empty JSON output")
	}
}

func TestCostRatesCommand_SetRate(t *testing.T) {
	// Set up temp environment
	tmpDir := t.TempDir()
	oldCaamHome := os.Getenv("CAAM_HOME")
	os.Setenv("CAAM_HOME", tmpDir)
	defer os.Setenv("CAAM_HOME", oldCaamHome)

	// Create DB directory
	dbDir := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(dbDir, 0700); err != nil {
		t.Fatalf("mkdir error: %v", err)
	}

	// Open DB to run migrations and test SetCostRate directly
	db, err := caamdb.OpenAt(filepath.Join(dbDir, "caam.db"))
	if err != nil {
		t.Fatalf("OpenAt error: %v", err)
	}
	defer db.Close()

	// Test SetCostRate function directly
	if err := db.SetCostRate("claude", 10, 5); err != nil {
		t.Fatalf("SetCostRate error: %v", err)
	}

	// Verify rate was set
	rate, err := db.GetCostRate("claude")
	if err != nil {
		t.Fatalf("GetCostRate error: %v", err)
	}

	if rate.CentsPerMinute != 10 {
		t.Errorf("CentsPerMinute = %d, want 10", rate.CentsPerMinute)
	}
	if rate.CentsPerSession != 5 {
		t.Errorf("CentsPerSession = %d, want 5", rate.CentsPerSession)
	}
}
