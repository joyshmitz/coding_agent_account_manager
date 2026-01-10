package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/authfile"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/health"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.CheckInterval != DefaultCheckInterval {
		t.Errorf("expected CheckInterval %v, got %v", DefaultCheckInterval, cfg.CheckInterval)
	}

	if cfg.RefreshThreshold != DefaultRefreshThreshold {
		t.Errorf("expected RefreshThreshold %v, got %v", DefaultRefreshThreshold, cfg.RefreshThreshold)
	}

	if cfg.Verbose {
		t.Error("expected Verbose to be false by default")
	}
}

func TestNewDaemon(t *testing.T) {
	tmpDir := t.TempDir()
	v := authfile.NewVault(tmpDir)
	hs := health.NewStorage(filepath.Join(tmpDir, "health.json"))

	d := New(v, hs, nil)

	if d == nil {
		t.Fatal("expected daemon to be created")
	}

	if d.config == nil {
		t.Error("expected config to be set")
	}

	if d.config.CheckInterval != DefaultCheckInterval {
		t.Errorf("expected default CheckInterval, got %v", d.config.CheckInterval)
	}
}

func TestNewDaemonWithConfig(t *testing.T) {
	tmpDir := t.TempDir()
	v := authfile.NewVault(tmpDir)
	hs := health.NewStorage(filepath.Join(tmpDir, "health.json"))

	cfg := &Config{
		CheckInterval:    1 * time.Minute,
		RefreshThreshold: 15 * time.Minute,
		Verbose:          true,
	}

	d := New(v, hs, cfg)

	if d.config.CheckInterval != 1*time.Minute {
		t.Errorf("expected CheckInterval 1m, got %v", d.config.CheckInterval)
	}

	if d.config.RefreshThreshold != 15*time.Minute {
		t.Errorf("expected RefreshThreshold 15m, got %v", d.config.RefreshThreshold)
	}

	if !d.config.Verbose {
		t.Error("expected Verbose to be true")
	}
}

func TestDaemonIsRunning(t *testing.T) {
	tmpDir := t.TempDir()
	v := authfile.NewVault(tmpDir)
	hs := health.NewStorage(filepath.Join(tmpDir, "health.json"))

	d := New(v, hs, nil)

	if d.IsRunning() {
		t.Error("daemon should not be running initially")
	}
}

func TestDaemonGetStats(t *testing.T) {
	tmpDir := t.TempDir()
	v := authfile.NewVault(tmpDir)
	hs := health.NewStorage(filepath.Join(tmpDir, "health.json"))

	d := New(v, hs, nil)

	stats := d.GetStats()

	if !stats.StartTime.IsZero() {
		t.Error("StartTime should be zero before daemon starts")
	}

	if stats.CheckCount != 0 {
		t.Errorf("expected CheckCount 0, got %d", stats.CheckCount)
	}
}

func TestDaemonStopBeforeStart(t *testing.T) {
	tmpDir := t.TempDir()
	v := authfile.NewVault(tmpDir)
	hs := health.NewStorage(filepath.Join(tmpDir, "health.json"))

	d := New(v, hs, nil)

	// Stop before start should not error
	if err := d.Stop(); err != nil {
		t.Errorf("Stop before Start should not error: %v", err)
	}
}

func TestPIDFilePath(t *testing.T) {
	path := PIDFilePath()
	if path == "" {
		t.Error("PIDFilePath should not be empty")
	}

	if filepath.Base(path) != "caam-daemon.pid" {
		t.Errorf("expected filename caam-daemon.pid, got %s", filepath.Base(path))
	}
}

func TestLogFilePath(t *testing.T) {
	path := LogFilePath()
	if path == "" {
		t.Error("LogFilePath should not be empty")
	}

	if filepath.Base(path) != "daemon.log" {
		t.Errorf("expected filename daemon.log, got %s", filepath.Base(path))
	}
}

