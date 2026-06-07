package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"raku/internal/app"
	"raku/internal/domain"
)

func WriteGetJSON(w io.Writer, result domain.Result) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func WriteGetText(w io.Writer, result domain.Result) {
	if result.Uninitialized {
		writeUninitialized(w, result)
		return
	}

	fmt.Fprintf(w, "PR #%d %s\n", result.PR.Number, result.PR.Title)
	if result.PR.URL != "" {
		fmt.Fprintln(w, result.PR.URL)
	}
	fmt.Fprintln(w)

	if len(result.Events) == 0 {
		if result.Mode == app.ModeAll {
			fmt.Fprintln(w, "No PR comments found.")
		} else {
			fmt.Fprintln(w, "No new or updated PR comments.")
		}
		return
	}

	for i, event := range result.Events {
		if i > 0 {
			fmt.Fprintln(w)
		}
		writeEvent(w, event)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Tip: aliases refer to the latest raku get pr-comment output.")
	fmt.Fprintln(w, "     Mark handled items with: raku seen pr-comment 1")
}

func WriteSeenText(w io.Writer, result app.SeenResult) {
	if result.Baseline {
		fmt.Fprintf(w, "Marked %d current PR comment(s) as seen baseline for %s.\n", result.Marked, result.PRKey)
		return
	}
	if result.All {
		fmt.Fprintf(w, "Marked all %d item(s) from current_view as seen for %s.\n", result.Marked, result.PRKey)
		return
	}
	fmt.Fprintf(w, "Marked %d item(s) as seen for %s.\n", result.Marked, result.PRKey)
}

func writeUninitialized(w io.Writer, result domain.Result) {
	fmt.Fprintf(w, "No raku state for %s#%d.\n\n", result.Repo, result.PR.Number)
	if result.Counts != nil {
		fmt.Fprintln(w, "Current PR has:")
		fmt.Fprintf(w, "- %d conversation comments\n", result.Counts.IssueComments)
		fmt.Fprintf(w, "- %d review line comments\n", result.Counts.ReviewComments)
		fmt.Fprintf(w, "- %d reviews\n\n", result.Counts.Reviews)
	}
	fmt.Fprintln(w, "Use:")
	fmt.Fprintln(w, "  raku get pr-comment --all")
	fmt.Fprintln(w, "    show all comments")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  raku seen pr-comment --baseline")
	fmt.Fprintln(w, "    mark current comments as seen baseline")
}

func writeEvent(w io.Writer, event domain.Event) {
	fmt.Fprintf(w, "[%d] %s %s by %s\n", event.Alias, event.Status, event.Type, event.Author)

	switch event.Type {
	case "review_comment":
		if event.Location != nil {
			fmt.Fprintf(w, "    file: %s\n", formatLocation(event.Location))
			if event.Location.Side != "" {
				fmt.Fprintf(w, "    side: %s\n", event.Location.Side)
			}
			if event.Location.CommitID != "" {
				fmt.Fprintf(w, "    commit: %s\n", shortSHA(event.Location.CommitID))
			}
			if event.Location.OriginalCommitID != "" {
				fmt.Fprintf(w, "    original_commit: %s\n", shortSHA(event.Location.OriginalCommitID))
			}
			if original := formatOriginalLocation(event.Location); original != "" {
				fmt.Fprintf(w, "    original_file: %s\n", original)
			}
		}
	case "issue_comment":
		fmt.Fprintln(w, "    location: Conversation")
	case "review":
		fmt.Fprintln(w, "    location: Review")
		if event.State != "" {
			fmt.Fprintf(w, "    state: %s\n", event.State)
		}
		if event.CommitID != "" {
			fmt.Fprintf(w, "    commit: %s\n", shortSHA(event.CommitID))
		}
	}
	if event.URL != "" {
		fmt.Fprintf(w, "    url: %s\n", event.URL)
	}

	fmt.Fprintln(w)
	if strings.TrimSpace(event.Body) == "" {
		fmt.Fprintln(w, "    (no body)")
		return
	}
	for _, line := range strings.Split(event.Body, "\n") {
		fmt.Fprintf(w, "    %s\n", line)
	}
}

func formatLocation(location *domain.Location) string {
	if location == nil {
		return ""
	}
	if location.Line != nil && location.StartLine != nil && *location.StartLine != *location.Line {
		return fmt.Sprintf("%s:%d-%d", location.Path, *location.StartLine, *location.Line)
	}
	if location.Line != nil {
		return fmt.Sprintf("%s:%d", location.Path, *location.Line)
	}
	if location.StartLine != nil {
		return fmt.Sprintf("%s:%d", location.Path, *location.StartLine)
	}
	return location.Path
}

func formatOriginalLocation(location *domain.Location) string {
	if location == nil {
		return ""
	}
	if location.OriginalLine != nil && location.OriginalStartLine != nil && *location.OriginalStartLine != *location.OriginalLine {
		return fmt.Sprintf("%s:%d-%d", location.Path, *location.OriginalStartLine, *location.OriginalLine)
	}
	if location.OriginalLine != nil {
		return fmt.Sprintf("%s:%d", location.Path, *location.OriginalLine)
	}
	if location.OriginalStartLine != nil {
		return fmt.Sprintf("%s:%d", location.Path, *location.OriginalStartLine)
	}
	return ""
}

func shortSHA(sha string) string {
	if len(sha) <= 12 {
		return sha
	}
	return sha[:12]
}
