package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestLoadMissingReturnsMissingError(t *testing.T) {
	t.Parallel()

	_, err := Load(filepath.Join(t.TempDir(), "config.json"))
	var missing *MissingError
	if !errors.As(err, &missing) {
		t.Fatalf("got %T, want MissingError", err)
	}
}

func TestLoadAndResolveWatchConfig(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{
  "watch": {
    "interval": "1h",
    "repositories": ["owner/repo", "owner/repo", "org/private"]
  }
}`), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	interval, err := cfg.WatchInterval("")
	if err != nil {
		t.Fatalf("WatchInterval returned error: %v", err)
	}
	if interval != time.Hour {
		t.Fatalf("interval %s, want 1h", interval)
	}
	override, err := cfg.WatchInterval("30m")
	if err != nil {
		t.Fatalf("WatchInterval override returned error: %v", err)
	}
	if override != 30*time.Minute {
		t.Fatalf("override %s, want 30m", override)
	}

	repos, err := cfg.WatchRepositories()
	if err != nil {
		t.Fatalf("WatchRepositories returned error: %v", err)
	}
	want := []string{"owner/repo", "org/private"}
	if !reflect.DeepEqual(repos, want) {
		t.Fatalf("repos %v, want %v", repos, want)
	}
}

func TestDefaultWatchInterval(t *testing.T) {
	t.Parallel()

	interval, err := (Config{}).WatchInterval("")
	if err != nil {
		t.Fatalf("WatchInterval returned error: %v", err)
	}
	if interval != 30*time.Minute {
		t.Fatalf("interval %s, want 30m", interval)
	}
}

func TestWatchRepositoriesRequired(t *testing.T) {
	t.Parallel()

	if _, err := (Config{}).WatchRepositories(); err == nil {
		t.Fatal("expected error")
	}
}

func TestSaveAndLoadOrDefault(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "raku", "config.json")
	cfg, err := LoadOrDefault(path)
	if err != nil {
		t.Fatalf("LoadOrDefault returned error: %v", err)
	}
	if len(cfg.Watch.Repositories) != 0 {
		t.Fatalf("repositories %v, want empty", cfg.Watch.Repositories)
	}

	if err := cfg.AddWatchRepository("owner/repo"); err != nil {
		t.Fatalf("AddWatchRepository returned error: %v", err)
	}
	if err := cfg.SetWatchInterval("45m"); err != nil {
		t.Fatalf("SetWatchInterval returned error: %v", err)
	}
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("file mode %v, want 0600", mode)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loaded.Watch.Interval != "45m" {
		t.Fatalf("interval %q, want 45m", loaded.Watch.Interval)
	}
	if !reflect.DeepEqual(loaded.Watch.Repositories, []string{"owner/repo"}) {
		t.Fatalf("repositories %v, want owner/repo", loaded.Watch.Repositories)
	}
}

func TestWatchRepositoryMutations(t *testing.T) {
	t.Parallel()

	cfg := Config{}
	if err := cfg.AddWatchRepository("owner/repo"); err != nil {
		t.Fatalf("AddWatchRepository returned error: %v", err)
	}
	if err := cfg.AddWatchRepository("owner/repo"); err != nil {
		t.Fatalf("duplicate AddWatchRepository returned error: %v", err)
	}
	if err := cfg.AddWatchRepository("org/private"); err != nil {
		t.Fatalf("AddWatchRepository returned error: %v", err)
	}
	want := []string{"owner/repo", "org/private"}
	if !reflect.DeepEqual(cfg.Watch.Repositories, want) {
		t.Fatalf("repositories %v, want %v", cfg.Watch.Repositories, want)
	}

	if err := cfg.RemoveWatchRepository("owner/repo"); err != nil {
		t.Fatalf("RemoveWatchRepository returned error: %v", err)
	}
	if !reflect.DeepEqual(cfg.Watch.Repositories, []string{"org/private"}) {
		t.Fatalf("repositories %v, want org/private", cfg.Watch.Repositories)
	}
	if err := cfg.RemoveWatchRepository("missing/repo"); err == nil {
		t.Fatal("expected error for missing repo")
	}
}

func TestWatchRepositoryValidation(t *testing.T) {
	t.Parallel()

	cfg := Config{}
	for _, repo := range []string{"", "owner", "owner/repo/extra", "/repo", "owner/"} {
		if err := cfg.AddWatchRepository(repo); err == nil {
			t.Fatalf("expected error for repo %q", repo)
		}
	}
}

func TestSetWatchIntervalValidation(t *testing.T) {
	t.Parallel()

	cfg := Config{}
	if err := cfg.SetWatchInterval("1h"); err != nil {
		t.Fatalf("SetWatchInterval returned error: %v", err)
	}
	if cfg.Watch.Interval != "1h" {
		t.Fatalf("interval %q, want 1h", cfg.Watch.Interval)
	}
	for _, interval := range []string{"", "0m", "-1m", "abc"} {
		if err := cfg.SetWatchInterval(interval); err == nil {
			t.Fatalf("expected error for interval %q", interval)
		}
	}
}
