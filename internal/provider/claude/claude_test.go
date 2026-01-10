package claude

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/profile"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/provider"
)

// =============================================================================
// Provider Factory Tests
// =============================================================================

func TestNew(t *testing.T) {
	p := New()
	if p == nil {
		t.Fatal("New() returned nil")
	}
}

// =============================================================================
// Provider Identity Tests
// =============================================================================

func TestProviderID(t *testing.T) {
	p := New()
	if p.ID() != "claude" {
		t.Errorf("ID() = %q, want %q", p.ID(), "claude")
	}
}

func TestProviderDisplayName(t *testing.T) {
	p := New()
	expected := "Claude Code (Anthropic Claude Max)"
	if p.DisplayName() != expected {
		t.Errorf("DisplayName() = %q, want %q", p.DisplayName(), expected)
	}
}

func TestProviderDefaultBin(t *testing.T) {
	p := New()
	if p.DefaultBin() != "claude" {
		t.Errorf("DefaultBin() = %q, want %q", p.DefaultBin(), "claude")
	}
}

// =============================================================================
// Auth Mode Tests
// =============================================================================

func TestSupportedAuthModes(t *testing.T) {
	p := New()
	modes := p.SupportedAuthModes()

	if len(modes) != 2 {
		t.Fatalf("SupportedAuthModes() returned %d modes, want 2", len(modes))
	}

	hasOAuth := false
	hasAPIKey := false
	for _, mode := range modes {
		if mode == provider.AuthModeOAuth {
			hasOAuth = true
		}
		if mode == provider.AuthModeAPIKey {
			hasAPIKey = true
		}
	}

	if !hasOAuth {
		t.Error("SupportedAuthModes() should include OAuth")
	}
	if !hasAPIKey {
		t.Error("SupportedAuthModes() should include APIKey")
	}
}

// =============================================================================
// Auth Files Tests
// =============================================================================

func TestAuthFiles(t *testing.T) {
	t.Run("returns four auth file specs", func(t *testing.T) {
		p := New()
		files := p.AuthFiles()

		if len(files) != 4 {
			t.Fatalf("AuthFiles() returned %d files, want 4", len(files))
		}
	})

	t.Run("first file is .credentials.json and required", func(t *testing.T) {
		p := New()
		files := p.AuthFiles()

		file := files[0]
		if !strings.HasSuffix(file.Path, filepath.Join(".claude", ".credentials.json")) {
			t.Errorf("AuthFiles()[0].Path = %q, should end with .claude/.credentials.json", file.Path)
		}
		if !file.Required {
			t.Error(".credentials.json should be required")
		}
	})

	t.Run("second file is .claude.json and optional", func(t *testing.T) {
		p := New()
		files := p.AuthFiles()

		file := files[1]
		if !strings.HasSuffix(file.Path, ".claude.json") {
			t.Errorf("AuthFiles()[1].Path = %q, should end with .claude.json", file.Path)
		}
		if file.Required {
			t.Error(".claude.json should be optional")
		}
	})

	t.Run("third file is auth.json and optional", func(t *testing.T) {
		p := New()
		files := p.AuthFiles()

		file := files[2]
		if !strings.HasSuffix(file.Path, "claude-code/auth.json") {
			t.Errorf("AuthFiles()[2].Path = %q, should end with claude-code/auth.json", file.Path)
		}
		if file.Required {
			t.Error("claude-code/auth.json should be optional")
		}
	})

	t.Run("fourth file is settings.json and optional", func(t *testing.T) {
		p := New()
		files := p.AuthFiles()

		file := files[3]
		if !strings.HasSuffix(file.Path, filepath.Join(".claude", "settings.json")) {
			t.Errorf("AuthFiles()[3].Path = %q, should end with .claude/settings.json", file.Path)
		}
		if file.Required {
			t.Error(".claude/settings.json should be optional")
		}
	})

	t.Run("uses XDG_CONFIG_HOME if set", func(t *testing.T) {
		originalXDG := os.Getenv("XDG_CONFIG_HOME")
		defer os.Setenv("XDG_CONFIG_HOME", originalXDG)

		os.Setenv("XDG_CONFIG_HOME", "/custom/config")
		p := New()
		files := p.AuthFiles()

		expected := "/custom/config/claude-code/auth.json"
		if files[2].Path != expected {
			t.Errorf("AuthFiles()[2].Path = %q, want %q", files[2].Path, expected)
		}
	})

	t.Run("uses default .config if XDG_CONFIG_HOME not set", func(t *testing.T) {
		originalXDG := os.Getenv("XDG_CONFIG_HOME")
		defer os.Setenv("XDG_CONFIG_HOME", originalXDG)

		os.Unsetenv("XDG_CONFIG_HOME")
		p := New()
		files := p.AuthFiles()

		homeDir, _ := os.UserHomeDir()
		expected := filepath.Join(homeDir, ".config", "claude-code", "auth.json")
		if files[2].Path != expected {
			t.Errorf("AuthFiles()[2].Path = %q, want %q", files[2].Path, expected)
		}
	})
}

