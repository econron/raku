package gh

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"raku/internal/domain"
)

type Runner interface {
	Run(ctx context.Context, args ...string) ([]byte, error)
}

type ExecRunner struct {
	Path string
}

func (r ExecRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	path := r.Path
	if path == "" {
		path = "gh"
	}

	cmd := exec.CommandContext(ctx, path, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return stdout.Bytes(), &CommandError{
			Args:   append([]string{path}, args...),
			Stdout: stdout.String(),
			Stderr: stderr.String(),
			Err:    err,
		}
	}
	return stdout.Bytes(), nil
}

type CommandError struct {
	Args   []string
	Stdout string
	Stderr string
	Err    error
}

func (e *CommandError) Error() string {
	if e == nil {
		return ""
	}
	if e.Stderr != "" {
		return strings.TrimSpace(e.Stderr)
	}
	if e.Stdout != "" {
		return strings.TrimSpace(e.Stdout)
	}
	return e.Err.Error()
}

func (e *CommandError) Unwrap() error {
	return e.Err
}

func (e *CommandError) CombinedOutput() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.Stdout + "\n" + e.Stderr)
}

type PreflightKind string

const (
	PreflightMissingGH PreflightKind = "missing_gh"
	PreflightAuth      PreflightKind = "auth"
	PreflightNetwork   PreflightKind = "network"
	PreflightUnknown   PreflightKind = "unknown"
)

type PreflightError struct {
	Kind   PreflightKind
	Detail string
}

func (e *PreflightError) Error() string {
	return e.Detail
}

type Client struct {
	Runner Runner
}

func NewClient(runner Runner) *Client {
	if runner == nil {
		runner = ExecRunner{}
	}
	return &Client{Runner: runner}
}

func (c *Client) Preflight(ctx context.Context) error {
	out, err := c.run(ctx, "auth", "status", "--json", "hosts")
	if err != nil {
		return classifyPreflightError(err)
	}

	var status authStatus
	if err := json.Unmarshal(out, &status); err != nil {
		return &PreflightError{
			Kind:   PreflightUnknown,
			Detail: fmt.Sprintf("gh auth status のJSONを解析できません: %v", err),
		}
	}

	accounts := status.Hosts["github.com"]
	if len(accounts) == 0 {
		return &PreflightError{
			Kind:   PreflightAuth,
			Detail: "github.com の gh 認証情報が見つかりません",
		}
	}

	active := accounts[0]
	for _, account := range accounts {
		if account.Active {
			active = account
			break
		}
	}

	if active.State == "success" {
		return nil
	}

	if isNetworkText(active.Error) {
		return &PreflightError{
			Kind:   PreflightNetwork,
			Detail: active.Error,
		}
	}

	detail := active.Error
	if detail == "" {
		detail = fmt.Sprintf("github.com の active account %q は認証済みではありません", active.Login)
	}
	return &PreflightError{Kind: PreflightAuth, Detail: detail}
}

func (c *Client) CurrentUser(ctx context.Context) (string, error) {
	out, err := c.run(ctx, "api", "user", "--jq", ".login")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *Client) Repo(ctx context.Context) (string, error) {
	out, err := c.run(ctx, "repo", "view", "--json", "nameWithOwner")
	if err != nil {
		return "", err
	}

	var repo struct {
		NameWithOwner string `json:"nameWithOwner"`
	}
	if err := json.Unmarshal(out, &repo); err != nil {
		return "", err
	}
	if repo.NameWithOwner == "" {
		return "", errors.New("gh repo view did not return nameWithOwner")
	}
	return repo.NameWithOwner, nil
}