func TestWriteAndReadPIDFile(t *testing.T) {
	pidPath := PIDFilePath()

	// If there's already a running daemon, skip this test
	if existingPID, err := ReadPIDFile(); err == nil && IsProcessRunning(existingPID) {
		t.Skipf("skipping: daemon already running with PID %d", existingPID)
	}

	// Clean up any stale PID file first
	os.Remove(pidPath)

	// Write PID file
	if err := WritePIDFile(); err != nil {
		t.Fatalf("WritePIDFile failed: %v", err)
	}
	defer os.Remove(pidPath)

	// Read PID file
	pid, err := ReadPIDFile()
	if err != nil {
		t.Fatalf("ReadPIDFile failed: %v", err)
	}

	if pid != os.Getpid() {
		t.Errorf("expected PID %d, got %d", os.Getpid(), pid)
	}

	// Remove PID file
	if err := RemovePIDFile(); err != nil {
		t.Errorf("RemovePIDFile failed: %v", err)
	}

	// Verify it's gone
	_, err = ReadPIDFile()
	if err == nil {
		t.Error("expected error reading removed PID file")
	}
}

func TestIsProcessRunning(t *testing.T) {
	// Current process should be running
	if !IsProcessRunning(os.Getpid()) {
		t.Error("current process should be reported as running")
	}

	// Non-existent PID should not be running
	// Use a very high PID that's unlikely to exist
	if IsProcessRunning(999999999) {
		t.Error("non-existent PID should not be reported as running")
	}
}

func TestGetDaemonStatusNotRunning(t *testing.T) {
	// Clean up any existing PID file
	os.Remove(PIDFilePath())

	running, pid, err := GetDaemonStatus()
	if err != nil {
		t.Fatalf("GetDaemonStatus failed: %v", err)
	}

	if running {
		t.Error("daemon should not be reported as running without PID file")
	}

	if pid != 0 {
		t.Errorf("expected PID 0, got %d", pid)
	}
}

func TestIsUnsupportedError(t *testing.T) {
	// Test nil error
	if isUnsupportedError(nil, nil) {
		t.Error("nil error should not be unsupported error")
	}

	// Test regular error
	if isUnsupportedError(os.ErrNotExist, nil) {
		t.Error("ErrNotExist should not be unsupported error")
	}
}

func TestIsUnsupportedError_WithTarget(t *testing.T) {
	// Create an UnsupportedError by importing refresh package
	// We can't easily test with actual UnsupportedError without importing refresh
	// which could cause circular imports. Test the false path more thoroughly.

	// Wrapped error should not match
	wrapped := os.ErrNotExist
	var target *struct{} // Different type
	if isUnsupportedError(wrapped, nil) {
		t.Error("wrapped regular error should not be unsupported error")
	}
	_ = target
}

func TestIsProcessRunning_EdgeCases(t *testing.T) {
	// Zero PID should not be running
	if IsProcessRunning(0) {
		t.Error("PID 0 should not be reported as running")
	}

	// Negative PID should not be running
	if IsProcessRunning(-1) {
		t.Error("negative PID should not be reported as running")
	}

	// PID 1 (init) should be running on Linux (unless in container)
	// We don't test this as it depends on permissions
}

func TestStopDaemonByPID(t *testing.T) {
	// Test with non-existent PID
	err := StopDaemonByPID(999999999)
	if err == nil {
		t.Error("StopDaemonByPID should error for non-existent PID")
	}

	// Test with invalid PID
	err = StopDaemonByPID(-1)
	if err == nil {
		t.Error("StopDaemonByPID should error for invalid PID")
	}
}

func TestGetDaemonStatus_StalePIDFile(t *testing.T) {
	// Create a PID file with a non-existent PID
	pidPath := PIDFilePath()

	// Cleanup before test
	os.Remove(pidPath)
	defer os.Remove(pidPath)

	// Write a stale PID (very high number unlikely to exist)
	err := os.WriteFile(pidPath, []byte("999999999\n"), 0600)
	if err != nil {
		t.Fatalf("failed to write test PID file: %v", err)
	}

	running, pid, err := GetDaemonStatus()
	if err != nil {
		t.Fatalf("GetDaemonStatus failed: %v", err)
	}

	if running {
		t.Error("daemon should not be reported as running for stale PID")
	}

	if pid != 0 {
		t.Errorf("expected PID 0 after cleanup, got %d", pid)
	}

	// Stale PID file should be removed
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("stale PID file should have been removed")
	}
}