// =============================================================================
// PrepareProfile Tests
// =============================================================================

func TestPrepareProfile(t *testing.T) {
	t.Run("creates home directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "claude",
			BasePath: tmpDir,
		}

		p := New()
		if err := p.PrepareProfile(context.Background(), prof); err != nil {
			t.Fatalf("PrepareProfile() error = %v", err)
		}

		homePath := prof.HomePath()
		info, err := os.Stat(homePath)
		if err != nil {
			t.Fatalf("home not created: %v", err)
		}
		if !info.IsDir() {
			t.Error("home should be a directory")
		}
	})

	t.Run("creates xdg_config directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "claude",
			BasePath: tmpDir,
		}

		p := New()
		p.PrepareProfile(context.Background(), prof)

		xdgPath := prof.XDGConfigPath()
		info, err := os.Stat(xdgPath)
		if err != nil {
			t.Fatalf("xdg_config not created: %v", err)
		}
		if !info.IsDir() {
			t.Error("xdg_config should be a directory")
		}
	})

	t.Run("creates claude-code directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "claude",
			BasePath: tmpDir,
		}

		p := New()
		p.PrepareProfile(context.Background(), prof)

		claudeCodeDir := filepath.Join(prof.XDGConfigPath(), "claude-code")
		info, err := os.Stat(claudeCodeDir)
		if err != nil {
			t.Fatalf("claude-code dir not created: %v", err)
		}
		if !info.IsDir() {
			t.Error("claude-code should be a directory")
		}
	})

	t.Run("creates .claude directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "claude",
			BasePath: tmpDir,
		}

		p := New()
		p.PrepareProfile(context.Background(), prof)

		claudeDir := filepath.Join(prof.HomePath(), ".claude")
		info, err := os.Stat(claudeDir)
		if err != nil {
			t.Fatalf(".claude dir not created: %v", err)
		}
		if !info.IsDir() {
			t.Error(".claude should be a directory")
		}
	})

	t.Run("sets secure permissions", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "claude",
			BasePath: tmpDir,
		}

		p := New()
		p.PrepareProfile(context.Background(), prof)

		// Check home directory permissions
		homePath := prof.HomePath()
		info, _ := os.Stat(homePath)
		if info.Mode().Perm() != 0700 {
			t.Errorf("home permissions = %o, want 0700", info.Mode().Perm())
		}

		// Check xdg_config permissions
		xdgPath := prof.XDGConfigPath()
		info, _ = os.Stat(xdgPath)
		if info.Mode().Perm() != 0700 {
			t.Errorf("xdg_config permissions = %o, want 0700", info.Mode().Perm())
		}
	})

	t.Run("idempotent - can be called multiple times", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "claude",
			BasePath: tmpDir,
		}

		p := New()
		p.PrepareProfile(context.Background(), prof)
		if err := p.PrepareProfile(context.Background(), prof); err != nil {
			t.Errorf("second PrepareProfile() error = %v", err)
		}
	})
}

// =============================================================================
// API Key Helper Tests
// =============================================================================

