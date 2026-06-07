package notify

import (
	"bytes"
	"context"
	"errors"
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

func TestNotifyDarwinSpeaksReviewRequest(t *testing.T) {
	var calls []string
	original := runCommand
	runCommand = func(_ context.Context, name string, args ...string) error {
		calls = append(calls, strings.Join(append([]string{name}, args...), " "))
		return nil
	}
	defer func() { runCommand = original }()

	err := notifyDarwin(context.Background(), domain.ReviewRequest{
		Repo:   "owner/repo",
		Number: 12,
		Title:  "Need review",
		Author: "octocat",
	})
	if err != nil {
		t.Fatalf("notifyDarwin returned error: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("command calls %v, want 2 calls", calls)
	}
	if !strings.HasPrefix(calls[0], "osascript -e display notification") {
		t.Fatalf("first call %q, want osascript notification", calls[0])
	}
	if calls[1] != "say "+reviewRequestVoiceMessage {
		t.Fatalf("second call %q, want say message", calls[1])
	}
}

func TestNotifyDarwinIgnoresSpeakError(t *testing.T) {
	original := runCommand
	runCommand = func(_ context.Context, name string, _ ...string) error {
		if name == "say" {
			return errors.New("say failed")
		}
		return nil
	}
	defer func() { runCommand = original }()

	if err := notifyDarwin(context.Background(), domain.ReviewRequest{}); err != nil {
		t.Fatalf("notifyDarwin returned error: %v", err)
	}
}

func TestFallbackNotifierWithNilWriter(t *testing.T) {
	t.Parallel()

	notifier := ReviewRequestNotifier{}
	if err := notifier.notifyFallback(domain.ReviewRequest{}); err != nil {
		t.Fatalf("notifyFallback returned error: %v", err)
	}
}
