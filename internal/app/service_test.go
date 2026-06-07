package app

import (
	"context"
	"reflect"
	"testing"
	"time"

	"raku/internal/domain"
	"raku/internal/state"
)

func TestParseAliases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		want    []int
		wantErr bool
	}{
		{name: "single", args: []string{"1"}, want: []int{1}},
		{name: "multiple", args: []string{"3", "1"}, want: []int{1, 3}},
		{name: "range", args: []string{"1-3"}, want: []int{1, 2, 3}},
		{name: "dedupe", args: []string{"1", "1-2"}, want: []int{1, 2}},
		{name: "zero", args: []string{"0"}, wantErr: true},
		{name: "reverse range", args: []string{"3-1"}, wantErr: true},
		{name: "bad", args: []string{"x"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAliases(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetUninitializedDoesNotSaveCurrentView(t *testing.T) {
	t.Parallel()

	store := &memoryStore{file: state.NewFile()}
	service := testService(store, []domain.Event{testEvent("issue_comment", 1, "hello")})

	result, err := service.Get(context.Background(), GetOptions{})
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !result.Uninitialized {
		t.Fatal("expected uninitialized result")
	}
	if store.saves != 0 {
		t.Fatalf("unexpected save count: %d", store.saves)
	}
	if len(store.file.CurrentView) != 0 {
		t.Fatalf("current view was saved: %#v", store.file.CurrentView)
	}
}

func TestGetAllSavesCurrentView(t *testing.T) {
	t.Parallel()

	store := &memoryStore{file: state.NewFile()}
	service := testService(store, []domain.Event{testEvent("issue_comment", 1, "hello")})

	result, err := service.Get(context.Background(), GetOptions{All: true})
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if len(result.Events) != 1 {
		t.Fatalf("got %d events, want 1", len(result.Events))
	}
	if result.Events[0].Alias != 1 || result.Events[0].Status != StatusNew {
		t.Fatalf("unexpected event: %#v", result.Events[0])
	}
	view := store.file.CurrentView["owner/repo#123"]
	if len(view.Items) != 1 {
		t.Fatalf("got %d current_view items, want 1", len(view.Items))
	}
	if view.Items[0].Key != "issue_comment:1" {
		t.Fatalf("unexpected view item: %#v", view.Items[0])
	}
}

func TestGetUnreadShowsUpdatedOnly(t *testing.T) {
	t.Parallel()

	oldEvent := testEvent("issue_comment", 1, "old")
	newEvent := testEvent("issue_comment", 1, "new")
	store := &memoryStore{file: state.NewFile()}
	store.file.MarkSeen("owner/repo#123", oldEvent.Key, oldEvent.Fingerprint, fixedTime())

	service := testService(store, []domain.Event{newEvent})
	result, err := service.Get(context.Background(), GetOptions{})
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if len(result.Events) != 1 {
		t.Fatalf("got %d events, want 1", len(result.Events))
	}
	if result.Events[0].Status != StatusUpdated {
		t.Fatalf("got status %q, want %q", result.Events[0].Status, StatusUpdated)
	}
}

func TestSeenCurrentViewMarksAliases(t *testing.T) {
	t.Parallel()

	event := testEvent("issue_comment", 1, "hello")
	store := &memoryStore{file: state.NewFile()}
	store.file.SetCurrentView("owner/repo#123", state.CurrentView{
		CreatedAt: fixedTime().Format(time.RFC3339),
		Items: []state.ViewItem{{
			Alias:       1,
			Key:         event.Key,
			Fingerprint: event.Fingerprint,
		}},
	})

	service := testService(store, nil)
	result, err := service.Seen(context.Background(), SeenOptions{Args: []string{"1"}})
	if err != nil {
		t.Fatalf("Seen returned error: %v", err)
	}
	if result.Marked != 1 {
		t.Fatalf("got marked %d, want 1", result.Marked)
	}
	seen := store.file.Seen["owner/repo#123"][event.Key]
	if seen.Fingerprint != event.Fingerprint {
		t.Fatalf("seen fingerprint %q, want %q", seen.Fingerprint, event.Fingerprint)
	}
}

func TestSeenBaselineMarksFetchedEvents(t *testing.T) {
	t.Parallel()

	event := testEvent("issue_comment", 1, "hello")
	store := &memoryStore{file: state.NewFile()}
	store.file.SetCurrentView("owner/repo#123", state.CurrentView{
		Items: []state.ViewItem{{Alias: 1, Key: event.Key, Fingerprint: "old"}},
	})

	service := testService(store, []domain.Event{event})
	result, err := service.Seen(context.Background(), SeenOptions{Baseline: true})
	if err != nil {
		t.Fatalf("Seen returned error: %v", err)
	}
	if result.Marked != 1 {
		t.Fatalf("got marked %d, want 1", result.Marked)
	}
	if _, ok := store.file.CurrentView["owner/repo#123"]; ok {
		t.Fatal("current_view was not cleared")
	}
	seen := store.file.Seen["owner/repo#123"][event.Key]
	if seen.Fingerprint != event.Fingerprint {
		t.Fatalf("seen fingerprint %q, want %q", seen.Fingerprint, event.Fingerprint)
	}
}

func TestGetPassesIncludeSelf(t *testing.T) {
	t.Parallel()

	store := &memoryStore{file: state.NewFile()}
	store.file.SeenFor("owner/repo#123")
	github := &fakeGitHub{events: []domain.Event{testEvent("issue_comment", 1, "hello")}}
	service := &Service{
		GitHub: github,
		State:  store,
		Clock:  fixedTime,
	}

	if _, err := service.Get(context.Background(), GetOptions{IncludeSelf: true}); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !github.includeSelf {
		t.Fatal("includeSelf was not passed to GitHub.Events")
	}
}

func TestSeenBaselinePassesIncludeSelf(t *testing.T) {
	t.Parallel()

	store := &memoryStore{file: state.NewFile()}
	github := &fakeGitHub{events: []domain.Event{testEvent("issue_comment", 1, "hello")}}
	service := &Service{
		GitHub: github,
		State:  store,
		Clock:  fixedTime,
	}

	if _, err := service.Seen(context.Background(), SeenOptions{Baseline: true, IncludeSelf: true}); err != nil {
		t.Fatalf("Seen returned error: %v", err)
	}
	if !github.includeSelf {
		t.Fatal("includeSelf was not passed to GitHub.Events")
	}
}

func TestSeenIncludeSelfWithoutBaselineReturnsError(t *testing.T) {
	t.Parallel()

	store := &memoryStore{file: state.NewFile()}
	service := testService(store, nil)

	if _, err := service.Seen(context.Background(), SeenOptions{IncludeSelf: true, Args: []string{"1"}}); err == nil {
		t.Fatal("expected error")
	}
}

func testService(store *memoryStore, events []domain.Event) *Service {
	return &Service{
		GitHub: &fakeGitHub{events: events},
		State:  store,
		Clock:  fixedTime,
	}
}

func testEvent(eventType string, id int64, body string) domain.Event {
	event := domain.Event{
		Key:       domain.EventKey(eventType, id),
		Type:      eventType,
		ID:        id,
		Author:    "reviewer",
		CreatedAt: "2026-06-06T10:00:00Z",
		UpdatedAt: "2026-06-06T10:00:00Z",
		Body:      body,
		URL:       "https://github.com/owner/repo/pull/123#comment",
	}
	event, err := domain.WithFingerprint(event)
	if err != nil {
		panic(err)
	}
	return event
}

func fixedTime() time.Time {
	return time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC)
}

type fakeGitHub struct {
	events      []domain.Event
	includeSelf bool
}

func (g *fakeGitHub) Preflight(context.Context) error {
	return nil
}

func (g *fakeGitHub) CurrentUser(context.Context) (string, error) {
	return "me", nil
}

func (g *fakeGitHub) Repo(context.Context) (string, error) {
	return "owner/repo", nil
}

func (g *fakeGitHub) PullRequest(context.Context) (domain.PR, error) {
	return domain.PR{
		Number:  123,
		Title:   "Test PR",
		URL:     "https://github.com/owner/repo/pull/123",
		Author:  "me",
		HeadRef: "feature",
		BaseRef: "main",
	}, nil
}

func (g *fakeGitHub) Events(_ context.Context, _ int, _ string, includeSelf bool) ([]domain.Event, error) {
	g.includeSelf = includeSelf
	return g.events, nil
}

type memoryStore struct {
	file  *state.File
	saves int
}

func (s *memoryStore) Load() (*state.File, error) {
	if s.file == nil {
		s.file = state.NewFile()
	}
	return s.file, nil
}

func (s *memoryStore) Save(file *state.File) error {
	s.file = file
	s.saves++
	return nil
}