func TestPrepareProfileWithAPIKey(t *testing.T) {
	t.Run("creates api_key_helper script", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "claude",
			AuthMode: string(provider.AuthModeAPIKey),
			BasePath: tmpDir,
		}

		p := New()
		if err := p.PrepareProfile(context.Background(), prof); err != nil {
			t.Fatalf("PrepareProfile() error = %v", err)
		}

		helperPath := filepath.Join(tmpDir, "api_key_helper.sh")
		info, err := os.Stat(helperPath)
		if err != nil {
			t.Fatalf("api_key_helper.sh not created: %v", err)
		}
		// Should be executable
		if info.Mode().Perm()&0100 == 0 {
			t.Error("api_key_helper.sh should be executable")
		}
	})

	t.Run("creates settings.json with apiKeyHelper", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "claude",
			AuthMode: string(provider.AuthModeAPIKey),
			BasePath: tmpDir,
		}

		p := New()
		p.PrepareProfile(context.Background(), prof)

		settingsPath := filepath.Join(prof.HomePath(), ".claude", "settings.json")
		data, err := os.ReadFile(settingsPath)
		if err != nil {
			t.Fatalf("settings.json not created: %v", err)
		}

		if !strings.Contains(string(data), "apiKeyHelper") {
			t.Error("settings.json should contain apiKeyHelper")
		}
	})

	t.Run("does not create settings.json for OAuth mode", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "claude",
			AuthMode: string(provider.AuthModeOAuth),
			BasePath: tmpDir,
		}

		p := New()
		p.PrepareProfile(context.Background(), prof)

		settingsPath := filepath.Join(prof.HomePath(), ".claude", "settings.json")
		if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
			t.Error("settings.json should not be created for OAuth mode")
		}
	})
}

// =============================================================================
// Env Tests
// =============================================================================

func TestEnv(t *testing.T) {
	t.Run("sets HOME", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "claude",
			BasePath: tmpDir,
		}

		p := New()
		env, err := p.Env(context.Background(), prof)
		if err != nil {
			t.Fatalf("Env() error = %v", err)
		}

		home, ok := env["HOME"]
		if !ok {
			t.Fatal("HOME not set in env")
		}

		expected := prof.HomePath()
		if home != expected {
			t.Errorf("HOME = %q, want %q", home, expected)
		}
	})

	t.Run("sets XDG_CONFIG_HOME", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "claude",
			BasePath: tmpDir,
		}

		p := New()
		env, _ := p.Env(context.Background(), prof)

		xdg, ok := env["XDG_CONFIG_HOME"]
		if !ok {
			t.Fatal("XDG_CONFIG_HOME not set in env")
		}

		expected := prof.XDGConfigPath()
		if xdg != expected {
			t.Errorf("XDG_CONFIG_HOME = %q, want %q", xdg, expected)
		}
	})

	t.Run("returns exactly two env vars", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "claude",
			BasePath: tmpDir,
		}

		p := New()
		env, _ := p.Env(context.Background(), prof)

		if len(env) != 2 {
			t.Errorf("Env() returned %d vars, want 2", len(env))
		}
	})
}

// =============================================================================
// Logout Tests
// =============================================================================

func TestLogout(t *testing.T) {
	t.Run("removes auth.json", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "claude",
			BasePath: tmpDir,
		}

		p := New()
		p.PrepareProfile(context.Background(), prof)

		// Create auth.json
		authDir := filepath.Join(prof.XDGConfigPath(), "claude-code")
		os.MkdirAll(authDir, 0700)
		authPath := filepath.Join(authDir, "auth.json")
		if err := os.WriteFile(authPath, []byte(`{"token":"test"}`), 0600); err != nil {
			t.Fatal(err)
		}

		// Logout
		if err := p.Logout(context.Background(), prof); err != nil {
			t.Fatalf("Logout() error = %v", err)
		}

		// Verify removed
		if _, err := os.Stat(authPath); !os.IsNotExist(err) {
			t.Error("auth.json should be removed after Logout")
		}
	})

	t.Run("removes .claude.json", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "claude",
			BasePath: tmpDir,
		}

		p := New()
		p.PrepareProfile(context.Background(), prof)

		// Create .claude.json
		claudeJsonPath := filepath.Join(prof.HomePath(), ".claude.json")
		if err := os.WriteFile(claudeJsonPath, []byte(`{"session":"test"}`), 0600); err != nil {
			t.Fatal(err)
		}

		// Logout
		if err := p.Logout(context.Background(), prof); err != nil {
			t.Fatalf("Logout() error = %v", err)
		}

		// Verify removed
		if _, err := os.Stat(claudeJsonPath); !os.IsNotExist(err) {
			t.Error(".claude.json should be removed after Logout")
		}
	})

	t.Run("handles non-existent auth files", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "claude",
			BasePath: tmpDir,
		}

		p := New()
		p.PrepareProfile(context.Background(), prof)

		// Don't create auth files, just logout
		if err := p.Logout(context.Background(), prof); err != nil {
			t.Errorf("Logout() error = %v, should handle missing files", err)
		}
	})
}