func (c *Client) PullRequest(ctx context.Context) (domain.PR, error) {
	out, err := c.run(ctx, "pr", "view", "--json", "number,title,url,author,headRefName,baseRefName,headRefOid")
	if err != nil {
		return domain.PR{}, err
	}

	var raw struct {
		Number      int    `json:"number"`
		Title       string `json:"title"`
		URL         string `json:"url"`
		HeadRefName string `json:"headRefName"`
		BaseRefName string `json:"baseRefName"`
		HeadRefOID  string `json:"headRefOid"`
		Author      struct {
			Login string `json:"login"`
		} `json:"author"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return domain.PR{}, err
	}
	if raw.Number == 0 {
		return domain.PR{}, errors.New("gh pr view did not return a PR number")
	}

	return domain.PR{
		Number:     raw.Number,
		Title:      raw.Title,
		URL:        raw.URL,
		Author:     raw.Author.Login,
		HeadRef:    raw.HeadRefName,
		BaseRef:    raw.BaseRefName,
		HeadRefOID: raw.HeadRefOID,
	}, nil
}

func (c *Client) ReviewRequests(ctx context.Context, repo string) ([]domain.ReviewRequest, error) {
	out, err := c.run(
		ctx,
		"pr", "list",
		"--repo", repo,
		"--state", "open",
		"--search", "review-requested:@me",
		"--limit", "100",
		"--json", "number,title,url,author,updatedAt,isDraft",
	)
	if err != nil {
		return nil, err
	}

	var raw []reviewRequestPR
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}

	requests := make([]domain.ReviewRequest, 0, len(raw))
	for _, pr := range raw {
		requests = append(requests, domain.ReviewRequest{
			Repo:      repo,
			Number:    pr.Number,
			Title:     pr.Title,
			URL:       pr.URL,
			Author:    pr.Author.Login,
			UpdatedAt: pr.UpdatedAt,
			IsDraft:   pr.IsDraft,
		})
	}
	sort.SliceStable(requests, func(i, j int) bool {
		if requests[i].Repo == requests[j].Repo {
			return requests[i].Number < requests[j].Number
		}
		return requests[i].Repo < requests[j].Repo
	})
	return requests, nil
}

func (c *Client) Events(ctx context.Context, prNumber int, viewer string, includeSelf bool) ([]domain.Event, error) {
	issueComments, err := c.issueComments(ctx, prNumber, viewer, includeSelf)
	if err != nil {
		return nil, err
	}
	reviewComments, err := c.reviewComments(ctx, prNumber, viewer, includeSelf)
	if err != nil {
		return nil, err
	}
	reviews, err := c.reviews(ctx, prNumber, viewer, includeSelf)
	if err != nil {
		return nil, err
	}

	events := append(issueComments, reviewComments...)
	events = append(events, reviews...)
	sort.SliceStable(events, func(i, j int) bool {
		left := eventTime(events[i])
		right := eventTime(events[j])
		if left == right {
			if events[i].Type == events[j].Type {
				return events[i].ID < events[j].ID
			}
			return events[i].Type < events[j].Type
		}
		return left < right
	})
	return events, nil
}

func (c *Client) issueComments(ctx context.Context, prNumber int, viewer string, includeSelf bool) ([]domain.Event, error) {
	endpoint := fmt.Sprintf("repos/{owner}/{repo}/issues/%d/comments", prNumber)
	out, err := c.run(ctx, "api", endpoint, "--paginate", "--slurp")
	if err != nil {
		return nil, err
	}

	var raw []issueComment
	if err := decodePaginatedArray(out, &raw); err != nil {
		return nil, err
	}

	events := make([]domain.Event, 0, len(raw))
	for _, comment := range raw {
		if !includeSelf && comment.User.Login == viewer {
			continue
		}
		event := domain.Event{
			Key:       domain.EventKey("issue_comment", comment.ID),
			Type:      "issue_comment",
			ID:        comment.ID,
			Author:    comment.User.Login,
			CreatedAt: comment.CreatedAt,
			UpdatedAt: comment.UpdatedAt,
			Body:      comment.Body,
			URL:       comment.HTMLURL,
		}
		event, err = domain.WithFingerprint(event)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func (c *Client) reviewComments(ctx context.Context, prNumber int, viewer string, includeSelf bool) ([]domain.Event, error) {
	endpoint := fmt.Sprintf("repos/{owner}/{repo}/pulls/%d/comments", prNumber)
	out, err := c.run(ctx, "api", endpoint, "--paginate", "--slurp")
	if err != nil {
		return nil, err
	}

	var raw []reviewComment
	if err := decodePaginatedArray(out, &raw); err != nil {
		return nil, err
	}

	events := make([]domain.Event, 0, len(raw))
	for _, comment := range raw {
		if !includeSelf && comment.User.Login == viewer {
			continue
		}
		event := domain.Event{
			Key:       domain.EventKey("review_comment", comment.ID),
			Type:      "review_comment",
			ID:        comment.ID,
			Author:    comment.User.Login,
			CreatedAt: comment.CreatedAt,
			UpdatedAt: comment.UpdatedAt,
			Body:      comment.Body,
			URL:       comment.HTMLURL,
			Location: &domain.Location{
				Path:              comment.Path,
				Line:              comment.Line,
				StartLine:         comment.StartLine,
				Side:              comment.Side,
				StartSide:         comment.StartSide,
				DiffHunk:          comment.DiffHunk,
				CommitID:          comment.CommitID,
				OriginalCommitID:  comment.OriginalCommitID,
				OriginalLine:      comment.OriginalLine,
				OriginalStartLine: comment.OriginalStartLine,
			},
		}
		event, err = domain.WithFingerprint(event)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func (c *Client) reviews(ctx context.Context, prNumber int, viewer string, includeSelf bool) ([]domain.Event, error) {
	endpoint := fmt.Sprintf("repos/{owner}/{repo}/pulls/%d/reviews", prNumber)
	out, err := c.run(ctx, "api", endpoint, "--paginate", "--slurp")
	if err != nil {
		return nil, err
	}

	var raw []review
	if err := decodePaginatedArray(out, &raw); err != nil {
		return nil, err
	}

	events := make([]domain.Event, 0, len(raw))
	for _, review := range raw {
		if !includeSelf && review.User.Login == viewer {
			continue
		}
		updatedAt := review.UpdatedAt
		if updatedAt == "" {
			updatedAt = review.SubmittedAt
		}
		event := domain.Event{
			Key:       domain.EventKey("review", review.ID),
			Type:      "review",
			ID:        review.ID,
			Author:    review.User.Login,
			State:     review.State,
			CreatedAt: review.SubmittedAt,
			UpdatedAt: updatedAt,
			Body:      review.Body,
			URL:       review.HTMLURL,
			CommitID:  review.CommitID,
		}
		event, err = domain.WithFingerprint(event)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func (c *Client) run(ctx context.Context, args ...string) ([]byte, error) {
	return c.Runner.Run(ctx, args...)
}

func classifyPreflightError(err error) error {
	var cmdErr *CommandError
	if errors.As(err, &cmdErr) {
		text := cmdErr.CombinedOutput()
		if isMissingGH(err) {
			return &PreflightError{Kind: PreflightMissingGH, Detail: cmdErr.Error()}
		}
		if isNetworkText(text) {
			return &PreflightError{Kind: PreflightNetwork, Detail: text}
		}
		if isAuthText(text) {
			return &PreflightError{Kind: PreflightAuth, Detail: text}
		}
		return &PreflightError{Kind: PreflightUnknown, Detail: text}
	}
	if isMissingGH(err) {
		return &PreflightError{Kind: PreflightMissingGH, Detail: err.Error()}
	}
	return &PreflightError{Kind: PreflightUnknown, Detail: err.Error()}
}

func isMissingGH(err error) bool {
	return errors.Is(err, exec.ErrNotFound) || strings.Contains(strings.ToLower(err.Error()), "executable file not found")
}

func isNetworkText(text string) bool {
	lower := strings.ToLower(text)
	needles := []string{
		"no such host",
		"could not resolve host",
		"temporary failure in name resolution",
		"network is unreachable",
		"dial tcp",
		"lookup api.github.com",
	}
	for _, needle := range needles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func isAuthText(text string) bool {
	lower := strings.ToLower(text)
	needles := []string{
		"not logged in",
		"failed to log in",
		"token",
		"authentication",
		"unauthorized",
		"invalid",
	}
	for _, needle := range needles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func decodePaginatedArray[T any](data []byte, dest *[]T) error {
	var outer []json.RawMessage
	if err := json.Unmarshal(data, &outer); err != nil {
		return err
	}

	var out []T
	for _, raw := range outer {
		var page []T
		if err := json.Unmarshal(raw, &page); err == nil {
			out = append(out, page...)
			continue
		}

		var item T
		if err := json.Unmarshal(raw, &item); err != nil {
			return err
		}
		out = append(out, item)
	}
	*dest = out
	return nil
}

func eventTime(event domain.Event) string {
	if event.CreatedAt != "" {
		return event.CreatedAt
	}
	return event.UpdatedAt
}

type authStatus struct {
	Hosts map[string][]authAccount `json:"hosts"`
}

type authAccount struct {
	State       string `json:"state"`
	Error       string `json:"error"`
	Active      bool   `json:"active"`
	Host        string `json:"host"`
	Login       string `json:"login"`
	TokenSource string `json:"tokenSource"`
	GitProtocol string `json:"gitProtocol"`
}

type user struct {
	Login string `json:"login"`
}

type issueComment struct {
	ID        int64  `json:"id"`
	User      user   `json:"user"`
	Body      string `json:"body"`
	HTMLURL   string `json:"html_url"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type reviewRequestPR struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Author    user   `json:"author"`
	UpdatedAt string `json:"updatedAt"`
	IsDraft   bool   `json:"isDraft"`
}

type reviewComment struct {
	ID                int64  `json:"id"`
	User              user   `json:"user"`
	Body              string `json:"body"`
	HTMLURL           string `json:"html_url"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
	Path              string `json:"path"`
	Line              *int   `json:"line"`
	StartLine         *int   `json:"start_line"`
	Side              string `json:"side"`
	StartSide         string `json:"start_side"`
	DiffHunk          string `json:"diff_hunk"`
	CommitID          string `json:"commit_id"`
	OriginalCommitID  string `json:"original_commit_id"`
	OriginalLine      *int   `json:"original_line"`
	OriginalStartLine *int   `json:"original_start_line"`
}

type review struct {
	ID          int64  `json:"id"`
	User        user   `json:"user"`
	Body        string `json:"body"`
	State       string `json:"state"`
	HTMLURL     string `json:"html_url"`
	SubmittedAt string `json:"submitted_at"`
	UpdatedAt   string `json:"updated_at"`
	CommitID    string `json:"commit_id"`
}
