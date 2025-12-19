package sync

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestParseAddress tests the ParseAddress function.
func TestParseAddress(t *testing.T) {
	tests := []struct {
		input    string
		wantHost string
		wantPort int
		wantUser string
	}{
		// Simple hostname
		{"example.com", "example.com", 0, ""},
		{"192.168.1.100", "192.168.1.100", 0, ""},

		// With port
		{"example.com:22", "example.com", 22, ""},
		{"192.168.1.100:2222", "192.168.1.100", 2222, ""},

		// With user
		{"user@example.com", "example.com", 0, "user"},
		{"jeff@192.168.1.100", "192.168.1.100", 0, "jeff"},

		// With user and port
		{"user@example.com:22", "example.com", 22, "user"},
		{"jeff@192.168.1.100:2222", "192.168.1.100", 2222, "jeff"},

		// IPv6
		{"[::1]", "::1", 0, ""},
		{"[::1]:22", "::1", 22, ""},
		{"user@[::1]:22", "::1", 22, "user"},

		// Edge cases
		{"", "", 0, ""},
		{"  example.com  ", "example.com", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			host, port, user := ParseAddress(tt.input)
			if host != tt.wantHost {
				t.Errorf("ParseAddress(%q) host = %q, want %q", tt.input, host, tt.wantHost)
			}
			if port != tt.wantPort {
				t.Errorf("ParseAddress(%q) port = %d, want %d", tt.input, port, tt.wantPort)
			}
			if user != tt.wantUser {
				t.Errorf("ParseAddress(%q) user = %q, want %q", tt.input, user, tt.wantUser)
			}
		})
	}
}

// TestNormalizeAddress tests the NormalizeAddress function.
func TestNormalizeAddress(t *testing.T) {
	tests := []struct {
		host string
		port int
		user string
		want string
	}{
		{"example.com", 0, "", "example.com"},
		{"example.com", 22, "", "example.com"}, // Default port omitted
		{"example.com", 2222, "", "example.com:2222"},
		{"example.com", 0, "jeff", "jeff@example.com"},
		{"example.com", 2222, "jeff", "jeff@example.com:2222"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := NormalizeAddress(tt.host, tt.port, tt.user)
			if got != tt.want {
				t.Errorf("NormalizeAddress(%q, %d, %q) = %q, want %q",
					tt.host, tt.port, tt.user, got, tt.want)
			}
		})
	}
}

// TestMachine tests Machine operations.
func TestMachine(t *testing.T) {
	m := NewMachine("test-machine", "192.168.1.100")

	// Check defaults
	if m.Port != DefaultSSHPort {
		t.Errorf("New machine port = %d, want %d", m.Port, DefaultSSHPort)
	}
	if m.Status != StatusUnknown {
		t.Errorf("New machine status = %q, want %q", m.Status, StatusUnknown)
	}
	if m.Source != SourceManual {
		t.Errorf("New machine source = %q, want %q", m.Source, SourceManual)
	}
	if m.ID == "" {
		t.Error("New machine should have an ID")
	}

	// Test HostPort
	expected := "192.168.1.100:22"
	if got := m.HostPort(); got != expected {
		t.Errorf("HostPort() = %q, want %q", got, expected)
	}

	// Test status updates
	m.SetError("connection refused")
	if m.Status != StatusError {
		t.Errorf("After SetError, status = %q, want %q", m.Status, StatusError)
	}
	if m.LastError != "connection refused" {
		t.Errorf("After SetError, LastError = %q, want %q", m.LastError, "connection refused")
	}

	m.SetOnline()
	if m.Status != StatusOnline {
		t.Errorf("After SetOnline, status = %q, want %q", m.Status, StatusOnline)
	}
	if m.LastError != "" {
		t.Error("After SetOnline, LastError should be cleared")
	}

	// Test validation
	if err := m.Validate(); err != nil {
		t.Errorf("Valid machine should not error: %v", err)
	}

	emptyMachine := &Machine{}
	if err := emptyMachine.Validate(); err == nil {
		t.Error("Empty machine should fail validation")
	}
}

