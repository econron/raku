package output

import (
	"bytes"
	"strings"
	"testing"

	"raku/internal/app"
	"raku/internal/domain"
)

func TestWriteGetTextShowsCommitMetadata(t *testing.T) {
	t.Parallel()

	line := 20
	originalLine := 18
	var buf bytes.Buffer
	WriteGetText(&buf, domain.Result{
		Repo: "owner/repo",
		PR: domain.PR{
			Number: 123,
			Title:  "Test PR",
			URL:    "https://github.com/owner/repo/pull/123",
		},
		Mode: app.ModeUnread,
		Events: []domain.Event{
			{
				Alias:  1,
				Status: app.StatusNew,
				Type:   "review_comment",
				Author: "reviewer",
				Body:   "line comment",
				URL:    "https://github.com/owner/repo/pull/123#discussion",
				Location: &domain.Location{
					Path:             "Makefile",
					Line:             &line,
					Side:             "RIGHT",
					CommitID:         "1111111111111111111111111111111111111111",
					OriginalCommitID: "2222222222222222222222222222222222222222",
					OriginalLine:     &originalLine,
				},
			},
			{
				Alias:    2,
				Status:   app.StatusNew,
				Type:     "review",
				Author:   "reviewer",
				State:    "COMMENTED",
				Body:     "review body",
				CommitID: "3333333333333333333333333333333333333333",
			},
		},
	})

	out := buf.String()
	wants := []string{
		"file: Makefile:20",
		"commit: 111111111111",
		"original_commit: 222222222222",
		"original_file: Makefile:18",
		"location: Review",
		"commit: 333333333333",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("output does not contain %q:\n%s", want, out)
		}
	}
}
