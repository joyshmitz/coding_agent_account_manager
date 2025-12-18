package rotation

import (
	"path/filepath"
	"testing"
	"time"

	caamdb "github.com/Dicklesworthstone/coding_agent_account_manager/internal/db"
)

func TestSelectSmart_UsesLastActivationFromDB(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := caamdb.OpenAt(filepath.Join(tmpDir, "caam.db"))
	if err != nil {
		t.Fatalf("db.OpenAt() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	now := time.Now().UTC().Truncate(time.Second)

	// Recently used profile "a".
	if err := db.LogEvent(caamdb.Event{
		Type:        caamdb.EventActivate,
		Provider:    "codex",
		ProfileName: "a",
		Timestamp:   now.Add(-5 * time.Minute),
	}); err != nil {
		t.Fatalf("LogEvent(a) error = %v", err)
	}

	// Older use for profile "b".
	if err := db.LogEvent(caamdb.Event{
		Type:        caamdb.EventActivate,
		Provider:    "codex",
		ProfileName: "b",
		Timestamp:   now.Add(-2 * time.Hour),
	}); err != nil {
		t.Fatalf("LogEvent(b) error = %v", err)
	}

	s := NewSelector(AlgorithmSmart, nil, db)
	result, err := s.Select("codex", []string{"a", "b"}, "")
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if result.Selected != "b" {
		t.Fatalf("Selected = %q, want %q", result.Selected, "b")
	}
}