// =============================================================================
// Status Tests
// =============================================================================

func TestStatus(t *testing.T) {
	t.Run("logged in when auth.json exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "claude",
			BasePath: tmpDir,
		}

		p := New()
		p.PrepareProfile(context.Background(), prof)

		// Create auth.json
		authDir := filepath.Join(prof.XDGConfigPath(), "claude-code")
		os.MkdirAll(authDir, 0700)
		authPath := filepath.Join(authDir, "auth.json")
		if err := os.WriteFile(authPath, []byte(`{}`), 0600); err != nil {
			t.Fatal(err)
		}

		status, err := p.Status(context.Background(), prof)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if !status.LoggedIn {
			t.Error("LoggedIn should be true when auth.json exists")
		}
	})

	t.Run("logged in when .claude.json exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "claude",
			BasePath: tmpDir,
		}

		p := New()
		p.PrepareProfile(context.Background(), prof)

		// Create .claude.json
		claudeJsonPath := filepath.Join(prof.HomePath(), ".claude.json")
		if err := os.WriteFile(claudeJsonPath, []byte(`{}`), 0600); err != nil {
			t.Fatal(err)
		}

		status, err := p.Status(context.Background(), prof)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if !status.LoggedIn {
			t.Error("LoggedIn should be true when .claude.json exists")
		}
	})

	t.Run("not logged in when no auth files exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "claude",
			BasePath: tmpDir,
		}

		p := New()
		p.PrepareProfile(context.Background(), prof)

		status, err := p.Status(context.Background(), prof)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if status.LoggedIn {
			t.Error("LoggedIn should be false when no auth files exist")
		}
	})

	t.Run("reports lock file status", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "claude",
			BasePath: tmpDir,
		}

		p := New()
		p.PrepareProfile(context.Background(), prof)

		// Initially not locked
		status, _ := p.Status(context.Background(), prof)
		if status.HasLockFile {
			t.Error("HasLockFile should be false initially")
		}

		// Lock the profile
		prof.Lock()
		defer prof.Unlock()

		status, _ = p.Status(context.Background(), prof)
		if !status.HasLockFile {
			t.Error("HasLockFile should be true when locked")
		}
	})
}

// =============================================================================
// ValidateProfile Tests
// =============================================================================

func TestValidateProfile(t *testing.T) {
	t.Run("valid when all directories exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "claude",
			BasePath: tmpDir,
		}

		p := New()
		p.PrepareProfile(context.Background(), prof)

		if err := p.ValidateProfile(context.Background(), prof); err != nil {
			t.Errorf("ValidateProfile() error = %v", err)
		}
	})

	t.Run("invalid when home missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "claude",
			BasePath: tmpDir,
		}

		p := New()
		// Don't call PrepareProfile

		err := p.ValidateProfile(context.Background(), prof)
		if err == nil {
			t.Error("ValidateProfile() should error when home missing")
		}
	})

	t.Run("invalid when xdg_config missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "claude",
			BasePath: tmpDir,
		}

		// Create home but not xdg_config
		os.MkdirAll(prof.HomePath(), 0700)

		p := New()
		err := p.ValidateProfile(context.Background(), prof)
		if err == nil {
			t.Error("ValidateProfile() should error when xdg_config missing")
		}
	})
}

// =============================================================================
// Interface Compliance Tests
// =============================================================================

func TestProviderInterface(t *testing.T) {
	// Ensure Provider implements provider.Provider
	var _ provider.Provider = (*Provider)(nil)

	p := New()
	var iface provider.Provider = p

	// Test all interface methods exist
	_ = iface.ID()
	_ = iface.DisplayName()
	_ = iface.DefaultBin()
	_ = iface.SupportedAuthModes()
	_ = iface.AuthFiles()
}

// =============================================================================
// xdgConfigHome Helper Tests
// =============================================================================