func TestReadPIDFile_InvalidFormat(t *testing.T) {
	pidPath := PIDFilePath()

	// Cleanup before test
	os.Remove(pidPath)
	defer os.Remove(pidPath)

	// Write invalid PID format
	err := os.WriteFile(pidPath, []byte("not-a-number\n"), 0600)
	if err != nil {
		t.Fatalf("failed to write test PID file: %v", err)
	}

	_, err = ReadPIDFile()
	if err == nil {
		t.Error("ReadPIDFile should error on invalid format")
	}
}

func TestLogFilePath_NoHomeDir(t *testing.T) {
	// LogFilePath falls back to temp dir if home dir is not available
	// We can't easily test this without modifying environment
	// but we can verify it returns a valid path
	path := LogFilePath()
	if path == "" {
		t.Error("LogFilePath should not return empty string")
	}
	if filepath.Ext(path) != ".log" {
		t.Errorf("LogFilePath should end with .log, got %s", path)
	}
}

func TestNewDaemon_WithLogPath(t *testing.T) {
	tmpDir := t.TempDir()
	v := authfile.NewVault(tmpDir)
	hs := health.NewStorage(filepath.Join(tmpDir, "health.json"))

	logPath := filepath.Join(tmpDir, "test-daemon.log")
	cfg := &Config{
		CheckInterval:    50 * time.Millisecond,
		RefreshThreshold: 1 * time.Minute,
		LogPath:          logPath,
	}

	d := New(v, hs, cfg)
	defer d.Stop()

	if d == nil {
		t.Fatal("expected daemon to be created")
	}

	// Log file should be created
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("log file should be created")
	}
}

func TestNewDaemon_WithInvalidLogPath(t *testing.T) {
	tmpDir := t.TempDir()
	v := authfile.NewVault(tmpDir)
	hs := health.NewStorage(filepath.Join(tmpDir, "health.json"))

	// Use an invalid log path
	cfg := &Config{
		LogPath: "/nonexistent/path/daemon.log",
	}

	// Should not panic, should fall back to stdout
	d := New(v, hs, cfg)
	if d == nil {
		t.Fatal("daemon should be created even with invalid log path")
	}
}

func TestDaemon_ReloadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	v := authfile.NewVault(tmpDir)
	hs := health.NewStorage(filepath.Join(tmpDir, "health.json"))

	cfg := &Config{
		CheckInterval:    50 * time.Millisecond,
		RefreshThreshold: 1 * time.Minute,
		Verbose:          true,
	}

	d := New(v, hs, cfg)

	// ReloadConfig should not panic
	d.ReloadConfig()
}

func TestDaemon_CheckAndRefresh_EmptyVault(t *testing.T) {
	tmpDir := t.TempDir()
	v := authfile.NewVault(tmpDir)
	hs := health.NewStorage(filepath.Join(tmpDir, "health.json"))

	cfg := &Config{
		CheckInterval:    50 * time.Millisecond,
		RefreshThreshold: 1 * time.Minute,
		Verbose:          true,
	}

	d := New(v, hs, cfg)

	// Should not panic with empty vault
	d.checkAndRefresh()

	stats := d.GetStats()
	if stats.CheckCount != 1 {
		t.Errorf("CheckCount should be 1, got %d", stats.CheckCount)
	}
}

func TestDaemon_CheckAndBackup_NoScheduler(t *testing.T) {
	tmpDir := t.TempDir()
	v := authfile.NewVault(tmpDir)
	hs := health.NewStorage(filepath.Join(tmpDir, "health.json"))

	cfg := &Config{
		CheckInterval:    50 * time.Millisecond,
		RefreshThreshold: 1 * time.Minute,
	}

	d := New(v, hs, cfg)

	// Should not panic when scheduler is nil
	d.checkAndBackup()
}

