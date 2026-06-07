package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"raku/internal/domain"
	"raku/internal/state"
)

type ReviewRequestGitHub interface {
	Preflight(ctx context.Context) error
	ReviewRequests(ctx context.Context, repo string) ([]domain.ReviewRequest, error)
}

type ReviewRequestNotifier interface {
	NotifyReviewRequest(ctx context.Context, request domain.ReviewRequest) error
}

type WatchService struct {
	GitHub   ReviewRequestGitHub
	State    StateStore
	Notifier ReviewRequestNotifier
	Clock    func() time.Time
}

type WatchPollResult struct {
	CheckedRepos int
	Active       int
	Notified     int
	Cleared      int
	NotifyErrors []error
}

func (s *WatchService) PollReviewRequests(ctx context.Context, repos []string) (WatchPollResult, error) {
	if s.GitHub == nil {
		return WatchPollResult{}, errors.New("github client is nil")
	}
	if s.State == nil {
		return WatchPollResult{}, errors.New("state store is nil")
	}
	if s.Notifier == nil {
		return WatchPollResult{}, errors.New("notifier is nil")
	}
	if len(repos) == 0 {
		return WatchPollResult{}, errors.New("at least one repository is required")
	}

	if err := s.GitHub.Preflight(ctx); err != nil {
		return WatchPollResult{}, err
	}

	file, err := s.State.Load()
	if err != nil {
		return WatchPollResult{}, err
	}

	result := WatchPollResult{CheckedRepos: len(repos)}
	active := map[string]domain.ReviewRequest{}
	watchedRepos := map[string]bool{}

	for _, repo := range repos {
		repo = strings.TrimSpace(repo)
		if repo == "" {
			return WatchPollResult{}, errors.New("repository name is empty")
		}
		watchedRepos[repo] = true

		requests, err := s.GitHub.ReviewRequests(ctx, repo)
		if err != nil {
			return result, err
		}
		for _, request := range requests {
			active[request.Key()] = request
		}
	}
	result.Active = len(active)

	changed := false
	for key, entry := range file.Watch.ReviewRequests {
		if !watchedRepos[entry.Repo] {
			continue
		}
		if _, ok := active[key]; !ok {
			delete(file.Watch.ReviewRequests, key)
			result.Cleared++
			changed = true
		}
	}

	for key, request := range active {
		if _, ok := file.Watch.ReviewRequests[key]; ok {
			continue
		}
		if err := s.Notifier.NotifyReviewRequest(ctx, request); err != nil {
			result.NotifyErrors = append(result.NotifyErrors, fmt.Errorf("%s: %w", key, err))
			continue
		}
		file.Watch.ReviewRequests[key] = state.WatchReviewRequestEntry{
			Repo:       request.Repo,
			Number:     request.Number,
			Title:      request.Title,
			URL:        request.URL,
			UpdatedAt:  request.UpdatedAt,
			NotifiedAt: s.now().UTC().Format(time.RFC3339),
		}
		result.Notified++
		changed = true
	}

	if changed {
		if err := s.State.Save(file); err != nil {
			return result, err
		}
	}

	if len(result.NotifyErrors) > 0 {
		return result, result.NotifyErrors[0]
	}
	return result, nil
}

func (s *WatchService) now() time.Time {
	if s.Clock != nil {
		return s.Clock()
	}
	return time.Now()
}