func TestXDGConfigHome(t *testing.T) {
	t.Run("respects XDG_CONFIG_HOME env var", func(t *testing.T) {
		original := os.Getenv("XDG_CONFIG_HOME")
		defer os.Setenv("XDG_CONFIG_HOME", original)

		os.Setenv("XDG_CONFIG_HOME", "/test/xdg")
		result := xdgConfigHome()
		if result != "/test/xdg" {
			t.Errorf("xdgConfigHome() = %q, want /test/xdg", result)
		}
	})

	t.Run("falls back to ~/.config", func(t *testing.T) {
		original := os.Getenv("XDG_CONFIG_HOME")
		defer os.Setenv("XDG_CONFIG_HOME", original)

		os.Unsetenv("XDG_CONFIG_HOME")
		result := xdgConfigHome()
		homeDir, _ := os.UserHomeDir()
		expected := filepath.Join(homeDir, ".config")
		if result != expected {
			t.Errorf("xdgConfigHome() = %q, want %s", result, expected)
		}
	})
}

// =============================================================================
// Integration Test
// =============================================================================

func TestFullProfileLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	prof := &profile.Profile{
		Name:     "lifecycle-test",
		Provider: "claude",
		AuthMode: "oauth",
		BasePath: tmpDir,
	}

	p := New()

	// Prepare
	if err := p.PrepareProfile(context.Background(), prof); err != nil {
		t.Fatalf("PrepareProfile() error = %v", err)
	}

	// Validate (should pass now)
	if err := p.ValidateProfile(context.Background(), prof); err != nil {
		t.Fatalf("ValidateProfile() error = %v", err)
	}

	// Status (not logged in yet)
	status, _ := p.Status(context.Background(), prof)
	if status.LoggedIn {
		t.Error("should not be logged in before login")
	}

	// Simulate login by creating .claude.json
	claudeJsonPath := filepath.Join(prof.HomePath(), ".claude.json")
	os.WriteFile(claudeJsonPath, []byte(`{"session":"test"}`), 0600)

	// Status (now logged in)
	status, _ = p.Status(context.Background(), prof)
	if !status.LoggedIn {
		t.Error("should be logged in after .claude.json created")
	}

	// Get env
	env, _ := p.Env(context.Background(), prof)
	if env["HOME"] == "" {
		t.Error("HOME should be set")
	}
	if env["XDG_CONFIG_HOME"] == "" {
		t.Error("XDG_CONFIG_HOME should be set")
	}

	// Logout
	if err := p.Logout(context.Background(), prof); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}

	// Status (logged out)
	status, _ = p.Status(context.Background(), prof)
	if status.LoggedIn {
		t.Error("should not be logged in after logout")
	}
}

// =============================================================================
// API Key Mode Integration Test
// =============================================================================

func TestAPIKeyModeLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	prof := &profile.Profile{
		Name:     "apikey-test",
		Provider: "claude",
		AuthMode: string(provider.AuthModeAPIKey),
		BasePath: tmpDir,
	}

	p := New()

	// Prepare (should create API key helper)
	if err := p.PrepareProfile(context.Background(), prof); err != nil {
		t.Fatalf("PrepareProfile() error = %v", err)
	}

	// Verify helper script exists and is executable
	helperPath := filepath.Join(tmpDir, "api_key_helper.sh")
	info, err := os.Stat(helperPath)
	if err != nil {
		t.Fatalf("api_key_helper.sh not found: %v", err)
	}
	if info.Mode().Perm()&0100 == 0 {
		t.Error("api_key_helper.sh should be executable")
	}

	// Verify settings.json exists
	settingsPath := filepath.Join(prof.HomePath(), ".claude", "settings.json")
	settingsData, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings.json not found: %v", err)
	}
	if !strings.Contains(string(settingsData), helperPath) {
		t.Error("settings.json should reference the helper script path")
	}

	// Status should report logged in for API key mode
	status, err := p.Status(context.Background(), prof)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if !status.LoggedIn {
		t.Error("Status() should report logged in when apiKeyHelper is configured")
	}

	// Passive validation should pass for API key mode
	result, err := p.ValidateToken(context.Background(), prof, true)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	if !result.Valid {
		t.Errorf("ValidateToken() should be valid, got error: %s", result.Error)
	}

	// Validate should pass
	if err := p.ValidateProfile(context.Background(), prof); err != nil {
		t.Errorf("ValidateProfile() error = %v", err)
	}
}
