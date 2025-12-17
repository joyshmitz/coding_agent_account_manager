// Package profile contains tests for profile management.
package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// =============================================================================
// Profile Methods Tests
// =============================================================================

func TestProfilePaths(t *testing.T) {
	prof := &Profile{
		Name:     "test-profile",
		Provider: "claude",
		BasePath: "/tmp/test-profiles/claude/test-profile",
	}

	tests := []struct {
		name     string
		method   func() string
		expected string
	}{
		{"HomePath", prof.HomePath, "/tmp/test-profiles/claude/test-profile/home"},
		{"XDGConfigPath", prof.XDGConfigPath, "/tmp/test-profiles/claude/test-profile/xdg_config"},
		{"CodexHomePath", prof.CodexHomePath, "/tmp/test-profiles/claude/test-profile/codex_home"},
		{"LockPath", prof.LockPath, "/tmp/test-profiles/claude/test-profile/.lock"},
		{"MetaPath", prof.MetaPath, "/tmp/test-profiles/claude/test-profile/profile.json"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.method()
			if got != tc.expected {
				t.Errorf("%s() = %q, want %q", tc.name, got, tc.expected)
			}
		})
	}
}

func TestHasBrowserConfig(t *testing.T) {
	tests := []struct {
		name           string
		browserCommand string
		browserProfile string
		expected       bool
	}{
		{"no config", "", "", false},
		{"command only", "chrome", "", true},
		{"profile only", "", "Profile 1", true},
		{"both set", "chrome", "Profile 1", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prof := &Profile{
				BrowserCommand:    tc.browserCommand,
				BrowserProfileDir: tc.browserProfile,
			}
			if got := prof.HasBrowserConfig(); got != tc.expected {
				t.Errorf("HasBrowserConfig() = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestBrowserDisplayName(t *testing.T) {
	tests := []struct {
		name           string
		profileName    string
		browserCmd     string
		browserProfile string
		expected       string
	}{
		{"custom name set", "My Chrome", "chrome", "Profile 1", "My Chrome"},
		{"profile only", "", "chrome", "Profile 1", "chrome (Profile 1)"},
		{"command only", "", "chrome", "", "chrome"},
		{"profile dir only", "", "", "Profile 1", "Profile 1"},
		{"nothing set", "", "", "", "system default"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prof := &Profile{
				BrowserProfileName: tc.profileName,
				BrowserCommand:     tc.browserCmd,
				BrowserProfileDir:  tc.browserProfile,
			}
			if got := prof.BrowserDisplayName(); got != tc.expected {
				t.Errorf("BrowserDisplayName() = %q, want %q", got, tc.expected)
			}
		})
	}
}

// =============================================================================
// Lock Management Tests
// =============================================================================

func TestLockUnlock(t *testing.T) {
	tmpDir := t.TempDir()
	prof := &Profile{
		Name:     "test",
		Provider: "codex",
		BasePath: tmpDir,
	}

	// Initially not locked
	if prof.IsLocked() {
		t.Error("expected profile to be unlocked initially")
	}

	// Lock
	if err := prof.Lock(); err != nil {
		t.Fatalf("Lock() error = %v", err)
	}

	// Now locked
	if !prof.IsLocked() {
		t.Error("expected profile to be locked after Lock()")
	}

	// Lock again should fail
	if err := prof.Lock(); err == nil {
		t.Error("expected Lock() to fail when already locked")
	}

	// Unlock
	if err := prof.Unlock(); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}

	// Now unlocked
	if prof.IsLocked() {
		t.Error("expected profile to be unlocked after Unlock()")
	}

	// Unlock again should be safe (idempotent)
	if err := prof.Unlock(); err != nil {
		t.Errorf("Unlock() on unlocked profile error = %v", err)
	}
}

func TestLockAtomicity(t *testing.T) {
	// Test that Lock() uses atomic file creation (O_EXCL)
	tmpDir := t.TempDir()
	prof := &Profile{
		Name:     "test",
		Provider: "codex",
		BasePath: tmpDir,
	}

	// Create a pre-existing lock file manually
	lockPath := filepath.Join(tmpDir, ".lock")
	if err := os.WriteFile(lockPath, []byte("existing"), 0600); err != nil {
		t.Fatalf("failed to create pre-existing lock: %v", err)
	}

	// Lock should fail because file exists
	if err := prof.Lock(); err == nil {
		t.Error("expected Lock() to fail when lock file already exists")
	}
}

func TestGetLockInfo(t *testing.T) {
	tmpDir := t.TempDir()
	prof := &Profile{
		Name:     "test",
		Provider: "codex",
		BasePath: tmpDir,
	}

	// No lock file - should return nil, nil
	info, err := prof.GetLockInfo()
	if err != nil {
		t.Fatalf("GetLockInfo() error = %v", err)
	}
	if info != nil {
		t.Error("expected nil lock info when no lock exists")
	}

	// Create lock
	if err := prof.Lock(); err != nil {
		t.Fatalf("Lock() error = %v", err)
	}

	// Get lock info
	info, err = prof.GetLockInfo()
	if err != nil {
		t.Fatalf("GetLockInfo() error = %v", err)
	}
	if info == nil {
		t.Fatal("expected non-nil lock info")
	}

	if info.PID != os.Getpid() {
		t.Errorf("LockInfo.PID = %d, want %d", info.PID, os.Getpid())
	}
	if info.LockedAt.IsZero() {
		t.Error("expected non-zero LockedAt")
	}

	// Clean up
	prof.Unlock()
}

func TestIsLockStale(t *testing.T) {
	tmpDir := t.TempDir()
	prof := &Profile{
		Name:     "test",
		Provider: "codex",
		BasePath: tmpDir,
	}

	// No lock - not stale (no lock to be stale)
	stale, err := prof.IsLockStale()
	if err != nil {
		t.Fatalf("IsLockStale() error = %v", err)
	}
	if stale {
		t.Error("expected not stale when no lock exists")
	}

	// Create lock with current PID (not stale)
	if err := prof.Lock(); err != nil {
		t.Fatalf("Lock() error = %v", err)
	}

	stale, err = prof.IsLockStale()
	if err != nil {
		t.Fatalf("IsLockStale() error = %v", err)
	}
	if stale {
		t.Error("expected not stale when lock is from current process")
	}

	prof.Unlock()

	// Create lock with fake dead PID
	lockPath := filepath.Join(tmpDir, ".lock")
	fakePID := 99999999 // Unlikely to be a real process
	content := `{"pid": 99999999, "locked_at": "2025-01-01T00:00:00Z"}`
	if err := os.WriteFile(lockPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write fake lock: %v", err)
	}

	stale, err = prof.IsLockStale()
	if err != nil {
		t.Fatalf("IsLockStale() error = %v (pid=%d)", err, fakePID)
	}
	if !stale {
		t.Error("expected stale when lock is from dead process")
	}
}

func TestCleanStaleLock(t *testing.T) {
	tmpDir := t.TempDir()
	prof := &Profile{
		Name:     "test",
		Provider: "codex",
		BasePath: tmpDir,
	}

	// No lock - should return false, nil
	cleaned, err := prof.CleanStaleLock()
	if err != nil {
		t.Fatalf("CleanStaleLock() error = %v", err)
	}
	if cleaned {
		t.Error("expected cleaned=false when no lock exists")
	}

	// Create stale lock
	lockPath := filepath.Join(tmpDir, ".lock")
	content := `{"pid": 99999999, "locked_at": "2025-01-01T00:00:00Z"}`
	if err := os.WriteFile(lockPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write stale lock: %v", err)
	}

	cleaned, err = prof.CleanStaleLock()
	if err != nil {
		t.Fatalf("CleanStaleLock() error = %v", err)
	}
	if !cleaned {
		t.Error("expected cleaned=true for stale lock")
	}

	// Lock should be gone
	if prof.IsLocked() {
		t.Error("expected lock to be removed after CleanStaleLock")
	}
}

func TestLockWithCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	prof := &Profile{
		Name:     "test",
		Provider: "codex",
		BasePath: tmpDir,
	}

	// Create stale lock first
	lockPath := filepath.Join(tmpDir, ".lock")
	content := `{"pid": 99999999, "locked_at": "2025-01-01T00:00:00Z"}`
	if err := os.WriteFile(lockPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write stale lock: %v", err)
	}

	// LockWithCleanup should clean stale lock and acquire new lock
	if err := prof.LockWithCleanup(); err != nil {
		t.Fatalf("LockWithCleanup() error = %v", err)
	}

	// Should now be locked with current PID
	info, err := prof.GetLockInfo()
	if err != nil {
		t.Fatalf("GetLockInfo() error = %v", err)
	}
	if info.PID != os.Getpid() {
		t.Errorf("LockInfo.PID = %d, want %d", info.PID, os.Getpid())
	}

	prof.Unlock()
}

func TestIsProcessAlive(t *testing.T) {
	// Current process should be alive
	if !IsProcessAlive(os.Getpid()) {
		t.Error("expected current process to be alive")
	}

	// PID 0 is never a user process
	if IsProcessAlive(0) {
		t.Error("expected PID 0 to not be alive")
	}

	// Negative PID should not be alive
	if IsProcessAlive(-1) {
		t.Error("expected negative PID to not be alive")
	}

	// Very high PID unlikely to exist
	if IsProcessAlive(99999999) {
		t.Error("expected very high PID to not be alive")
	}
}

// =============================================================================
// Profile Save/Load Tests
// =============================================================================

func TestProfileSave(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "claude", "test-profile")

	prof := &Profile{
		Name:         "test-profile",
		Provider:     "claude",
		AuthMode:     "oauth",
		BasePath:     basePath,
		AccountLabel: "test@example.com",
		CreatedAt:    time.Now(),
		Metadata:     map[string]string{"key": "value"},
	}

	// Save
	if err := prof.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	metaPath := filepath.Join(basePath, "profile.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("failed to read saved profile: %v", err)
	}

	// Parse and verify
	var loaded Profile
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("failed to parse saved profile: %v", err)
	}

	if loaded.Name != prof.Name {
		t.Errorf("loaded.Name = %q, want %q", loaded.Name, prof.Name)
	}
	if loaded.Provider != prof.Provider {
		t.Errorf("loaded.Provider = %q, want %q", loaded.Provider, prof.Provider)
	}
	if loaded.AccountLabel != prof.AccountLabel {
		t.Errorf("loaded.AccountLabel = %q, want %q", loaded.AccountLabel, prof.AccountLabel)
	}
}