func TestDaemon_GetProfileHealth_EmptyVault(t *testing.T) {
	tmpDir := t.TempDir()
	v := authfile.NewVault(tmpDir)
	hs := health.NewStorage(filepath.Join(tmpDir, "health.json"))

	cfg := &Config{
		CheckInterval: 50 * time.Millisecond,
	}

	d := New(v, hs, cfg)

	// Should return nil for non-existent profile
	ph := d.getProfileHealth("claude", "nonexistent")
	if ph != nil {
		t.Error("getProfileHealth should return nil for non-existent profile")
	}
}

func TestDaemon_GetProfileHealth_FromHealthStore(t *testing.T) {
	tmpDir := t.TempDir()
	v := authfile.NewVault(tmpDir)
	hs := health.NewStorage(filepath.Join(tmpDir, "health.json"))

	// Store health data
	expiry := time.Now().Add(1 * time.Hour)
	ph := &health.ProfileHealth{
		TokenExpiresAt: expiry,
	}
	if err := hs.UpdateProfile("claude", "test", ph); err != nil {
		t.Fatalf("failed to update profile health: %v", err)
	}

	cfg := &Config{
		CheckInterval: 50 * time.Millisecond,
	}

	d := New(v, hs, cfg)

	gotPh := d.getProfileHealth("claude", "test")
	if gotPh == nil {
		t.Fatal("getProfileHealth should return health data from store")
	}

	if gotPh.TokenExpiresAt.IsZero() {
		t.Error("TokenExpiresAt should not be zero")
	}
}

func TestDaemon_CheckProfile_NonExistentProfile(t *testing.T) {
	tmpDir := t.TempDir()
	v := authfile.NewVault(tmpDir)
	hs := health.NewStorage(filepath.Join(tmpDir, "health.json"))

	cfg := &Config{
		CheckInterval:    50 * time.Millisecond,
		RefreshThreshold: 1 * time.Minute,
		Verbose:          true,
	}

	d := New(v, hs, cfg)

	// Should not panic for non-existent profile
	d.checkProfile("claude", "nonexistent")
}

func TestDaemon_CheckProfile_TokenNotExpiring(t *testing.T) {
	tmpDir := t.TempDir()
	v := authfile.NewVault(tmpDir)
	hs := health.NewStorage(filepath.Join(tmpDir, "health.json"))

	// Store health data with token expiring in 2 hours (not due for refresh)
	expiry := time.Now().Add(2 * time.Hour)
	ph := &health.ProfileHealth{
		TokenExpiresAt: expiry,
	}
	if err := hs.UpdateProfile("claude", "test", ph); err != nil {
		t.Fatalf("failed to update profile health: %v", err)
	}

	cfg := &Config{
		CheckInterval:    50 * time.Millisecond,
		RefreshThreshold: 10 * time.Minute, // Only refresh if < 10 min
		Verbose:          true,
	}

	d := New(v, hs, cfg)

	// Should not attempt refresh (TTL > threshold)
	d.checkProfile("claude", "test")

	stats := d.GetStats()
	if stats.RefreshCount != 0 {
		t.Errorf("RefreshCount should be 0 (token not expiring), got %d", stats.RefreshCount)
	}
}

func TestSetPIDFilePath(t *testing.T) {
	// Save original
	original := PIDFilePath()
	defer SetPIDFilePath(original)

	// Set custom path
	SetPIDFilePath("/custom/path.pid")
	if got := PIDFilePath(); got != "/custom/path.pid" {
		t.Errorf("PIDFilePath() = %s, want /custom/path.pid", got)
	}

	// Set back to empty to restore default behavior
	SetPIDFilePath("")
	if got := PIDFilePath(); got == "/custom/path.pid" {
		t.Error("PIDFilePath should not be /custom/path.pid after reset")
	}
}

