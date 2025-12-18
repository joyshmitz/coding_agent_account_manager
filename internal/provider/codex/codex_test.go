package codex

import (
	"context"
	"os"
	"path/filepath"
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

func TestReadAPIKeyFromStdin_NonTTY(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "key.txt")
	if err := os.WriteFile(keyPath, []byte("sk-test-123\n"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	f, err := os.Open(keyPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()

	key, hidden, err := readAPIKeyFromStdin(f)
	if err != nil {
		t.Fatalf("readAPIKeyFromStdin: %v", err)
	}
	if hidden {
		t.Fatal("expected non-tty read to not be hidden")
	}
	if key != "sk-test-123" {
		t.Fatalf("key=%q, want %q", key, "sk-test-123")
	}
}

// =============================================================================
// Provider Identity Tests
// =============================================================================

func TestProviderID(t *testing.T) {
	p := New()
	if p.ID() != "codex" {
		t.Errorf("ID() = %q, want %q", p.ID(), "codex")
	}
}

func TestProviderDisplayName(t *testing.T) {
	p := New()
	expected := "Codex CLI (OpenAI GPT Pro)"
	if p.DisplayName() != expected {
		t.Errorf("DisplayName() = %q, want %q", p.DisplayName(), expected)
	}
}

func TestProviderDefaultBin(t *testing.T) {
	p := New()
	if p.DefaultBin() != "codex" {
		t.Errorf("DefaultBin() = %q, want %q", p.DefaultBin(), "codex")
	}
}

// =============================================================================
// Auth Mode Tests
// =============================================================================

func TestSupportedAuthModes(t *testing.T) {
	p := New()
	modes := p.SupportedAuthModes()

	if len(modes) != 3 {
		t.Fatalf("SupportedAuthModes() returned %d modes, want 3", len(modes))
	}

	hasOAuth := false
	hasDeviceCode := false
	hasAPIKey := false
	for _, mode := range modes {
		if mode == provider.AuthModeOAuth {
			hasOAuth = true
		}
		if mode == provider.AuthModeDeviceCode {
			hasDeviceCode = true
		}
		if mode == provider.AuthModeAPIKey {
			hasAPIKey = true
		}
	}

	if !hasOAuth {
		t.Error("SupportedAuthModes() should include OAuth")
	}
	if !hasDeviceCode {
		t.Error("SupportedAuthModes() should include DeviceCode")
	}
	if !hasAPIKey {
		t.Error("SupportedAuthModes() should include APIKey")
	}
}

// =============================================================================
// Auth Files Tests
// =============================================================================

func TestAuthFiles(t *testing.T) {
	t.Run("returns auth.json spec", func(t *testing.T) {
		p := New()
		files := p.AuthFiles()

		if len(files) != 1 {
			t.Fatalf("AuthFiles() returned %d files, want 1", len(files))
		}

		file := files[0]
		if !filepath.IsAbs(file.Path) {
			// May not be absolute if HOME is set oddly, just check it ends with auth.json
			if filepath.Base(file.Path) != "auth.json" {
				t.Errorf("AuthFiles()[0].Path = %q, should end with auth.json", file.Path)
			}
		}
		if !file.Required {
			t.Error("auth.json should be required")
		}
	})

	t.Run("uses CODEX_HOME if set", func(t *testing.T) {
		originalHome := os.Getenv("CODEX_HOME")
		defer os.Setenv("CODEX_HOME", originalHome)

		os.Setenv("CODEX_HOME", "/custom/codex/home")
		p := New()
		files := p.AuthFiles()

		if len(files) != 1 {
			t.Fatal("expected 1 auth file")
		}
		expected := "/custom/codex/home/auth.json"
		if files[0].Path != expected {
			t.Errorf("AuthFiles()[0].Path = %q, want %q", files[0].Path, expected)
		}
	})

	t.Run("uses default .codex if CODEX_HOME not set", func(t *testing.T) {
		originalHome := os.Getenv("CODEX_HOME")
		defer os.Setenv("CODEX_HOME", originalHome)

		os.Unsetenv("CODEX_HOME")
		p := New()
		files := p.AuthFiles()

		if len(files) != 1 {
			t.Fatal("expected 1 auth file")
		}
		homeDir, _ := os.UserHomeDir()
		expected := filepath.Join(homeDir, ".codex", "auth.json")
		if files[0].Path != expected {
			t.Errorf("AuthFiles()[0].Path = %q, want %q", files[0].Path, expected)
		}
	})
}

// =============================================================================
// PrepareProfile Tests
// =============================================================================

func TestPrepareProfile(t *testing.T) {
	t.Run("creates directories", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "codex",
			BasePath: tmpDir,
		}

		p := New()
		if err := p.PrepareProfile(context.Background(), prof); err != nil {
			t.Fatalf("PrepareProfile() error = %v", err)
		}

		// Check codex_home
		codexHomePath := prof.CodexHomePath()
		info, err := os.Stat(codexHomePath)
		if err != nil {
			t.Fatalf("codex_home not created: %v", err)
		}
		if !info.IsDir() {
			t.Error("codex_home should be a directory")
		}

		// Check home (pseudo-home for passthroughs)
		homePath := prof.HomePath()
		info, err = os.Stat(homePath)
		if err != nil {
			t.Fatalf("home not created: %v", err)
		}
		if !info.IsDir() {
			t.Error("home should be a directory")
		}
	})

	t.Run("sets secure permissions", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "codex",
			BasePath: tmpDir,
		}

		p := New()
		p.PrepareProfile(context.Background(), prof)

		codexHomePath := prof.CodexHomePath()
		info, err := os.Stat(codexHomePath)
		if err != nil {
			t.Fatal(err)
		}

		// Should be 0700 (user only)
		if info.Mode().Perm() != 0700 {
			t.Errorf("codex_home permissions = %o, want 0700", info.Mode().Perm())
		}
	})

	t.Run("idempotent - can be called multiple times", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "codex",
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
// Env Tests
// =============================================================================

func TestEnv(t *testing.T) {
	t.Run("sets env vars", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "codex",
			BasePath: tmpDir,
		}

		p := New()
		env, err := p.Env(context.Background(), prof)
		if err != nil {
			t.Fatalf("Env() error = %v", err)
		}

		if len(env) != 2 {
			t.Errorf("Env() returned %d vars, want 2", len(env))
		}

		// Check CODEX_HOME
		codexHome, ok := env["CODEX_HOME"]
		if !ok {
			t.Error("CODEX_HOME not set in env")
		}
		expectedCodexHome := prof.CodexHomePath()
		if codexHome != expectedCodexHome {
			t.Errorf("CODEX_HOME = %q, want %q", codexHome, expectedCodexHome)
		}

		// Check HOME
		home, ok := env["HOME"]
		if !ok {
			t.Error("HOME not set in env")
		}
		expectedHome := prof.HomePath()
		if home != expectedHome {
			t.Errorf("HOME = %q, want %q", home, expectedHome)
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
			Provider: "codex",
			BasePath: tmpDir,
		}

		p := New()
		p.PrepareProfile(context.Background(), prof)

		// Create auth.json
		authPath := filepath.Join(prof.CodexHomePath(), "auth.json")
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

	t.Run("handles non-existent auth.json", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "codex",
			BasePath: tmpDir,
		}

		p := New()
		p.PrepareProfile(context.Background(), prof)

		// Don't create auth.json, just logout
		if err := p.Logout(context.Background(), prof); err != nil {
			t.Errorf("Logout() error = %v, should handle missing file", err)
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
			Provider: "codex",
			BasePath: tmpDir,
		}

		p := New()
		p.PrepareProfile(context.Background(), prof)

		// Create auth.json
		authPath := filepath.Join(prof.CodexHomePath(), "auth.json")
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

	t.Run("not logged in when auth.json missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "codex",
			BasePath: tmpDir,
		}

		p := New()
		p.PrepareProfile(context.Background(), prof)

		status, err := p.Status(context.Background(), prof)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if status.LoggedIn {
			t.Error("LoggedIn should be false when auth.json missing")
		}
	})

	t.Run("reports lock file status", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "codex",
			BasePath: tmpDir,
		}

		p := New()

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
	t.Run("valid when codex_home exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "codex",
			BasePath: tmpDir,
		}

		p := New()
		p.PrepareProfile(context.Background(), prof)

		if err := p.ValidateProfile(context.Background(), prof); err != nil {
			t.Errorf("ValidateProfile() error = %v", err)
		}
	})

	t.Run("invalid when codex_home missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		prof := &profile.Profile{
			Name:     "test",
			Provider: "codex",
			BasePath: tmpDir,
		}

		p := New()
		// Don't call PrepareProfile

		err := p.ValidateProfile(context.Background(), prof)
		if err == nil {
			t.Error("ValidateProfile() should error when codex_home missing")
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
// codexHome Helper Tests (via AuthFiles)
// =============================================================================

func TestCodexHomeHelper(t *testing.T) {
	// Test CODEX_HOME environment variable override
	originalHome := os.Getenv("CODEX_HOME")
	defer os.Setenv("CODEX_HOME", originalHome)

	t.Run("respects CODEX_HOME env var", func(t *testing.T) {
		os.Setenv("CODEX_HOME", "/test/codex")
		p := New()
		files := p.AuthFiles()
		if !hasPrefix(files[0].Path, "/test/codex") {
			t.Errorf("Path %q should use CODEX_HOME=/test/codex", files[0].Path)
		}
	})

	t.Run("falls back to ~/.codex", func(t *testing.T) {
		os.Unsetenv("CODEX_HOME")
		p := New()
		files := p.AuthFiles()
		homeDir, _ := os.UserHomeDir()
		expected := filepath.Join(homeDir, ".codex")
		if !hasPrefix(files[0].Path, expected) {
			t.Errorf("Path %q should use %s", files[0].Path, expected)
		}
	})
}

// hasPrefix checks if path starts with prefix.
func hasPrefix(path, prefix string) bool {
	return len(path) >= len(prefix) && path[:len(prefix)] == prefix
}

// =============================================================================
// Integration Test
// =============================================================================

func TestFullProfileLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	prof := &profile.Profile{
		Name:     "lifecycle-test",
		Provider: "codex",
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

	// Simulate login by creating auth.json
	authPath := filepath.Join(prof.CodexHomePath(), "auth.json")
	os.WriteFile(authPath, []byte(`{"token":"test"}`), 0600)

	// Status (now logged in)
	status, _ = p.Status(context.Background(), prof)
	if !status.LoggedIn {
		t.Error("should be logged in after auth.json created")
	}

	// Get env
	env, _ := p.Env(context.Background(), prof)
	if env["CODEX_HOME"] == "" {
		t.Error("CODEX_HOME should be set")
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
