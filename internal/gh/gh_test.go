package gh

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestPreflightSuccess(t *testing.T) {
	t.Parallel()

	client := NewClient(fakeRunner{
		"auth status --json hosts": []byte(`{"hosts":{"github.com":[{"state":"success","active":true,"login":"me"}]}}`),
	})
	if err := client.Preflight(context.Background()); err != nil {
		t.Fatalf("Preflight returned error: %v", err)
	}
}

func TestPreflightNetworkError(t *testing.T) {
	t.Parallel()

	client := NewClient(fakeRunner{
		"auth status --json hosts": []byte(`{"hosts":{"github.com":[{"state":"error","active":true,"login":"me","error":"Get \"https://api.github.com/\": dial tcp: lookup api.github.com: no such host"}]}}`),
	})
	err := client.Preflight(context.Background())
	var preflight *PreflightError
	if !errors.As(err, &preflight) {
		t.Fatalf("got %T, want PreflightError", err)
	}
	if preflight.Kind != PreflightNetwork {
		t.Fatalf("kind %q, want %q", preflight.Kind, PreflightNetwork)
	}
}

func TestPreflightAuthError(t *testing.T) {
	t.Parallel()

	client := NewClient(fakeRunner{
		"auth status --json hosts": []byte(`{"hosts":{"github.com":[{"state":"error","active":true,"login":"me","error":"token is invalid"}]}}`),
	})
	err := client.Preflight(context.Background())
	var preflight *PreflightError
	if !errors.As(err, &preflight) {
		t.Fatalf("got %T, want PreflightError", err)
	}
	if preflight.Kind != PreflightAuth {
		t.Fatalf("kind %q, want %q", preflight.Kind, PreflightAuth)
	}
}

func TestEventsNormalizesAndExcludesViewer(t *testing.T) {
	t.Parallel()

	client := NewClient(eventsRunner())

	events, err := client.Events(context.Background(), 123, "me", false)
	if err != nil {
		t.Fatalf("Events returned error: %v", err)
	}
	keys := []string{}
	for _, event := range events {
		keys = append(keys, event.Key)
		if event.Fingerprint == "" {
			t.Fatalf("event has empty fingerprint: %#v", event)
		}
	}
	want := []string{"issue_comment:1", "review_comment:3", "review:4"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys %v, want %v", keys, want)
	}
	if events[1].Location == nil || events[1].Location.Line == nil || *events[1].Location.Line != 87 {
		t.Fatalf("review comment location not normalized: %#v", events[1].Location)
	}
}

func TestEventsIncludesViewerWhenRequested(t *testing.T) {
	t.Parallel()

	client := NewClient(eventsRunner())
	events, err := client.Events(context.Background(), 123, "me", true)
	if err != nil {
		t.Fatalf("Events returned error: %v", err)
	}
	keys := []string{}
	for _, event := range events {
		keys = append(keys, event.Key)
	}
	want := []string{
		"issue_comment:1",
		"issue_comment:2",
		"review_comment:3",
		"review_comment:5",
		"review:4",
		"review:6",
	}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys %v, want %v", keys, want)
	}
}

func TestDecodePaginatedArray(t *testing.T) {
	t.Parallel()

	var got []struct {
		ID int `json:"id"`
	}
	if err := decodePaginatedArray([]byte(`[[{"id":1}],[{"id":2}]]`), &got); err != nil {
		t.Fatalf("decodePaginatedArray returned error: %v", err)
	}
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 2 {
		t.Fatalf("got %#v", got)
	}
}

func eventsRunner() fakeRunner {
	return fakeRunner{
		"api repos/{owner}/{repo}/issues/123/comments --paginate --slurp": []byte(`[[{"id":1,"user":{"login":"reviewer"},"body":"conversation","html_url":"https://example.com/issue","created_at":"2026-06-06T10:00:00Z","updated_at":"2026-06-06T10:00:00Z"},{"id":2,"user":{"login":"me"},"body":"mine","html_url":"https://example.com/mine","created_at":"2026-06-06T10:01:00Z","updated_at":"2026-06-06T10:01:00Z"}]]`),
		"api repos/{owner}/{repo}/pulls/123/comments --paginate --slurp":  []byte(`[[{"id":3,"user":{"login":"reviewer"},"body":"line","html_url":"https://example.com/review-comment","created_at":"2026-06-06T10:02:00Z","updated_at":"2026-06-06T10:02:00Z","path":"internal/user/profile.go","line":87,"side":"RIGHT","diff_hunk":"@@ -1 +1 @@"},{"id":5,"user":{"login":"me"},"body":"my line","html_url":"https://example.com/my-review-comment","created_at":"2026-06-06T10:03:00Z","updated_at":"2026-06-06T10:03:00Z","path":"internal/user/profile.go","line":88,"side":"RIGHT","diff_hunk":"@@ -1 +1 @@"}]]`),
		"api repos/{owner}/{repo}/pulls/123/reviews --paginate --slurp":   []byte(`[[{"id":4,"user":{"login":"reviewer"},"body":"review","state":"COMMENTED","html_url":"https://example.com/review","submitted_at":"2026-06-06T10:04:00Z"},{"id":6,"user":{"login":"me"},"body":"my review","state":"COMMENTED","html_url":"https://example.com/my-review","submitted_at":"2026-06-06T10:05:00Z"}]]`),
	}
}

type fakeRunner map[string][]byte

func (r fakeRunner) Run(_ context.Context, args ...string) ([]byte, error) {
	key := strings.Join(args, " ")
	out, ok := r[key]
	if !ok {
		return nil, errors.New("unexpected gh command: " + key)
	}
	return out, nil
}