func TestUpdateLastUsed(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "codex", "test")

	prof := &Profile{
		Name:     "test",
		Provider: "codex",
		BasePath: basePath,
	}

	// Initially zero
	if !prof.LastUsedAt.IsZero() {
		t.Error("expected LastUsedAt to be zero initially")
	}

	before := time.Now()
	if err := prof.UpdateLastUsed(); err != nil {
		t.Fatalf("UpdateLastUsed() error = %v", err)
	}
	after := time.Now()

	if prof.LastUsedAt.Before(before) || prof.LastUsedAt.After(after) {
		t.Errorf("LastUsedAt = %v, expected between %v and %v", prof.LastUsedAt, before, after)
	}
}

// =============================================================================
// Store Tests
// =============================================================================

func TestNewStore(t *testing.T) {
	store := NewStore("/tmp/test-profiles")
	if store == nil {
		t.Fatal("NewStore returned nil")
	}
	if store.basePath != "/tmp/test-profiles" {
		t.Errorf("basePath = %q, want %q", store.basePath, "/tmp/test-profiles")
	}
}

func TestStoreProfilePath(t *testing.T) {
	store := NewStore("/tmp/profiles")
	path := store.ProfilePath("claude", "work")
	expected := "/tmp/profiles/claude/work"
	if path != expected {
		t.Errorf("ProfilePath() = %q, want %q", path, expected)
	}
}

