package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"raku/internal/domain"
	"raku/internal/state"
)

func TestWatchPollNotifiesOnlyNewRequests(t *testing.T) {
	t.Parallel()

	request := testReviewRequest(1)
	store := &memoryStore{file: state.NewFile()}
	github := &fakeWatchGitHub{requests: map[string][]domain.ReviewRequest{
		"owner/repo": {request},
	}}
	notifier := &fakeNotifier{}
	service := testWatchService(store, github, notifier)

	first, err := service.PollReviewRequests(context.Background(), []string{"owner/repo"})
	if err != nil {
		t.Fatalf("PollReviewRequests returned error: %v", err)
	}
	if first.Active != 1 || first.Notified != 1 {
		t.Fatalf("first result %#v", first)
	}
	if len(notifier.requests) != 1 {
		t.Fatalf("notified %d times, want 1", len(notifier.requests))
	}

	second, err := service.PollReviewRequests(context.Background(), []string{"owner/repo"})
	if err != nil {
		t.Fatalf("PollReviewRequests returned error: %v", err)
	}
	if second.Active != 1 || second.Notified != 0 {
		t.Fatalf("second result %#v", second)
	}
	if len(notifier.requests) != 1 {
		t.Fatalf("notified %d times, want still 1", len(notifier.requests))
	}
}

func TestWatchPollClearsInactiveAndRenotifies(t *testing.T) {
	t.Parallel()

	request := testReviewRequest(1)
	store := &memoryStore{file: state.NewFile()}
	github := &fakeWatchGitHub{requests: map[string][]domain.ReviewRequest{
		"owner/repo": {request},
	}}
	notifier := &fakeNotifier{}
	service := testWatchService(store, github, notifier)

	if _, err := service.PollReviewRequests(context.Background(), []string{"owner/repo"}); err != nil {
		t.Fatalf("initial PollReviewRequests returned error: %v", err)
	}

	github.requests["owner/repo"] = nil
	cleared, err := service.PollReviewRequests(context.Background(), []string{"owner/repo"})
	if err != nil {
		t.Fatalf("clear PollReviewRequests returned error: %v", err)
	}
	if cleared.Cleared != 1 {
		t.Fatalf("cleared %d, want 1", cleared.Cleared)
	}

	github.requests["owner/repo"] = []domain.ReviewRequest{request}
	renotify, err := service.PollReviewRequests(context.Background(), []string{"owner/repo"})
	if err != nil {
		t.Fatalf("renotify PollReviewRequests returned error: %v", err)
	}
	if renotify.Notified != 1 {
		t.Fatalf("renotify notified %d, want 1", renotify.Notified)
	}
	if len(notifier.requests) != 2 {
		t.Fatalf("notified %d times, want 2", len(notifier.requests))
	}
}

func TestWatchPollDoesNotMarkNotificationFailure(t *testing.T) {
	t.Parallel()

	request := testReviewRequest(1)
	store := &memoryStore{file: state.NewFile()}
	github := &fakeWatchGitHub{requests: map[string][]domain.ReviewRequest{
		"owner/repo": {request},
	}}
	notifier := &fakeNotifier{err: errors.New("notify failed")}
	service := testWatchService(store, github, notifier)

	result, err := service.PollReviewRequests(context.Background(), []string{"owner/repo"})
	if err == nil {
		t.Fatal("expected error")
	}
	if result.Notified != 0 {
		t.Fatalf("notified %d, want 0", result.Notified)
	}
	if _, ok := store.file.Watch.ReviewRequests[request.Key()]; ok {
		t.Fatal("failed notification was marked as notified")
	}
}

func testWatchService(store *memoryStore, github *fakeWatchGitHub, notifier *fakeNotifier) *WatchService {
	return &WatchService{
		GitHub:   github,
		State:    store,
		Notifier: notifier,
		Clock:    fixedTime,
	}
}

func testReviewRequest(number int) domain.ReviewRequest {
	return domain.ReviewRequest{
		Repo:      "owner/repo",
		Number:    number,
		Title:     "Need review",
		URL:       "https://github.com/owner/repo/pull/1",
		Author:    "teammate",
		UpdatedAt: fixedTime().Format(time.RFC3339),
	}
}

type fakeWatchGitHub struct {
	requests map[string][]domain.ReviewRequest
}

func (g *fakeWatchGitHub) Preflight(context.Context) error {
	return nil
}

func (g *fakeWatchGitHub) ReviewRequests(_ context.Context, repo string) ([]domain.ReviewRequest, error) {
	return g.requests[repo], nil
}

type fakeNotifier struct {
	requests []domain.ReviewRequest
	err      error
}

func (n *fakeNotifier) NotifyReviewRequest(_ context.Context, request domain.ReviewRequest) error {
	if n.err != nil {
		return n.err
	}
	n.requests = append(n.requests, request)
	return nil
}
