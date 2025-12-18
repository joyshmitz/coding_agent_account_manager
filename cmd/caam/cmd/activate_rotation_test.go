package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/authfile"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/config"
	caamdb "github.com/Dicklesworthstone/coding_agent_account_manager/internal/db"
	"github.com/spf13/cobra"
)

func TestActivate_AutoSelect_ChoosesNonCooldownProfile(t *testing.T) {
	tmpDir := t.TempDir()

	oldCodexHome := os.Getenv("CODEX_HOME")
	t.Cleanup(func() { _ = os.Setenv("CODEX_HOME", oldCodexHome) })
	_ = os.Setenv("CODEX_HOME", filepath.Join(tmpDir, "codex_home"))

	oldCaamHome := os.Getenv("CAAM_HOME")
	t.Cleanup(func() { _ = os.Setenv("CAAM_HOME", oldCaamHome) })
	_ = os.Setenv("CAAM_HOME", filepath.Join(tmpDir, "caam_home"))

	if err := os.MkdirAll(os.Getenv("CODEX_HOME"), 0700); err != nil {
		t.Fatalf("MkdirAll(CODEX_HOME) error = %v", err)
	}
	if err := os.MkdirAll(os.Getenv("CAAM_HOME"), 0700); err != nil {
		t.Fatalf("MkdirAll(CAAM_HOME) error = %v", err)
	}

	// Create current auth state.
	authPath := filepath.Join(os.Getenv("CODEX_HOME"), "auth.json")
	if err := os.WriteFile(authPath, []byte(`{"access_token":"current"}`), 0600); err != nil {
		t.Fatalf("WriteFile(current auth) error = %v", err)
	}

	// Use a temp vault with two profiles.
	oldVault := vault
	vault = authfile.NewVault(filepath.Join(tmpDir, "vault"))
	t.Cleanup(func() { vault = oldVault })

	if err := os.MkdirAll(vault.ProfilePath("codex", "a"), 0700); err != nil {
		t.Fatalf("MkdirAll(profile a) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault.ProfilePath("codex", "a"), "auth.json"), []byte(`{"access_token":"a"}`), 0600); err != nil {
		t.Fatalf("WriteFile(profile a) error = %v", err)
	}
	if err := os.MkdirAll(vault.ProfilePath("codex", "b"), 0700); err != nil {
		t.Fatalf("MkdirAll(profile b) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault.ProfilePath("codex", "b"), "auth.json"), []byte(`{"access_token":"b"}`), 0600); err != nil {
		t.Fatalf("WriteFile(profile b) error = %v", err)
	}

	// Put profile a in cooldown.
	db, err := caamdb.Open()
	if err != nil {
		t.Fatalf("db.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.SetCooldown("codex", "a", time.Now().UTC(), 60*time.Minute, ""); err != nil {
		t.Fatalf("SetCooldown() error = %v", err)
	}

	c := &cobra.Command{}
	c.Flags().Bool("backup-current", false, "")
	c.Flags().Bool("force", false, "")
	c.Flags().Bool("auto", true, "")
	_ = c.Flags().Set("auto", "true")

	if err := runActivate(c, []string{"codex"}); err != nil {
		t.Fatalf("runActivate(--auto) error = %v", err)
	}

	got, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatalf("ReadFile(active auth) error = %v", err)
	}
	if string(got) != `{"access_token":"b"}` {
		t.Fatalf("auth mismatch: got %q want %q", string(got), `{"access_token":"b"}`)
	}
}

func TestActivate_NoProfileNoDefault_UsesRotationWhenEnabled(t *testing.T) {
	tmpDir := t.TempDir()

	oldCodexHome := os.Getenv("CODEX_HOME")
	t.Cleanup(func() { _ = os.Setenv("CODEX_HOME", oldCodexHome) })
	_ = os.Setenv("CODEX_HOME", filepath.Join(tmpDir, "codex_home"))

	oldCaamHome := os.Getenv("CAAM_HOME")
	t.Cleanup(func() { _ = os.Setenv("CAAM_HOME", oldCaamHome) })
	_ = os.Setenv("CAAM_HOME", filepath.Join(tmpDir, "caam_home"))

	if err := os.MkdirAll(os.Getenv("CODEX_HOME"), 0700); err != nil {
		t.Fatalf("MkdirAll(CODEX_HOME) error = %v", err)
	}
	if err := os.MkdirAll(os.Getenv("CAAM_HOME"), 0700); err != nil {
		t.Fatalf("MkdirAll(CAAM_HOME) error = %v", err)
	}

	// Enable rotation in SPM config.
	spmCfg := []byte("version: 1\nstealth:\n  rotation:\n    enabled: true\n    algorithm: smart\n")
	if err := os.WriteFile(config.SPMConfigPath(), spmCfg, 0600); err != nil {
		t.Fatalf("WriteFile(config.yaml) error = %v", err)
	}

	// Ensure caam global config has no default profiles for this test.
	oldCfg := cfg
	cfg = nil
	t.Cleanup(func() { cfg = oldCfg })

	// Use a temp vault with one profile.
	oldVault := vault
	vault = authfile.NewVault(filepath.Join(tmpDir, "vault"))
	t.Cleanup(func() { vault = oldVault })

	if err := os.MkdirAll(vault.ProfilePath("codex", "only"), 0700); err != nil {
		t.Fatalf("MkdirAll(profile) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault.ProfilePath("codex", "only"), "auth.json"), []byte(`{"access_token":"only"}`), 0600); err != nil {
		t.Fatalf("WriteFile(profile auth) error = %v", err)
	}

	c := &cobra.Command{}
	c.Flags().Bool("backup-current", false, "")
	c.Flags().Bool("force", false, "")
	c.Flags().Bool("auto", false, "")

	if err := runActivate(c, []string{"codex"}); err != nil {
		t.Fatalf("runActivate() error = %v", err)
	}

	authPath := filepath.Join(os.Getenv("CODEX_HOME"), "auth.json")
	got, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatalf("ReadFile(active auth) error = %v", err)
	}
	if string(got) != `{"access_token":"only"}` {
		t.Fatalf("auth mismatch: got %q want %q", string(got), `{"access_token":"only"}`)
	}
}

func TestActivate_DefaultInCooldown_AutoSelectsAlternative(t *testing.T) {
	tmpDir := t.TempDir()

	oldCodexHome := os.Getenv("CODEX_HOME")
	t.Cleanup(func() { _ = os.Setenv("CODEX_HOME", oldCodexHome) })
	_ = os.Setenv("CODEX_HOME", filepath.Join(tmpDir, "codex_home"))

	oldCaamHome := os.Getenv("CAAM_HOME")
	t.Cleanup(func() { _ = os.Setenv("CAAM_HOME", oldCaamHome) })
	_ = os.Setenv("CAAM_HOME", filepath.Join(tmpDir, "caam_home"))

	if err := os.MkdirAll(os.Getenv("CODEX_HOME"), 0700); err != nil {
		t.Fatalf("MkdirAll(CODEX_HOME) error = %v", err)
	}
	if err := os.MkdirAll(os.Getenv("CAAM_HOME"), 0700); err != nil {
		t.Fatalf("MkdirAll(CAAM_HOME) error = %v", err)
	}

	// Enable rotation in SPM config.
	spmCfg := []byte("version: 1\nstealth:\n  rotation:\n    enabled: true\n    algorithm: smart\n")
	if err := os.WriteFile(config.SPMConfigPath(), spmCfg, 0600); err != nil {
		t.Fatalf("WriteFile(config.yaml) error = %v", err)
	}

	// Set caam global config default to profile a.
	oldCfg := cfg
	cfg = config.DefaultConfig()
	cfg.SetDefault("codex", "a")
	t.Cleanup(func() { cfg = oldCfg })

	// Use a temp vault with two profiles.
	oldVault := vault
	vault = authfile.NewVault(filepath.Join(tmpDir, "vault"))
	t.Cleanup(func() { vault = oldVault })

	if err := os.MkdirAll(vault.ProfilePath("codex", "a"), 0700); err != nil {
		t.Fatalf("MkdirAll(profile a) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault.ProfilePath("codex", "a"), "auth.json"), []byte(`{"access_token":"a"}`), 0600); err != nil {
		t.Fatalf("WriteFile(profile a) error = %v", err)
	}
	if err := os.MkdirAll(vault.ProfilePath("codex", "b"), 0700); err != nil {
		t.Fatalf("MkdirAll(profile b) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault.ProfilePath("codex", "b"), "auth.json"), []byte(`{"access_token":"b"}`), 0600); err != nil {
		t.Fatalf("WriteFile(profile b) error = %v", err)
	}

	// Put default profile a in cooldown.
	db, err := caamdb.Open()
	if err != nil {
		t.Fatalf("db.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.SetCooldown("codex", "a", time.Now().UTC(), 60*time.Minute, ""); err != nil {
		t.Fatalf("SetCooldown() error = %v", err)
	}

	c := &cobra.Command{}
	c.Flags().Bool("backup-current", false, "")
	c.Flags().Bool("force", false, "")
	c.Flags().Bool("auto", false, "")

	if err := runActivate(c, []string{"codex"}); err != nil {
		t.Fatalf("runActivate() error = %v", err)
	}

	authPath := filepath.Join(os.Getenv("CODEX_HOME"), "auth.json")
	got, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatalf("ReadFile(active auth) error = %v", err)
	}
	if string(got) != `{"access_token":"b"}` {
		t.Fatalf("auth mismatch: got %q want %q", string(got), `{"access_token":"b"}`)
	}
}