func TestStoreCreate(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	// Create profile
	prof, err := store.Create("codex", "test-profile", "oauth")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if prof.Name != "test-profile" {
		t.Errorf("Name = %q, want %q", prof.Name, "test-profile")
	}
	if prof.Provider != "codex" {
		t.Errorf("Provider = %q, want %q", prof.Provider, "codex")
	}
	if prof.AuthMode != "oauth" {
		t.Errorf("AuthMode = %q, want %q", prof.AuthMode, "oauth")
	}
	if prof.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}

	// Verify file exists
	metaPath := filepath.Join(tmpDir, "codex", "test-profile", "profile.json")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Error("expected profile.json to exist")
	}

	// Create duplicate should fail
	_, err = store.Create("codex", "test-profile", "oauth")
	if err == nil {
		t.Error("expected Create() to fail for duplicate profile")
	}
}

func TestStoreLoad(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	// Create first
	created, err := store.Create("claude", "work", "oauth")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	created.AccountLabel = "work@company.com"
	created.Save()

	// Load
	loaded, err := store.Load("claude", "work")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.Name != "work" {
		t.Errorf("Name = %q, want %q", loaded.Name, "work")
	}
	if loaded.Provider != "claude" {
		t.Errorf("Provider = %q, want %q", loaded.Provider, "claude")
	}
	if loaded.AccountLabel != "work@company.com" {
		t.Errorf("AccountLabel = %q, want %q", loaded.AccountLabel, "work@company.com")
	}

	// Load non-existent should fail
	_, err = store.Load("claude", "nonexistent")
	if err == nil {
		t.Error("expected Load() to fail for non-existent profile")
	}
}

