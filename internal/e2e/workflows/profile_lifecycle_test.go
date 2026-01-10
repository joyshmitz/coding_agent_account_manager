package workflows

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/authfile"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/testutil"
)

// TestE2E_ProfileLifecycleWorkflow tests the complete lifecycle of a profile:
// Create -> Activate -> Verify -> Delete -> Verify Gone
func TestE2E_ProfileLifecycleWorkflow(t *testing.T) {
	h := testutil.NewExtendedHarness(t)
	defer h.Close()

	// ==========================================================================
	// Phase 1: Setup
	// ==========================================================================
	h.StartStep("setup", "Setting up test environment")
	homeDir := h.SubDir("home")
	vaultDir := h.SubDir("vault")

	// Setup Codex home
	codexHome := filepath.Join(homeDir, ".codex")
	if err := os.MkdirAll(codexHome, 0700); err != nil {
		t.Fatalf("Failed to create codex home: %v", err)
	}
	codexAuthPath := filepath.Join(codexHome, "auth.json")

	fileSet := authfile.AuthFileSet{
		Tool: "codex",
		Files: []authfile.AuthFileSpec{
			{Tool: "codex", Path: codexAuthPath, Required: true},
		},
	}

	vault := authfile.NewVault(vaultDir)
	h.EndStep("setup")

	// ==========================================================================
	// Phase 2: Create Profile (Backup)
	// ==========================================================================
	h.StartStep("create_profile", "Creating new profile")
	profileName := "lifecycle-test"

	// Create source auth file
	content := map[string]interface{}{
		"access_token": "lifecycle-token",
		"created_at":   "now",
	}
	jsonBytes, _ := json.MarshalIndent(content, "", "  ")
	if err := os.WriteFile(codexAuthPath, jsonBytes, 0600); err != nil {
		t.Fatalf("Failed to write auth file: %v", err)
	}

	// Backup (Create)
	if err := vault.Backup(fileSet, profileName); err != nil {
		t.Fatalf("Backup failed: %v", err)
	}

	// Verify existence in vault
	profiles, err := vault.List("codex")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	found := false
	for _, p := range profiles {
		if p == profileName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Profile %s not found in list after creation", profileName)
	}
	h.LogInfo("Profile created", "name", profileName)
	h.EndStep("create_profile")

	// ==========================================================================
	// Phase 3: Activate Profile
	// ==========================================================================
	h.StartStep("activate_profile", "Activating profile")

	// Clear local auth file to verify restoration
	if err := os.Remove(codexAuthPath); err != nil {
		t.Fatalf("Failed to remove local auth: %v", err)
	}

	// Activate
	if err := vault.Restore(fileSet, profileName); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// Verify file restored
	if _, err := os.Stat(codexAuthPath); os.IsNotExist(err) {
		t.Errorf("Auth file not restored")
	}

	// Verify active status
	active, err := vault.ActiveProfile(fileSet)
	if err != nil {
		t.Fatalf("ActiveProfile failed: %v", err)
	}
	if active != profileName {
		t.Errorf("Expected active profile %s, got %s", profileName, active)
	}
	h.LogInfo("Profile activated", "active", active)
	h.EndStep("activate_profile")

	// ==========================================================================
	// Phase 4: Delete Profile
	// ==========================================================================
	h.StartStep("delete_profile", "Deleting profile")

	// Delete from vault
	if err := vault.Delete("codex", profileName); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify gone from list
	profiles, err = vault.List("codex")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	for _, p := range profiles {
		if p == profileName {
			t.Errorf("Profile %s still present in list after deletion", profileName)
		}
	}

	// Verify directory gone
	profileDir := filepath.Join(vaultDir, "codex", profileName)
	if _, err := os.Stat(profileDir); !os.IsNotExist(err) {
		t.Errorf("Profile directory still exists: %s", profileDir)
	}

	h.LogInfo("Profile deleted")
	h.EndStep("delete_profile")

	t.Log("\n" + h.Summary())
}