func TestDaemon_CheckProfile_ExpiringToken(t *testing.T) {
	tmpDir := t.TempDir()
	v := authfile.NewVault(tmpDir)
	hs := health.NewStorage(filepath.Join(tmpDir, "health.json"))

	// Store health data with token expiring soon (below refresh threshold)
	expiry := time.Now().Add(5 * time.Minute)
	ph := &health.ProfileHealth{
		TokenExpiresAt: expiry,
	}
	if err := hs.UpdateProfile("claude", "test", ph); err != nil {
		t.Fatalf("failed to update profile health: %v", err)
	}

	cfg := &Config{
		CheckInterval:    50 * time.Millisecond,
		RefreshThreshold: 10 * time.Minute, // Token is expiring in 5min, threshold is 10min
		Verbose:          true,
	}

	d := New(v, hs, cfg)
	d.ctx, d.cancel = context.WithCancel(context.Background())
	defer d.cancel()

	// This will attempt to refresh, but fail because there's no actual profile
	// The important thing is that it exercises the code path
	d.checkProfile("claude", "test")

	stats := d.GetStats()
	// Should have recorded an error (refresh fails because profile doesn't exist in vault)
	if stats.RefreshErrors != 1 {
		t.Errorf("RefreshErrors should be 1, got %d", stats.RefreshErrors)
	}
}

func TestDaemon_GetProfileHealth_FallbackToParsing(t *testing.T) {
	tmpDir := t.TempDir()
	v := authfile.NewVault(tmpDir)
	// No health store - simulate fallback to parsing files
	hs := health.NewStorage(filepath.Join(tmpDir, "health.json"))

	// Create a Claude profile with auth file
	profileDir := filepath.Join(tmpDir, "claude", "test")
	os.MkdirAll(profileDir, 0700)
	// Create an auth file that health.ParseClaudeExpiry can parse
	// (the function looks for specific files)

	cfg := &Config{
		CheckInterval: 50 * time.Millisecond,
	}

	d := New(v, hs, cfg)

	// This will try health store first (empty), then fallback to parsing files
	ph := d.getProfileHealth("claude", "test")
	// Will likely be nil since we haven't created a proper auth file
	_ = ph // Just exercising the code path
}

func TestDaemonStartAndStop(t *testing.T) {
	tmpDir := t.TempDir()
	v := authfile.NewVault(tmpDir)
	hs := health.NewStorage(filepath.Join(tmpDir, "health.json"))

	cfg := &Config{
		CheckInterval:    50 * time.Millisecond,
		RefreshThreshold: 1 * time.Minute,
		Verbose:          false,
	}

	d := New(v, hs, cfg)

	// Start daemon in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Start()
	}()

	// Wait a bit for it to start
	time.Sleep(100 * time.Millisecond)

	if !d.IsRunning() {
		t.Error("daemon should be running after Start")
	}

	// Check that it did at least one check
	stats := d.GetStats()
	if stats.CheckCount < 1 {
		t.Errorf("expected at least 1 check, got %d", stats.CheckCount)
	}

	// Stop daemon
	if err := d.Stop(); err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	if d.IsRunning() {
		t.Error("daemon should not be running after Stop")
	}

	// Get the result from Start (should be nil after Stop)
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Start returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Start did not return after Stop")
	}
}

func TestDaemonDoubleStart(t *testing.T) {
	tmpDir := t.TempDir()
	v := authfile.NewVault(tmpDir)
	hs := health.NewStorage(filepath.Join(tmpDir, "health.json"))

	cfg := &Config{
		CheckInterval:    50 * time.Millisecond,
		RefreshThreshold: 1 * time.Minute,
	}

	d := New(v, hs, cfg)

	// Start daemon in goroutine
	go func() {
		_ = d.Start()
	}()

	// Wait for it to start
	time.Sleep(100 * time.Millisecond)
	defer d.Stop()

	// Second start should fail
	err := d.Start()
	if err == nil {
		t.Error("expected error on double start")
	}
}
