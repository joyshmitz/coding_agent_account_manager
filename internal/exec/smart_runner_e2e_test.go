package exec

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/authfile"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/config"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/notify"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/profile"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/provider"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/rotation"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMockCLI_Handoff simulates a CLI tool that hits a rate limit and then accepts login.
func TestMockCLI_Handoff(t *testing.T) {
	if os.Getenv("GO_WANT_MOCK_CLI") != "1" {
		return
	}

	// 1. Output rate limit message
	fmt.Println("Processing...")
	time.Sleep(100 * time.Millisecond)
	fmt.Println("Error: rate limit exceeded")

	// 2. Wait for login command
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	
	if strings.TrimSpace(line) == "/login" {
		fmt.Println("Logging in...")
		time.Sleep(100 * time.Millisecond)
		fmt.Println("successfully logged in")
	} else {
		fmt.Printf("Unknown command: %s", line)
		os.Exit(1)
	}
	
	// Keep running a bit
	time.Sleep(500 * time.Millisecond)
}

type MockNotifier struct {
	Alerts []*notify.Alert
}

func (m *MockNotifier) Notify(alert *notify.Alert) error {
	m.Alerts = append(m.Alerts, alert)
	return nil
}
func (m *MockNotifier) Name() string { return "mock" }
func (m *MockNotifier) Available() bool { return true }

func TestSmartRunner_E2E(t *testing.T) {
	h := testutil.NewExtendedHarness(t)
	defer h.Close()

	// 1. Setup
	h.StartStep("Setup", "Initialize environment")
	rootDir := h.TempDir
	vaultDir := filepath.Join(rootDir, "vault")
	
	// Setup profiles
	// Profile 1 (Current): "active"
	// Profile 2 (Backup): "backup"
	createProfile := func(name string) {
		dir := filepath.Join(vaultDir, "claude", name)
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".claude.json"), []byte("{}"), 0600))
	}
	createProfile("active")
	createProfile("backup")
	
	vault := authfile.NewVault(vaultDir)
	
	// Mock ExecCommand
	originalExec := ExecCommand
	defer func() { ExecCommand = originalExec }()
	
	ExecCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		fmt.Println("DEBUG: ExecCommand called")
		cs := []string{"-test.run=^TestMockCLI_Handoff$", "--"}
		cs = append(cs, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_MOCK_CLI=1")
		return cmd
	}
	
	// Setup SmartRunner
	cfg := config.DefaultSPMConfig().Handoff
	notifier := &MockNotifier{}
	
	// Need mock provider registry?
	// SmartRunner.Run uses opts.Provider.
	// We need a provider that returns "claude" ID.
	// We can use the real Claude provider, but we need to ensure it uses our mocked paths?
	// `DetectExistingAuth` etc. are not used by `SmartRunner.Run` directly, only `Runner.Run`.
	// `Runner.Run` calls `opts.Provider.Env`.
	// `internal/provider/claude/claude.go` implements `Env`.
	
	// Using a mock provider is safer to avoid file system dependency issues.
	mockProv := &MockProvider{id: "claude"} // Reuse MockProvider if exported or define locally
	
	// SmartRunner needs rotation selector
	// Selector needs health store and db
	// We can use nil DB and empty health store
	selector := rotation.NewSelector(rotation.AlgorithmRoundRobin, nil, nil)
	
	runner := &Runner{}
	
	opts := SmartRunnerOptions{
		HandoffConfig: &cfg,
		Vault:         vault,
		Rotation:      selector,
		Notifier:      notifier,
	}
	
	sr := NewSmartRunner(runner, opts)
	
	// Prepare RunOptions
	prof, err := profile.NewStore(filepath.Join(rootDir, "profiles")).Create("claude", "active", "oauth")
	require.NoError(t, err)
	
	runOpts := RunOptions{
		Profile:  prof,
		Provider: mockProv,
		Args:     []string{},
		Env:      map[string]string{"GO_WANT_MOCK_CLI": "1"},
	}
	
h.EndStep("Setup")
	
	// 2. Run
	h.StartStep("Run", "Execute SmartRunner")
	
	// Run should:
	// 1. Start mock CLI
	// 2. Detect "rate limit exceeded"
	// 3. Trigger handoff
	// 4. Select "backup"
	// 5. Swap auth (mocked vault works on temp dir)
	// 6. Inject "/login"
	// 7. Detect "successfully logged in"
	// 8. Notify user
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	err = sr.Run(ctx, runOpts)
	require.NoError(t, err)
	
	h.EndStep("Run")
	
	// 3. Verify
	h.StartStep("Verify", "Check state and notifications")
	
	assert.Equal(t, "backup", sr.currentProfile)
	assert.Equal(t, 1, sr.handoffCount)
	
	// Check notifications
	require.NotEmpty(t, notifier.Alerts)
	foundSwitch := false
	for _, a := range notifier.Alerts {
		if strings.Contains(a.Message, "Switched to backup") {
			foundSwitch = true
		}
	}
	assert.True(t, foundSwitch, "Did not notify about switch")
	
	h.EndStep("Verify")
}

// Local MockProvider (minimal)
type MockProvider struct {
	id string
}
func (m *MockProvider) ID() string { return m.id }
func (m *MockProvider) DisplayName() string { return "Mock" }
func (m *MockProvider) DefaultBin() string { return "mock-bin" }
func (m *MockProvider) Env(ctx context.Context, p *profile.Profile) (map[string]string, error) { return nil, nil }
// Other methods needed for interface compliance...
func (m *MockProvider) SupportedAuthModes() []provider.AuthMode { return nil }
func (m *MockProvider) AuthFiles() []provider.AuthFileSpec { return nil }
func (m *MockProvider) PrepareProfile(ctx context.Context, p *profile.Profile) error { return nil }
func (m *MockProvider) Login(ctx context.Context, p *profile.Profile) error { return nil }
func (m *MockProvider) Logout(ctx context.Context, p *profile.Profile) error { return nil }
func (m *MockProvider) Status(ctx context.Context, p *profile.Profile) (*provider.ProfileStatus, error) { return nil, nil }
func (m *MockProvider) ValidateProfile(ctx context.Context, p *profile.Profile) error { return nil }
func (m *MockProvider) DetectExistingAuth() (*provider.AuthDetection, error) { return nil, nil }
func (m *MockProvider) ImportAuth(ctx context.Context, s string, p *profile.Profile) ([]string, error) { return nil, nil }
func (m *MockProvider) ValidateToken(ctx context.Context, p *profile.Profile, passive bool) (*provider.ValidationResult, error) { return nil, nil }
