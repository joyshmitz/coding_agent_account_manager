package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStore_LoadMissingFile_ReturnsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(filepath.Join(tmpDir, "projects.json"))

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got == nil {
		t.Fatalf("Load() returned nil store")
	}
	if got.Version != 1 {
		t.Fatalf("Version = %d, want 1", got.Version)
	}
	if got.Associations == nil || got.Defaults == nil {
		t.Fatalf("expected maps to be initialized")
	}
}

func TestStore_LoadCorruptFile_ReturnsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "projects.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0600); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}

	store := NewStore(path)
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got == nil {
		t.Fatalf("Load() returned nil store")
	}
	if len(got.Associations) != 0 {
		t.Fatalf("Associations size = %d, want 0", len(got.Associations))
	}
}

func TestStore_SaveRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "projects.json")
	store := NewStore(path)

	data := &StoreData{
		Version: 1,
		Associations: map[string]map[string]string{
			"/tmp/project": {"claude": "work"},
		},
		Defaults: map[string]string{
			"codex": "main",
		},
	}

	if err := store.Save(data); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Version != 1 {
		t.Fatalf("Version = %d, want 1", got.Version)
	}
	if got.Defaults["codex"] != "main" {
		t.Fatalf("Defaults[codex] = %q, want %q", got.Defaults["codex"], "main")
	}
}

func TestStore_Resolve_InheritanceAndDefaults(t *testing.T) {
	base := t.TempDir()
	work := filepath.Join(base, "work")
	clientA := filepath.Join(work, "client-a")
	subdir := filepath.Join(clientA, "subdir")

	store := NewStore(filepath.Join(base, "projects.json"))
	if err := store.SetAssociation(work, "claude", "work@company.com"); err != nil {
		t.Fatalf("SetAssociation(work) error = %v", err)
	}
	if err := store.SetAssociation(clientA, "codex", "client-main"); err != nil {
		t.Fatalf("SetAssociation(clientA) error = %v", err)
	}
	if err := store.SetDefault("gemini", "personal"); err != nil {
		t.Fatalf("SetDefault(gemini) error = %v", err)
	}

	resolved, err := store.Resolve(subdir)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if got := resolved.Profiles["codex"]; got != "client-main" {
		t.Fatalf("codex = %q, want %q", got, "client-main")
	}
	if got := resolved.Profiles["claude"]; got != "work@company.com" {
		t.Fatalf("claude = %q, want %q", got, "work@company.com")
	}
	if got := resolved.Profiles["gemini"]; got != "personal" {
		t.Fatalf("gemini = %q, want %q", got, "personal")
	}

	if got := resolved.Sources["codex"]; got != filepath.Clean(clientA) {
		t.Fatalf("codex source = %q, want %q", got, filepath.Clean(clientA))
	}
	if got := resolved.Sources["claude"]; got != filepath.Clean(work) {
		t.Fatalf("claude source = %q, want %q", got, filepath.Clean(work))
	}
	if got := resolved.Sources["gemini"]; got != "<default>" {
		t.Fatalf("gemini source = %q, want %q", got, "<default>")
	}
}

func TestStore_Resolve_GlobPatterns_AndExactOverride(t *testing.T) {
	base := t.TempDir()
	work := filepath.Join(base, "work")
	clientA := filepath.Join(work, "client-a")

	store := NewStore(filepath.Join(base, "projects.json"))

	// Pattern matches /work/<anything>.
	if err := store.SetAssociation(filepath.Join(work, "*"), "claude", "pattern"); err != nil {
		t.Fatalf("SetAssociation(pattern) error = %v", err)
	}

	// Exact match should win for the same provider.
	if err := store.SetAssociation(clientA, "claude", "exact"); err != nil {
		t.Fatalf("SetAssociation(exact) error = %v", err)
	}

	resolved, err := store.Resolve(clientA)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got := resolved.Profiles["claude"]; got != "exact" {
		t.Fatalf("claude = %q, want %q", got, "exact")
	}
	if got := resolved.Sources["claude"]; got != filepath.Clean(clientA) {
		t.Fatalf("claude source = %q, want %q", got, filepath.Clean(clientA))
	}
}

