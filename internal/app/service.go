package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"raku/internal/domain"
	"raku/internal/state"
)

const (
	ModeUnread        = "unread"
	ModeAll           = "all"
	ModeUninitialized = "uninitialized"

	StatusNew     = "new"
	StatusUpdated = "updated"
	StatusSeen    = "seen"
)

type GitHub interface {
	Preflight(ctx context.Context) error
	CurrentUser(ctx context.Context) (string, error)
	Repo(ctx context.Context) (string, error)
	PullRequest(ctx context.Context) (domain.PR, error)
	Events(ctx context.Context, prNumber int, viewer string, includeSelf bool) ([]domain.Event, error)
}

type StateStore interface {
	Load() (*state.File, error)
	Save(file *state.File) error
}

type Service struct {
	GitHub GitHub
	State  StateStore
	Clock  func() time.Time
}

type GetOptions struct {
	All         bool
	IncludeSelf bool
}

type SeenOptions struct {
	All         bool
	Baseline    bool
	IncludeSelf bool
	Args        []string
}

type SeenResult struct {
	Repo     string
	PR       domain.PR
	PRKey    string
	Marked   int
	Baseline bool
	All      bool
	Aliases  []int
}

func (s *Service) Get(ctx context.Context, opts GetOptions) (domain.Result, error) {
	file, full, err := s.loadFullContext(ctx, true, opts.IncludeSelf)
	if err != nil {
		return domain.Result{}, err
	}

	prKey := makePRKey(full.Repo, full.PR.Number)
	counts := domain.CountEvents(full.Events)
	if !opts.All && !file.HasSeenPR(prKey) {
		return domain.Result{
			Repo:          full.Repo,
			PR:            full.PR,
			Mode:          ModeUninitialized,
			Uninitialized: true,
			Counts:        &counts,
			Events:        []domain.Event{},
		}, nil
	}

	seen := file.Seen[prKey]
	display := make([]domain.Event, 0, len(full.Events))
	for _, event := range full.Events {
		event.Status = classifyEvent(seen, event)
		if opts.All || event.Status != StatusSeen {
			display = append(display, event)
		}
	}

	for i := range display {
		display[i].Alias = i + 1
	}

	file.SetCurrentView(prKey, state.CurrentView{
		CreatedAt: s.now().UTC().Format(time.RFC3339),
		Items:     viewItems(display),
	})
	if err := s.State.Save(file); err != nil {
		return domain.Result{}, err
	}

	mode := ModeUnread
	if opts.All {
		mode = ModeAll
	}
	return domain.Result{
		Repo:   full.Repo,
		PR:     full.PR,
		Mode:   mode,
		Events: display,
	}, nil
}

func (s *Service) Seen(ctx context.Context, opts SeenOptions) (SeenResult, error) {
	if opts.IncludeSelf && !opts.Baseline {
		return SeenResult{}, errors.New("--include-self can only be used with --baseline")
	}
	if opts.All && opts.Baseline {
		return SeenResult{}, errors.New("--all and --baseline cannot be used together")
	}
	if opts.Baseline && len(opts.Args) > 0 {
		return SeenResult{}, errors.New("--baseline does not accept aliases")
	}
	if opts.All && len(opts.Args) > 0 {
		return SeenResult{}, errors.New("--all does not accept aliases")
	}
	if !opts.All && !opts.Baseline && len(opts.Args) == 0 {
		return SeenResult{}, errors.New("alias is required")
	}

	if opts.Baseline {
		return s.seenBaseline(ctx, opts.IncludeSelf)
	}
	return s.seenCurrentView(ctx, opts)
}

func (s *Service) seenBaseline(ctx context.Context, includeSelf bool) (SeenResult, error) {
	file, full, err := s.loadFullContext(ctx, true, includeSelf)
	if err != nil {
		return SeenResult{}, err
	}

	prKey := makePRKey(full.Repo, full.PR.Number)
	for _, event := range full.Events {
		file.MarkSeen(prKey, event.Key, event.Fingerprint, s.now())
	}
	file.ClearCurrentView(prKey)

	if err := s.State.Save(file); err != nil {
		return SeenResult{}, err
	}
	return SeenResult{
		Repo:     full.Repo,
		PR:       full.PR,
		PRKey:    prKey,
		Marked:   len(full.Events),
		Baseline: true,
	}, nil
}