// TestMachinesEqual tests the MachinesEqual function.
func TestMachinesEqual(t *testing.T) {
	m1 := NewMachine("m1", "192.168.1.100")
	m1.Port = 22

	m2 := NewMachine("m2", "192.168.1.100")
	m2.Port = 22

	m3 := NewMachine("m3", "192.168.1.100")
	m3.Port = 2222

	m4 := NewMachine("m4", "192.168.1.200")
	m4.Port = 22

	if !MachinesEqual(m1, m2) {
		t.Error("Machines with same address:port should be equal")
	}
	if MachinesEqual(m1, m3) {
		t.Error("Machines with different port should not be equal")
	}
	if MachinesEqual(m1, m4) {
		t.Error("Machines with different address should not be equal")
	}
	if MachinesEqual(nil, m1) || MachinesEqual(m1, nil) {
		t.Error("nil machines should not be equal to non-nil")
	}
	if !MachinesEqual(nil, nil) {
		t.Error("nil == nil should be true")
	}
}

// TestSyncPool tests SyncPool operations.
func TestSyncPool(t *testing.T) {
	pool := NewSyncPool()

	// Check defaults
	if pool.Enabled {
		t.Error("New pool should have Enabled = false")
	}
	if pool.AutoSync {
		t.Error("New pool should have AutoSync = false")
	}
	if !pool.IsEmpty() {
		t.Error("New pool should be empty")
	}

	// Add machine
	m := NewMachine("test", "192.168.1.100")
	if err := pool.AddMachine(m); err != nil {
		t.Errorf("AddMachine failed: %v", err)
	}

	if pool.IsEmpty() {
		t.Error("Pool should not be empty after AddMachine")
	}
	if pool.MachineCount() != 1 {
		t.Errorf("Pool count = %d, want 1", pool.MachineCount())
	}

	// Get by ID
	if got := pool.GetMachine(m.ID); got != m {
		t.Error("GetMachine should return added machine")
	}

	// Get by name
	if got := pool.GetMachineByName("test"); got != m {
		t.Error("GetMachineByName should return added machine")
	}

	// Duplicate name should fail
	m2 := NewMachine("test", "192.168.1.200")
	if err := pool.AddMachine(m2); err == nil {
		t.Error("Adding duplicate name should fail")
	}

	// Remove machine
	if err := pool.RemoveMachine(m.ID); err != nil {
		t.Errorf("RemoveMachine failed: %v", err)
	}
	if !pool.IsEmpty() {
		t.Error("Pool should be empty after RemoveMachine")
	}

	// Remove non-existent should fail
	if err := pool.RemoveMachine("nonexistent"); err == nil {
		t.Error("Removing non-existent machine should fail")
	}
}

// TestSyncPoolPersistence tests saving and loading SyncPool.
func TestSyncPoolPersistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Set XDG_DATA_HOME to redirect data storage
	oldXDG := os.Getenv("XDG_DATA_HOME")
	os.Setenv("XDG_DATA_HOME", tmpDir)
	defer os.Setenv("XDG_DATA_HOME", oldXDG)

	// Create and save pool
	pool := NewSyncPool()
	pool.Enable()

	m := NewMachine("test", "192.168.1.100")
	if err := pool.AddMachine(m); err != nil {
		t.Fatalf("AddMachine failed: %v", err)
	}

	if err := pool.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load pool
	loaded, err := LoadSyncPool()
	if err != nil {
		t.Fatalf("LoadSyncPool failed: %v", err)
	}

	if !loaded.Enabled {
		t.Error("Loaded pool should have Enabled = true")
	}
	if loaded.MachineCount() != 1 {
		t.Errorf("Loaded pool count = %d, want 1", loaded.MachineCount())
	}
	if loaded.GetMachineByName("test") == nil {
		t.Error("Loaded pool should have test machine")
	}
}

// TestLocalIdentity tests identity creation and loading.
func TestLocalIdentity(t *testing.T) {
	tmpDir := t.TempDir()

	// Set XDG_DATA_HOME to redirect data storage
	oldXDG := os.Getenv("XDG_DATA_HOME")
	os.Setenv("XDG_DATA_HOME", tmpDir)
	defer os.Setenv("XDG_DATA_HOME", oldXDG)

	// Create identity
	identity, err := GetOrCreateLocalIdentity()
	if err != nil {
		t.Fatalf("GetOrCreateLocalIdentity failed: %v", err)
	}

	if identity.ID == "" {
		t.Error("Identity should have an ID")
	}
	if identity.Hostname == "" {
		t.Error("Identity should have a hostname")
	}
	if identity.CreatedAt.IsZero() {
		t.Error("Identity should have CreatedAt")
	}

	// Load same identity
	loaded, err := GetOrCreateLocalIdentity()
	if err != nil {
		t.Fatalf("Second GetOrCreateLocalIdentity failed: %v", err)
	}

	if loaded.ID != identity.ID {
		t.Error("Loading should return same identity")
	}
}