func TestStore_Resolve_GlobSpecificity(t *testing.T) {
	base := t.TempDir()
	work := filepath.Join(base, "work")
	clientA := filepath.Join(work, "client-a")

	store := NewStore(filepath.Join(base, "projects.json"))

	// Two patterns that match clientA; the longer one should win.
	if err := store.SetAssociation(filepath.Join(work, "*"), "codex", "less-specific"); err != nil {
		t.Fatalf("SetAssociation(pattern1) error = %v", err)
	}
	if err := store.SetAssociation(filepath.Join(base, "*", "client-a"), "codex", "more-specific"); err != nil {
		t.Fatalf("SetAssociation(pattern2) error = %v", err)
	}

	resolved, err := store.Resolve(clientA)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got := resolved.Profiles["codex"]; got != "more-specific" {
		t.Fatalf("codex = %q, want %q", got, "more-specific")
	}
	if got := resolved.Sources["codex"]; got != filepath.Clean(filepath.Join(base, "*", "client-a")) {
		t.Fatalf("codex source = %q, want %q", got, filepath.Clean(filepath.Join(base, "*", "client-a")))
	}
}

func TestStore_NormalizesProviderAndProfile(t *testing.T) {
	base := t.TempDir()
	work := filepath.Join(base, "work")

	store := NewStore(filepath.Join(base, "projects.json"))

	if err := store.SetAssociation(work, "  CLAUDE  ", "  work@company.com  "); err != nil {
		t.Fatalf("SetAssociation() error = %v", err)
	}
	if err := store.SetDefault("  CODEX ", " main "); err != nil {
		t.Fatalf("SetDefault() error = %v", err)
	}

	resolved, err := store.Resolve(work)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if got := resolved.Profiles["claude"]; got != "work@company.com" {
		t.Fatalf("claude = %q, want %q", got, "work@company.com")
	}
	if got := resolved.Profiles["codex"]; got != "main" {
		t.Fatalf("codex = %q, want %q", got, "main")
	}

	// Ensure on-disk representation is normalized too.
	key, err := normalizeKey(work)
	if err != nil {
		t.Fatalf("normalizeKey() error = %v", err)
	}
	data, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := data.Associations[key]["claude"]; got != "work@company.com" {
		t.Fatalf("stored associations[%s][claude] = %q, want %q", key, got, "work@company.com")
	}
	if got := data.Defaults["codex"]; got != "main" {
		t.Fatalf("stored defaults[codex] = %q, want %q", got, "main")
	}
}

func TestStore_NormalizesExistingStoredData(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "projects.json")
	work := filepath.Join(base, "work")

	key, err := normalizeKey(work)
	if err != nil {
		t.Fatalf("normalizeKey() error = %v", err)
	}

	// Seed a file with unnormalized keys/values (as if edited manually or from older versions).
	seed := &StoreData{
		Version: 1,
		Associations: map[string]map[string]string{
			key: {"  CLAUDE  ": "  work@company.com  "},
		},
		Defaults: map[string]string{
			"  CODEX  ": " main ",
		},
	}
	raw, err := json.MarshalIndent(seed, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() error = %v", err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store := NewStore(path)

	// Ensure a normal command works even if the stored provider key is unnormalized.
	if err := store.RemoveAssociation(work, "claude"); err != nil {
		t.Fatalf("RemoveAssociation() error = %v", err)
	}

	// Ensure setting a normalized default doesn't leave multiple variants behind.
	if err := store.SetDefault("codex", "secondary"); err != nil {
		t.Fatalf("SetDefault() error = %v", err)
	}

	data, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if _, ok := data.Associations[key]; ok {
		t.Fatalf("expected associations[%s] to be removed", key)
	}
	if got := data.Defaults["codex"]; got != "secondary" {
		t.Fatalf("defaults[codex] = %q, want %q", got, "secondary")
	}
	if _, ok := data.Defaults["  CODEX  "]; ok {
		t.Fatalf("expected unnormalized default key to be removed")
	}
}

func TestGlobNormalization(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "projects.json")
	store := NewStore(storePath)

	// Intention: Allow any directory named "frontend" anywhere to use "work" profile
	// Pattern: */frontend
	err := store.SetAssociation("*/frontend", "claude", "work")
	if err != nil {
		t.Fatalf("SetAssociation failed: %v", err)
	}

	data, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// We expect one key
	if len(data.Associations) != 1 {
		t.Fatalf("expected 1 association, got %d", len(data.Associations))
	}

	// Extract the key
	var key string
	for k := range data.Associations {
		key = k
	}

	// Check if the key was absolutized
	if filepath.IsAbs(key) {
		t.Errorf("expected relative glob key '*/frontend', got absolute path '%s'", key)
	}
}
