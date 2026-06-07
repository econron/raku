package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const Version = 1

type File struct {
	Version     int                             `json:"version"`
	Seen        map[string]map[string]SeenEntry `json:"seen"`
	CurrentView map[string]CurrentView          `json:"current_view"`
	Watch       WatchState                      `json:"watch"`
}

type SeenEntry struct {
	Fingerprint string `json:"fingerprint"`
	SeenAt      string `json:"seen_at"`
}

type CurrentView struct {
	CreatedAt string     `json:"created_at"`
	Items     []ViewItem `json:"items"`
}

type ViewItem struct {
	Alias       int    `json:"alias"`
	Key         string `json:"key"`
	Fingerprint string `json:"fingerprint"`
}

type WatchState struct {
	ReviewRequests map[string]WatchReviewRequestEntry `json:"review_requests"`
}

type WatchReviewRequestEntry struct {
	Repo       string `json:"repo"`
	Number     int    `json:"number"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	UpdatedAt  string `json:"updated_at"`
	NotifiedAt string `json:"notified_at"`
}

type Store struct {
	Path string
}

func DefaultPath() (string, error) {
	if stateHome := os.Getenv("XDG_STATE_HOME"); stateHome != "" {
		return filepath.Join(stateHome, "raku", "state.json"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "raku", "state.json"), nil
}

func NewStore(path string) *Store {
	return &Store{Path: path}
}

func (s *Store) Load() (*File, error) {
	if s.Path == "" {
		return nil, errors.New("state path is empty")
	}

	data, err := os.ReadFile(s.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewFile(), nil
		}
		return nil, err
	}

	var file File
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}
	file.normalize()
	return &file, nil
}

func (s *Store) Save(file *File) error {
	if s.Path == "" {
		return errors.New("state path is empty")
	}
	if file == nil {
		return errors.New("state file is nil")
	}
	file.normalize()

	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(s.Path), ".state-*.json")
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
	if err := encoder.Encode(file); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, s.Path)
}

func NewFile() *File {
	file := &File{Version: Version}
	file.normalize()
	return file
}

func (f *File) HasSeenPR(prKey string) bool {
	if f == nil || f.Seen == nil {
		return false
	}
	_, ok := f.Seen[prKey]
	return ok
}

func (f *File) SeenFor(prKey string) map[string]SeenEntry {
	f.normalize()
	if _, ok := f.Seen[prKey]; !ok {
		f.Seen[prKey] = map[string]SeenEntry{}
	}
	return f.Seen[prKey]
}

func (f *File) MarkSeen(prKey, eventKey, fingerprint string, seenAt time.Time) {
	seen := f.SeenFor(prKey)
	seen[eventKey] = SeenEntry{
		Fingerprint: fingerprint,
		SeenAt:      seenAt.UTC().Format(time.RFC3339),
	}
}

func (f *File) SetCurrentView(prKey string, view CurrentView) {
	f.normalize()
	f.CurrentView[prKey] = view
}

func (f *File) ClearCurrentView(prKey string) {
	if f == nil || f.CurrentView == nil {
		return
	}
	delete(f.CurrentView, prKey)
}

func (f *File) normalize() {
	if f.Version == 0 {
		f.Version = Version
	}
	if f.Seen == nil {
		f.Seen = map[string]map[string]SeenEntry{}
	}
	if f.CurrentView == nil {
		f.CurrentView = map[string]CurrentView{}
	}
	if f.Watch.ReviewRequests == nil {
		f.Watch.ReviewRequests = map[string]WatchReviewRequestEntry{}
	}
}
