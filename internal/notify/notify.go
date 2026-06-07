package notify

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"

	"raku/internal/domain"
)

type ReviewRequestNotifier struct {
	Writer io.Writer
}

const reviewRequestVoiceMessage = "PRレビューのリクエストが来てます"

var runCommand = func(ctx context.Context, name string, args ...string) error {
	return exec.CommandContext(ctx, name, args...).Run()
}

func (n ReviewRequestNotifier) NotifyReviewRequest(ctx context.Context, request domain.ReviewRequest) error {
	if runtime.GOOS == "darwin" {
		if _, err := exec.LookPath("osascript"); err == nil {
			return notifyDarwin(ctx, request)
		}
	}
	return n.notifyFallback(request)
}

func notifyDarwin(ctx context.Context, request domain.ReviewRequest) error {
	title := "raku"
	subtitle := fmt.Sprintf("%s#%d", request.Repo, request.Number)
	message := request.Title
	if request.Author != "" {
		message = fmt.Sprintf("%s by %s", request.Title, request.Author)
	}

	script := fmt.Sprintf(
		`display notification "%s" with title "%s" subtitle "%s"`,
		escapeAppleScript(message),
		escapeAppleScript(title),
		escapeAppleScript(subtitle),
	)
	if err := runCommand(ctx, "osascript", "-e", script); err != nil {
		return err
	}
	_ = speakReviewRequest(ctx)
	return nil
}

func speakReviewRequest(ctx context.Context) error {
	return runCommand(ctx, "say", reviewRequestVoiceMessage)
}

func (n ReviewRequestNotifier) notifyFallback(request domain.ReviewRequest) error {
	if n.Writer == nil {
		return nil
	}
	_, err := fmt.Fprintf(
		n.Writer,
		"review requested: %s#%d %s %s\n",
		request.Repo,
		request.Number,
		request.Title,
		request.URL,
	)
	return err
}

func escapeAppleScript(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}
