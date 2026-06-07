package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingReturnsEmptyState(t *testing.T) {
	t.Parallel()

	store := NewStore(filepath.Join(t.TempDir(), "state.json"))
	file, err := store.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if file.Version != Version {
		t.Fatalf("version %d, want %d", file.Version, Version)
	}
	if file.Seen == nil || file.CurrentView == nil {
		t.Fatalf("state maps were not initialized: %#v", file)
	}
}

func TestSaveAndLoad(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "raku", "state.json")
	store := NewStore(path)
	file := NewFile()
	file.MarkSeen("owner/repo#123", "issue_comment:1", "sha256:test", time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC))
	file.SetCurrentView("owner/repo#123", CurrentView{
		CreatedAt: "2026-06-07T00:00:00Z",
		Items: []ViewItem{{
			Alias:       1,
			Key:         "issue_comment:1",
			Fingerprint: "sha256:test",
		}},
	})

	if err := store.Save(file); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("file mode %v, want 0600", mode)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	seen := loaded.Seen["owner/repo#123"]["issue_comment:1"]
	if seen.Fingerprint != "sha256:test" {
		t.Fatalf("fingerprint %q, want sha256:test", seen.Fingerprint)
	}
	if got := len(loaded.CurrentView["owner/repo#123"].Items); got != 1 {
		t.Fatalf("current_view items %d, want 1", got)
	}
}