func TestStoreDelete(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	// Create
	if _, err := store.Create("gemini", "personal", "oauth"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Delete
	if err := store.Delete("gemini", "personal"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Should no longer exist
	if store.Exists("gemini", "personal") {
		t.Error("expected profile to not exist after Delete")
	}

	// Delete non-existent should fail
	if err := store.Delete("gemini", "personal"); err == nil {
		t.Error("expected Delete() to fail for non-existent profile")
	}
}

func TestStoreDeleteLockedFails(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	// Create and lock
	prof, err := store.Create("codex", "locked-profile", "oauth")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := prof.Lock(); err != nil {
		t.Fatalf("Lock() error = %v", err)
	}
	defer prof.Unlock()

	// Delete should fail
	if err := store.Delete("codex", "locked-profile"); err == nil {
		t.Error("expected Delete() to fail for locked profile")
	}
}

func TestStoreList(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	// Create some profiles
	store.Create("claude", "work", "oauth")
	store.Create("claude", "personal", "oauth")
	store.Create("codex", "main", "api-key")

	// List claude profiles
	profiles, err := store.List("claude")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(profiles) != 2 {
		t.Errorf("len(profiles) = %d, want 2", len(profiles))
	}

	// List codex profiles
	profiles, err = store.List("codex")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(profiles) != 1 {
		t.Errorf("len(profiles) = %d, want 1", len(profiles))
	}

	// List non-existent provider
	profiles, err = store.List("gemini")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(profiles) != 0 {
		t.Errorf("len(profiles) = %d, want 0", len(profiles))
	}
}

func TestStoreListAll(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	// Create profiles for multiple providers
	store.Create("claude", "a", "oauth")
	store.Create("claude", "b", "oauth")
	store.Create("codex", "c", "api-key")

	allProfiles, err := store.ListAll()
	if err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}

	if len(allProfiles) != 2 {
		t.Errorf("len(allProfiles) = %d, want 2", len(allProfiles))
	}
	if len(allProfiles["claude"]) != 2 {
		t.Errorf("len(allProfiles[claude]) = %d, want 2", len(allProfiles["claude"]))
	}
	if len(allProfiles["codex"]) != 1 {
		t.Errorf("len(allProfiles[codex]) = %d, want 1", len(allProfiles["codex"]))
	}
}

func TestStoreExists(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	// Doesn't exist yet
	if store.Exists("claude", "test") {
		t.Error("expected Exists() = false before creation")
	}

	// Create
	store.Create("claude", "test", "oauth")

	// Now exists
	if !store.Exists("claude", "test") {
		t.Error("expected Exists() = true after creation")
	}
}

// =============================================================================
// DefaultStorePath Tests
// =============================================================================

func TestDefaultStorePath(t *testing.T) {
	// Test with XDG_DATA_HOME set
	originalXDG := os.Getenv("XDG_DATA_HOME")
	defer os.Setenv("XDG_DATA_HOME", originalXDG)

	os.Setenv("XDG_DATA_HOME", "/custom/data")
	path := DefaultStorePath()
	expected := "/custom/data/caam/profiles"
	if path != expected {
		t.Errorf("DefaultStorePath() with XDG = %q, want %q", path, expected)
	}

	// Test without XDG_DATA_HOME (uses home dir)
	os.Unsetenv("XDG_DATA_HOME")
	path = DefaultStorePath()
	// Should contain ".local/share/caam/profiles" relative to home
	if !filepath.IsAbs(path) {
		// If path is relative, it means UserHomeDir failed - that's the fallback
		expected := ".local/share/caam/profiles"
		if path != expected {
			t.Errorf("DefaultStorePath() fallback = %q, want %q", path, expected)
		}
	}
}
