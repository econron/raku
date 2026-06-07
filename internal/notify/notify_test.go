package notify

import (
	"bytes"
	"strings"
	"testing"

	"raku/internal/domain"
)

func TestFallbackNotifierWritesReviewRequest(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	notifier := ReviewRequestNotifier{Writer: &buf}
	err := notifier.notifyFallback(domain.ReviewRequest{
		Repo:   "owner/repo",
		Number: 12,
		Title:  "Need review",
		URL:    "https://github.com/owner/repo/pull/12",
	})
	if err != nil {
		t.Fatalf("notifyFallback returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "owner/repo#12") || !strings.Contains(out, "Need review") {
		t.Fatalf("unexpected fallback output: %q", out)
	}
}

func TestEscapeAppleScript(t *testing.T) {
	t.Parallel()

	got := escapeAppleScript(`quote " and slash \`)
	want := `quote \" and slash \\`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFallbackNotifierWithNilWriter(t *testing.T) {
	t.Parallel()

	notifier := ReviewRequestNotifier{}
	if err := notifier.notifyFallback(domain.ReviewRequest{}); err != nil {
		t.Fatalf("notifyFallback returned error: %v", err)
	}
}
