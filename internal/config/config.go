package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultWatchInterval = 30 * time.Minute

type Config struct {
	Watch Watch `json:"watch"`
}

type Watch struct {
	Interval     string   `json:"interval"`
	Repositories []string `json:"repositories"`
}

type MissingError struct {
	Path string
}

func (e *MissingError) Error() string {
	return fmt.Sprintf("config file not found: %s", e.Path)
}

func DefaultPath() (string, error) {
	if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		return filepath.Join(configHome, "raku", "config.json"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "raku", "config.json"), nil
}

func Load(path string) (Config, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return Config{}, err
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, &MissingError{Path: path}
		}
		return Config{}, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	cfg.normalize()
	return cfg, nil
}

func LoadOrDefault(path string) (Config, error) {
	cfg, err := Load(path)
	if err == nil {
		return cfg, nil
	}
	var missing *MissingError
	if errors.As(err, &missing) {
		cfg := Config{}
		cfg.normalize()
		return cfg, nil
	}
	return Config{}, err
}

func Save(path string, cfg Config) error {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return err
		}
	}

	cfg.normalize()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}

	encoder := json.NewEncoder(tmp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(cfg); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func (c Config) WatchInterval(override string) (time.Duration, error) {
	value := override
	if value == "" {
		value = c.Watch.Interval
	}
	if value == "" {
		return defaultWatchInterval, nil
	}

	interval, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid watch interval %q: %w", value, err)
	}
	if interval <= 0 {
		return 0, fmt.Errorf("watch interval must be positive: %s", value)
	}
	return interval, nil
}

func (c Config) WatchRepositories() ([]string, error) {
	if len(c.Watch.Repositories) == 0 {
		return nil, errors.New("watch.repositories is required")
	}

	repos := make([]string, 0, len(c.Watch.Repositories))
	seen := map[string]bool{}
	for _, repo := range c.Watch.Repositories {
		if repo == "" {
			return nil, errors.New("watch.repositories contains an empty repository")
		}
		if seen[repo] {
			continue
		}
		seen[repo] = true
		repos = append(repos, repo)
	}
	return repos, nil
}

func (c *Config) AddWatchRepository(repo string) error {
	repo, err := normalizeRepo(repo)
	if err != nil {
		return err
	}
	c.normalize()
	for _, existing := range c.Watch.Repositories {
		if existing == repo {
			return nil
		}
	}
	c.Watch.Repositories = append(c.Watch.Repositories, repo)
	return nil
}

func (c *Config) RemoveWatchRepository(repo string) error {
	repo, err := normalizeRepo(repo)
	if err != nil {
		return err
	}
	c.normalize()
	next := c.Watch.Repositories[:0]
	removed := false
	for _, existing := range c.Watch.Repositories {
		if existing == repo {
			removed = true
			continue
		}
		next = append(next, existing)
	}
	if !removed {
		return fmt.Errorf("watch repository %q is not configured", repo)
	}
	c.Watch.Repositories = next
	return nil
}

func (c *Config) SetWatchInterval(interval string) error {
	interval = strings.TrimSpace(interval)
	if interval == "" {
		return errors.New("watch interval is required")
	}
	if _, err := (Config{}).WatchInterval(interval); err != nil {
		return err
	}
	c.Watch.Interval = interval
	c.normalize()
	return nil
}

func Sample() string {
	return `{
  "watch": {
    "interval": "30m",
    "repositories": [
      "owner/repo",
      "org/private-repo"
    ]
  }
}`
}

func (c *Config) normalize() {
	if c.Watch.Repositories == nil {
		c.Watch.Repositories = []string{}
	}
	repos := make([]string, 0, len(c.Watch.Repositories))
	seen := map[string]bool{}
	for _, repo := range c.Watch.Repositories {
		repo = strings.TrimSpace(repo)
		if repo == "" || seen[repo] {
			continue
		}
		seen[repo] = true
		repos = append(repos, repo)
	}
	c.Watch.Repositories = repos
}

func normalizeRepo(repo string) (string, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return "", errors.New("repository is required")
	}
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("repository must be owner/repo: %q", repo)
	}
	return repo, nil
}
