package db

import (
	"sync"
	"testing"
	"time"
)

func TestDB_LogEventAndStats(t *testing.T) {
	tmpDir := t.TempDir()
	d, err := OpenAt(tmpDir + "/caam.db")
	if err != nil {
		t.Fatalf("OpenAt() error = %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	if err := d.LogEvent(Event{
		Type:        EventActivate,
		Provider:    "codex",
		ProfileName: "work",
		Details:     map[string]any{"from": "test"},
	}); err != nil {
		t.Fatalf("LogEvent(activate) error = %v", err)
	}

	if err := d.LogEvent(Event{
		Type:        EventError,
		Provider:    "codex",
		ProfileName: "work",
		Details:     map[string]any{"err": "boom"},
	}); err != nil {
		t.Fatalf("LogEvent(error) error = %v", err)
	}

	if err := d.LogEvent(Event{
		Type:        EventDeactivate,
		Provider:    "codex",
		ProfileName: "work",
		Duration:    90 * time.Second,
	}); err != nil {
		t.Fatalf("LogEvent(deactivate) error = %v", err)
	}

	stats, err := d.GetStats("codex", "work")
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if stats == nil {
		t.Fatalf("GetStats() = nil, want stats")
	}
	if stats.TotalActivations != 1 {
		t.Fatalf("TotalActivations = %d, want 1", stats.TotalActivations)
	}
	if stats.TotalErrors != 1 {
		t.Fatalf("TotalErrors = %d, want 1", stats.TotalErrors)
	}
	if stats.TotalActiveSeconds != 90 {
		t.Fatalf("TotalActiveSeconds = %d, want 90", stats.TotalActiveSeconds)
	}
	if stats.LastActivated.IsZero() {
		t.Fatalf("LastActivated is zero, want non-zero")
	}
	if stats.LastError.IsZero() {
		t.Fatalf("LastError is zero, want non-zero")
	}

	events, err := d.GetEvents("codex", "work", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetEvents() error = %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("GetEvents() len = %d, want 3", len(events))
	}
}

func TestDB_LogEvent_Concurrent(t *testing.T) {
	tmpDir := t.TempDir()
	d, err := OpenAt(tmpDir + "/caam.db")
	if err != nil {
		t.Fatalf("OpenAt() error = %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	const (
		goroutines = 10
		perWorker  = 25
	)

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*perWorker)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < perWorker; j++ {
				if err := d.LogEvent(Event{
					Type:        EventActivate,
					Provider:    "claude",
					ProfileName: "work",
					Details:     map[string]any{"worker": worker, "n": j},
				}); err != nil {
					errCh <- err
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("concurrent LogEvent error = %v", err)
	}

	stats, err := d.GetStats("claude", "work")
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if stats == nil {
		t.Fatalf("GetStats() = nil, want stats")
	}
	want := goroutines * perWorker
	if stats.TotalActivations != want {
		t.Fatalf("TotalActivations = %d, want %d", stats.TotalActivations, want)
	}
}
