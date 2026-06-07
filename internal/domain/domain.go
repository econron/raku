package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

type PR struct {
	Number     int    `json:"number"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	Author     string `json:"author"`
	HeadRef    string `json:"head_ref"`
	BaseRef    string `json:"base_ref"`
	HeadRefOID string `json:"head_ref_oid,omitempty"`
}

type Location struct {
	Path              string `json:"path"`
	Line              *int   `json:"line"`
	StartLine         *int   `json:"start_line"`
	Side              string `json:"side"`
	StartSide         string `json:"start_side"`
	DiffHunk          string `json:"diff_hunk"`
	CommitID          string `json:"commit_id,omitempty"`
	OriginalCommitID  string `json:"original_commit_id,omitempty"`
	OriginalLine      *int   `json:"original_line,omitempty"`
	OriginalStartLine *int   `json:"original_start_line,omitempty"`
}

type Event struct {
	Alias       int       `json:"alias,omitempty"`
	Status      string    `json:"status,omitempty"`
	Key         string    `json:"key"`
	Type        string    `json:"type"`
	ID          int64     `json:"id"`
	Author      string    `json:"author"`
	State       string    `json:"state,omitempty"`
	CreatedAt   string    `json:"created_at,omitempty"`
	UpdatedAt   string    `json:"updated_at,omitempty"`
	Body        string    `json:"body"`
	URL         string    `json:"url,omitempty"`
	CommitID    string    `json:"commit_id,omitempty"`
	Location    *Location `json:"location,omitempty"`
	Fingerprint string    `json:"-"`
}

type Counts struct {
	IssueComments  int `json:"issue_comments"`
	ReviewComments int `json:"review_comments"`
	Reviews        int `json:"reviews"`
	Total          int `json:"total"`
}

type Result struct {
	Repo          string  `json:"repo"`
	PR            PR      `json:"pr"`
	Mode          string  `json:"mode"`
	Uninitialized bool    `json:"uninitialized,omitempty"`
	Counts        *Counts `json:"counts,omitempty"`
	Events        []Event `json:"events"`
}

type ReviewRequest struct {
	Repo      string `json:"repo"`
	Number    int    `json:"number"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Author    string `json:"author"`
	UpdatedAt string `json:"updated_at"`
	IsDraft   bool   `json:"is_draft"`
}

func (r ReviewRequest) Key() string {
	return fmt.Sprintf("%s#%d", r.Repo, r.Number)
}

func EventKey(eventType string, id int64) string {
	return fmt.Sprintf("%s:%d", eventType, id)
}

func Fingerprint(event Event) (string, error) {
	var payload any

	switch event.Type {
	case "issue_comment":
		payload = struct {
			Type   string `json:"type"`
			ID     int64  `json:"id"`
			Author string `json:"author"`
			Body   string `json:"body"`
		}{
			Type:   event.Type,
			ID:     event.ID,
			Author: event.Author,
			Body:   event.Body,
		}
	case "review_comment":
		location := Location{}
		if event.Location != nil {
			location = *event.Location
		}
		payload = struct {
			Type      string `json:"type"`
			ID        int64  `json:"id"`
			Author    string `json:"author"`
			Body      string `json:"body"`
			Path      string `json:"path"`
			Line      *int   `json:"line"`
			StartLine *int   `json:"start_line"`
			Side      string `json:"side"`
			StartSide string `json:"start_side"`
			DiffHunk  string `json:"diff_hunk"`
		}{
			Type:      event.Type,
			ID:        event.ID,
			Author:    event.Author,
			Body:      event.Body,
			Path:      location.Path,
			Line:      location.Line,
			StartLine: location.StartLine,
			Side:      location.Side,
			StartSide: location.StartSide,
			DiffHunk:  location.DiffHunk,
		}
	case "review":
		payload = struct {
			Type   string `json:"type"`
			ID     int64  `json:"id"`
			Author string `json:"author"`
			State  string `json:"state"`
			Body   string `json:"body"`
		}{
			Type:   event.Type,
			ID:     event.ID,
			Author: event.Author,
			State:  event.State,
			Body:   event.Body,
		}
	default:
		return "", fmt.Errorf("unsupported event type %q", event.Type)
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func WithFingerprint(event Event) (Event, error) {
	fp, err := Fingerprint(event)
	if err != nil {
		return Event{}, err
	}
	event.Fingerprint = fp
	return event, nil
}

func CountEvents(events []Event) Counts {
	counts := Counts{Total: len(events)}
	for _, event := range events {
		switch event.Type {
		case "issue_comment":
			counts.IssueComments++
		case "review_comment":
			counts.ReviewComments++
		case "review":
			counts.Reviews++
		}
	}
	return counts
}
