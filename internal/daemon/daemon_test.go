package daemon

import (
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
	// Save original PID file path and restore after test
	// For testing, we'll use a temp directory
	tmpDir := t.TempDir()
	originalPath := PIDFilePath()
	defer func() {
		// Clean up any PID file we created
		os.Remove(originalPath)
	}()

	// Write PID file
	if err := WritePIDFile(); err != nil {
		t.Fatalf("WritePIDFile failed: %v", err)
	}
	defer RemovePIDFile()

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
	_ = tmpDir // use tmpDir to avoid unused variable warning
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
