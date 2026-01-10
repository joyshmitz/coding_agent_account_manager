package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/authfile"
	caamdb "github.com/Dicklesworthstone/coding_agent_account_manager/internal/db"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/testutil"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/wrap"
	"github.com/stretchr/testify/require"
)

// TestHelperProcess_Run is the entry point for the mock process for run tests.
func TestHelperProcess_Run(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	mode := os.Getenv("MOCK_RUN_MODE")
	switch mode {
	case "success":
		fmt.Println("Command success")
		os.Exit(0)
	case "ratelimit":
		fmt.Println("Error: rate limit exceeded")
		os.Exit(1)
	case "failover":
		// Fail first time (if profile is work), succeed second time (if profile is personal)
		// We can detect profile by checking auth file content or some marker
		// For simplicity, let's check an env var that we set in the test
		// But runOnce activates the profile, it doesn't set env vars for the subprocess 
		// except those passed to it.
		// However, runOnce activates the profile by restoring files.
		// We can check the content of the restored auth file.
		
		authPath := os.Getenv("MOCK_AUTH_PATH")
		content, _ := os.ReadFile(authPath)
		if string(content) == `{"token":"work"}` {
			fmt.Println("Error: rate limit exceeded")
			os.Exit(1)
		} else {
			fmt.Println("Command success (failover)")
			os.Exit(0)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s\n", mode)
		os.Exit(1)
	}
}

func TestRunCommand_Extended(t *testing.T) {
	h := testutil.NewExtendedHarness(t)
	defer h.Close()

	// 1. Setup
	h.StartStep("Setup", "Create vault and mock globals")
	
	rootDir := h.TempDir
	vaultDir := filepath.Join(rootDir, "vault")
	
	// Create database path
	dbDir := filepath.Join(rootDir, "db")
	require.NoError(t, os.MkdirAll(dbDir, 0755))
	
	// Override DB path
	h.SetEnv("HOME", rootDir)
	h.SetEnv("XDG_DATA_HOME", rootDir)
	configDir := filepath.Join(rootDir, "caam")
	h.SetEnv("CAAM_HOME", configDir)
	h.SetEnv("XDG_CONFIG_HOME", rootDir) // for config.json

	// Write config to set initial delay
	require.NoError(t, os.MkdirAll(filepath.Join(rootDir, "caam"), 0755))
	configPath := filepath.Join(rootDir, "caam", "config.json")
	configJSON := `{"wrap": {"initial_delay": "10ms"}}`
	require.NoError(t, os.WriteFile(configPath, []byte(configJSON), 0600))
	
	// Override vault and tools
	originalVault := vault
	originalTools := make(map[string]func() authfile.AuthFileSet)
	for k, v := range tools {
		originalTools[k] = v
	}
	originalExecCommand := wrap.ExecCommand
	originalGetWd := getWd
	originalAuthFileSetProvider := wrap.AuthFileSetForProvider
	defer func() {
		vault = originalVault
		tools = originalTools
		wrap.ExecCommand = originalExecCommand
		getWd = originalGetWd
		wrap.AuthFileSetForProvider = originalAuthFileSetProvider
	}()
	
	vault = authfile.NewVault(vaultDir)
	
	// Define target location for restore
	homeDir := filepath.Join(rootDir, "home")
	targetPath := filepath.Join(homeDir, "auth.json")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	
	// Setup profiles in vault
	// 1. Active profile (should be picked first alphabetically)
	activeDir := filepath.Join(vaultDir, "claude", "active_profile")
	require.NoError(t, os.MkdirAll(activeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(activeDir, "auth.json"), []byte(`{"token":"work"}`), 0600))
	
	// 2. Backup profile
	backupDir := filepath.Join(vaultDir, "claude", "backup_profile")
	require.NoError(t, os.MkdirAll(backupDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(backupDir, "auth.json"), []byte(`{"token":"personal"}`), 0600))
	
	tools["claude"] = func() authfile.AuthFileSet {
	
		return authfile.AuthFileSet{
	
			Tool: "claude",
	
			Files: []authfile.AuthFileSpec{
	
				{Path: targetPath, Required: true},
	
			},
	
		}
	
	}
	
	
	
	wrap.AuthFileSetForProvider = func(provider string) (authfile.AuthFileSet, bool) {
	
		if provider == "claude" {
	
			return tools["claude"](), true
	
		}
	
		return authfile.AuthFileSet{}, false
	
	}
	
	
	
	// Mock getWd
	

	getWd = func() (string, error) {
		return rootDir, nil
	}
	
h.EndStep("Setup")
	
	// 2. Test Success
	h.StartStep("Success", "Test successful run")
	
	wrap.ExecCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cs := []string{"-test.run", "^TestHelperProcess_Run$", "--", name}
		cs = append(cs, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "MOCK_RUN_MODE=success")
		return cmd
	}
	
	runCmd.Flags().Set("quiet", "true")
	err := runWrap(runCmd, []string{"claude", "prompt"})
	require.NoError(t, err)
	
h.EndStep("Success")
	
	// 3. Test Failover
	h.StartStep("Failover", "Test rate limit failover")
	
	// We need config to specify cooldown duration and retries
	runCmd.Flags().Set("max-retries", "1")
	runCmd.Flags().Set("cooldown", "30m")
	runCmd.Flags().Set("quiet", "true")
	runCmd.Flags().Set("algorithm", "round_robin")
	
	// Reset DB to clear any previous cooldowns
	db, _ := caamdb.Open()
	db.ClearAllCooldowns()
	db.Close()
	
	wrap.ExecCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cs := []string{"-test.run", "^TestHelperProcess_Run$", "--", name}
		cs = append(cs, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], cs...)
		cmd.Env = append(os.Environ(), 
			"GO_WANT_HELPER_PROCESS=1", 
			"MOCK_RUN_MODE=failover",
			"MOCK_AUTH_PATH="+targetPath,
		)
		return cmd
	}
	
	// Ensure we start with "work" profile active (so failover logic triggers correctly)
	// We can force this by making "work" the only active profile initially?
	// Or rely on rotation logic. Rotation prefers healthy profiles. Both are healthy.
	// We can manually activate "work" first.
	// runWrap logic: "Select initial profile".
	// It doesn't necessarily use the currently active one if it thinks another is better?
	// `rotation.NewSelector` with Smart algo might pick either.
	// To ensure determism, let's mark "personal" as "used recently" via DB?
	// Or just rely on the fact that `failover` mode checks file content.
	
	err = runWrap(runCmd, []string{"claude", "prompt"})
	require.NoError(t, err)
	
	// Verify "active_profile" is in cooldown
	db, err = caamdb.Open()
	require.NoError(t, err)
	defer db.Close()
	
	ev, err := db.ActiveCooldown("claude", "active_profile", time.Now())
	require.NoError(t, err)
	require.NotNil(t, ev, "Active profile should be in cooldown")
	
h.EndStep("Failover")
}