func (s *Service) seenCurrentView(ctx context.Context, opts SeenOptions) (SeenResult, error) {
	file, full, err := s.loadFullContext(ctx, false, false)
	if err != nil {
		return SeenResult{}, err
	}

	prKey := makePRKey(full.Repo, full.PR.Number)
	view, ok := file.CurrentView[prKey]
	if !ok {
		return SeenResult{}, fmt.Errorf("no current_view for %s; run raku get pr-comment first", prKey)
	}

	var aliases []int
	if opts.All {
		for _, item := range view.Items {
			aliases = append(aliases, item.Alias)
		}
	} else {
		aliases, err = ParseAliases(opts.Args)
		if err != nil {
			return SeenResult{}, err
		}
	}

	itemsByAlias := map[int]state.ViewItem{}
	for _, item := range view.Items {
		itemsByAlias[item.Alias] = item
	}

	seenAt := s.now()
	marked := 0
	for _, alias := range aliases {
		item, ok := itemsByAlias[alias]
		if !ok {
			return SeenResult{}, fmt.Errorf("alias %d is not in current_view; run raku get pr-comment again", alias)
		}
		file.MarkSeen(prKey, item.Key, item.Fingerprint, seenAt)
		marked++
	}

	if err := s.State.Save(file); err != nil {
		return SeenResult{}, err
	}

	return SeenResult{
		Repo:    full.Repo,
		PR:      full.PR,
		PRKey:   prKey,
		Marked:  marked,
		All:     opts.All,
		Aliases: aliases,
	}, nil
}

func ParseAliases(args []string) ([]int, error) {
	seen := map[int]bool{}
	for _, arg := range args {
		parts := strings.Split(arg, "-")
		switch len(parts) {
		case 1:
			value, err := parseAlias(parts[0])
			if err != nil {
				return nil, err
			}
			seen[value] = true
		case 2:
			start, err := parseAlias(parts[0])
			if err != nil {
				return nil, err
			}
			end, err := parseAlias(parts[1])
			if err != nil {
				return nil, err
			}
			if start > end {
				return nil, fmt.Errorf("invalid alias range %q", arg)
			}
			for value := start; value <= end; value++ {
				seen[value] = true
			}
		default:
			return nil, fmt.Errorf("invalid alias %q", arg)
		}
	}

	aliases := make([]int, 0, len(seen))
	for alias := range seen {
		aliases = append(aliases, alias)
	}
	sort.Ints(aliases)
	return aliases, nil
}

type fullContext struct {
	Repo   string
	PR     domain.PR
	Viewer string
	Events []domain.Event
}

func (s *Service) loadFullContext(ctx context.Context, fetchEvents bool, includeSelf bool) (*state.File, fullContext, error) {
	if s.GitHub == nil {
		return nil, fullContext{}, errors.New("github client is nil")
	}
	if s.State == nil {
		return nil, fullContext{}, errors.New("state store is nil")
	}

	if err := s.GitHub.Preflight(ctx); err != nil {
		return nil, fullContext{}, err
	}

	repo, err := s.GitHub.Repo(ctx)
	if err != nil {
		return nil, fullContext{}, err
	}
	pr, err := s.GitHub.PullRequest(ctx)
	if err != nil {
		return nil, fullContext{}, err
	}
	viewer, err := s.GitHub.CurrentUser(ctx)
	if err != nil {
		return nil, fullContext{}, err
	}
	if pr.Author != viewer {
		return nil, fullContext{}, fmt.Errorf("current PR author is %q, not current gh user %q", pr.Author, viewer)
	}

	file, err := s.State.Load()
	if err != nil {
		return nil, fullContext{}, err
	}

	full := fullContext{Repo: repo, PR: pr, Viewer: viewer}
	if fetchEvents {
		events, err := s.GitHub.Events(ctx, pr.Number, viewer, includeSelf)
		if err != nil {
			return nil, fullContext{}, err
		}
		full.Events = events
	}
	return file, full, nil
}

func classifyEvent(seen map[string]state.SeenEntry, event domain.Event) string {
	entry, ok := seen[event.Key]
	if !ok {
		return StatusNew
	}
	if entry.Fingerprint != event.Fingerprint {
		return StatusUpdated
	}
	return StatusSeen
}

func viewItems(events []domain.Event) []state.ViewItem {
	items := make([]state.ViewItem, 0, len(events))
	for _, event := range events {
		items = append(items, state.ViewItem{
			Alias:       event.Alias,
			Key:         event.Key,
			Fingerprint: event.Fingerprint,
		})
	}
	return items
}

func makePRKey(repo string, prNumber int) string {
	return fmt.Sprintf("%s#%d", repo, prNumber)
}

func parseAlias(value string) (int, error) {
	alias, err := strconv.Atoi(value)
	if err != nil || alias <= 0 {
		return 0, fmt.Errorf("invalid alias %q", value)
	}
	return alias, nil
}

func (s *Service) now() time.Time {
	if s.Clock != nil {
		return s.Clock()
	}
	return time.Now()
}