// TestSSHConfigParsing tests SSH config file parsing.
func TestSSHConfigParsing(t *testing.T) {
	sshConfig := `# Test SSH config
Host work-laptop
    HostName 192.168.1.100
    Port 22
    User jeff
    IdentityFile ~/.ssh/work_key

Host home-desktop
    HostName 10.0.0.50
    Port 2222
    User admin

Host github.com
    HostName github.com
    User git

Host *
    AddKeysToAgent yes

Host proxy-server
    HostName 192.168.1.200
    ProxyJump bastion
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")
	if err := os.WriteFile(configPath, []byte(sshConfig), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	machines, err := parseSSHConfig(configPath)
	if err != nil {
		t.Fatalf("parseSSHConfig failed: %v", err)
	}

	// Should have work-laptop and home-desktop (not github.com, not *, not proxy-server)
	if len(machines) != 2 {
		t.Errorf("Expected 2 machines, got %d", len(machines))
		for _, m := range machines {
			t.Logf("  Machine: %s (%s)", m.Name, m.Address)
		}
	}

	// Check work-laptop
	var workLaptop *Machine
	for _, m := range machines {
		if m.Name == "work-laptop" {
			workLaptop = m
			break
		}
	}

	if workLaptop == nil {
		t.Fatal("work-laptop not found")
	}
	if workLaptop.Address != "192.168.1.100" {
		t.Errorf("work-laptop address = %q, want %q", workLaptop.Address, "192.168.1.100")
	}
	if workLaptop.Port != 22 {
		t.Errorf("work-laptop port = %d, want 22", workLaptop.Port)
	}
	if workLaptop.SSHUser != "jeff" {
		t.Errorf("work-laptop user = %q, want %q", workLaptop.SSHUser, "jeff")
	}
	if workLaptop.Source != SourceSSHConfig {
		t.Errorf("work-laptop source = %q, want %q", workLaptop.Source, SourceSSHConfig)
	}
}

// TestCSVOperations tests CSV file operations.
func TestCSVOperations(t *testing.T) {
	tmpDir := t.TempDir()

	// Set HOME to redirect CSV file location
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create CSV directory
	csvDir := filepath.Join(tmpDir, ".caam")
	if err := os.MkdirAll(csvDir, 0700); err != nil {
		t.Fatalf("Failed to create CSV dir: %v", err)
	}

	// Write test CSV
	csvContent := `# Test CSV
machine_name,address,ssh_key_path
work-laptop,192.168.1.100,~/.ssh/id_ed25519
home-desktop,admin@10.0.0.50:2222,~/.ssh/home_key
`
	csvPath := filepath.Join(csvDir, CSVFileName)
	if err := os.WriteFile(csvPath, []byte(csvContent), 0600); err != nil {
		t.Fatalf("Failed to write CSV: %v", err)
	}

	// Load from CSV
	machines, err := LoadFromCSV()
	if err != nil {
		t.Fatalf("LoadFromCSV failed: %v", err)
	}

	if len(machines) != 2 {
		t.Errorf("Expected 2 machines, got %d", len(machines))
	}

	// Check work-laptop
	var found bool
	for _, m := range machines {
		if m.Name == "work-laptop" {
			found = true
			if m.Address != "192.168.1.100" {
				t.Errorf("work-laptop address = %q, want %q", m.Address, "192.168.1.100")
			}
			if m.Source != SourceCSV {
				t.Errorf("work-laptop source = %q, want %q", m.Source, SourceCSV)
			}
		}
	}
	if !found {
		t.Error("work-laptop not found in loaded machines")
	}
}

// TestSyncQueue tests queue operations.
func TestSyncQueue(t *testing.T) {
	state := NewSyncState(t.TempDir())

	// Add entries
	state.AddToQueue("claude", "alice@gmail.com", "m1", "connection error")
	state.AddToQueue("codex", "work@company.com", "m2", "timeout")

	if len(state.Queue.Entries) != 2 {
		t.Errorf("Queue len = %d, want 2", len(state.Queue.Entries))
	}

	// Adding same entry should update, not duplicate
	state.AddToQueue("claude", "alice@gmail.com", "m1", "retry error")
	if len(state.Queue.Entries) != 2 {
		t.Errorf("Queue len after duplicate = %d, want 2", len(state.Queue.Entries))
	}

	// Check that attempts was incremented
	for _, e := range state.Queue.Entries {
		if e.Provider == "claude" && e.Profile == "alice@gmail.com" {
			if e.Attempts != 2 {
				t.Errorf("Attempts = %d, want 2", e.Attempts)
			}
		}
	}

	// Remove from queue
	state.RemoveFromQueue("claude", "alice@gmail.com", "m1")
	if len(state.Queue.Entries) != 1 {
		t.Errorf("Queue len after remove = %d, want 1", len(state.Queue.Entries))
	}
}

// TestSyncHistory tests history operations.
func TestSyncHistory(t *testing.T) {
	state := NewSyncState(t.TempDir())

	// Add entries
	for i := 0; i < 5; i++ {
		state.AddToHistory(HistoryEntry{
			Timestamp: time.Now(),
			Provider:  "claude",
			Profile:   "test",
			Machine:   "test-machine",
			Success:   i%2 == 0,
		})
	}

	if len(state.History.Entries) != 5 {
		t.Errorf("History len = %d, want 5", len(state.History.Entries))
	}

	// Recent should return in reverse order
	recent := state.RecentHistory(3)
	if len(recent) != 3 {
		t.Errorf("RecentHistory(3) len = %d, want 3", len(recent))
	}

	// Most recent should be first
	if recent[0].Timestamp.Before(recent[1].Timestamp) {
		t.Error("Recent history should be in reverse chronological order")
	}
}

// TestSyncStatePersistence tests full state save/load cycle.
func TestSyncStatePersistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Set XDG_DATA_HOME to redirect data storage
	oldXDG := os.Getenv("XDG_DATA_HOME")
	os.Setenv("XDG_DATA_HOME", tmpDir)
	defer os.Setenv("XDG_DATA_HOME", oldXDG)

	// Create state with base path in temp dir
	basePath := filepath.Join(tmpDir, "caam", "sync")
	state := NewSyncState(basePath)

	// First load to create identity
	if err := state.Load(); err != nil {
		t.Fatalf("Initial Load failed: %v", err)
	}

	// Add some data
	m := NewMachine("test", "192.168.1.100")
	if err := state.Pool.AddMachine(m); err != nil {
		t.Fatalf("AddMachine failed: %v", err)
	}
	state.Pool.Enable()

	state.AddToQueue("claude", "test", m.ID, "test error")
	state.AddToHistory(HistoryEntry{
		Timestamp: time.Now(),
		Provider:  "claude",
		Success:   true,
	})

	// Save
	if err := state.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load into new state
	loaded := NewSyncState(basePath)
	if err := loaded.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !loaded.Pool.Enabled {
		t.Error("Loaded pool should be enabled")
	}
	if loaded.Pool.MachineCount() != 1 {
		t.Errorf("Loaded pool count = %d, want 1", loaded.Pool.MachineCount())
	}
	if len(loaded.Queue.Entries) != 1 {
		t.Errorf("Loaded queue len = %d, want 1", len(loaded.Queue.Entries))
	}
	if len(loaded.History.Entries) != 1 {
		t.Errorf("Loaded history len = %d, want 1", len(loaded.History.Entries))
	}
}

// TestMergeDiscoveredMachines tests machine merging.
func TestMergeDiscoveredMachines(t *testing.T) {
	existing := []*Machine{
		NewMachine("m1", "192.168.1.100"),
		NewMachine("m2", "192.168.1.101"),
	}

	discovered := []*Machine{
		NewMachine("m1", "10.0.0.1"),   // Same name, different address - should be skipped
		NewMachine("m3", "192.168.1.102"), // New - should be added
	}

	merged := MergeDiscoveredMachines(existing, discovered)

	if len(merged) != 3 {
		t.Errorf("Merged len = %d, want 3", len(merged))
	}

	// m1 should have original address (existing takes precedence)
	for _, m := range merged {
		if m.Name == "m1" && m.Address != "192.168.1.100" {
			t.Errorf("m1 address = %q, want %q (existing should take precedence)", m.Address, "192.168.1.100")
		}
	}
}
