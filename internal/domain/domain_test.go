package domain

import "testing"

func TestFingerprintIgnoresUpdatedAt(t *testing.T) {
	t.Parallel()

	first := Event{
		Key:       EventKey("issue_comment", 1),
		Type:      "issue_comment",
		ID:        1,
		Author:    "reviewer",
		CreatedAt: "2026-06-06T10:00:00Z",
		UpdatedAt: "2026-06-06T10:00:00Z",
		Body:      "same",
	}
	second := first
	second.UpdatedAt = "2026-06-06T11:00:00Z"

	firstFP, err := Fingerprint(first)
	if err != nil {
		t.Fatalf("Fingerprint returned error: %v", err)
	}
	secondFP, err := Fingerprint(second)
	if err != nil {
		t.Fatalf("Fingerprint returned error: %v", err)
	}
	if firstFP != secondFP {
		t.Fatalf("fingerprint changed for updated_at only: %q != %q", firstFP, secondFP)
	}
}

func TestFingerprintChangesWhenBodyChanges(t *testing.T) {
	t.Parallel()

	first := Event{
		Key:    EventKey("review", 1),
		Type:   "review",
		ID:     1,
		Author: "reviewer",
		State:  "COMMENTED",
		Body:   "old",
	}
	second := first
	second.Body = "new"

	firstFP, err := Fingerprint(first)
	if err != nil {
		t.Fatalf("Fingerprint returned error: %v", err)
	}
	secondFP, err := Fingerprint(second)
	if err != nil {
		t.Fatalf("Fingerprint returned error: %v", err)
	}
	if firstFP == secondFP {
		t.Fatal("fingerprint did not change when body changed")
	}
}

func TestReviewCommentFingerprintIncludesLocation(t *testing.T) {
	t.Parallel()

	line := 10
	first := Event{
		Key:    EventKey("review_comment", 1),
		Type:   "review_comment",
		ID:     1,
		Author: "reviewer",
		Body:   "comment",
		Location: &Location{
			Path: "a.go",
			Line: &line,
			Side: "RIGHT",
		},
	}
	otherLine := 11
	second := first
	second.Location = &Location{
		Path: "a.go",
		Line: &otherLine,
		Side: "RIGHT",
	}

	firstFP, err := Fingerprint(first)
	if err != nil {
		t.Fatalf("Fingerprint returned error: %v", err)
	}
	secondFP, err := Fingerprint(second)
	if err != nil {
		t.Fatalf("Fingerprint returned error: %v", err)
	}
	if firstFP == secondFP {
		t.Fatal("fingerprint did not change when review comment location changed")
	}
}

func TestFingerprintIgnoresCommitMetadata(t *testing.T) {
	t.Parallel()

	line := 10
	originalLine := 9
	first := Event{
		Key:    EventKey("review_comment", 1),
		Type:   "review_comment",
		ID:     1,
		Author: "reviewer",
		Body:   "comment",
		Location: &Location{
			Path:              "a.go",
			Line:              &line,
			OriginalLine:      &originalLine,
			CommitID:          "1111111111111111111111111111111111111111",
			OriginalCommitID:  "2222222222222222222222222222222222222222",
			OriginalStartLine: &originalLine,
		},
	}
	second := first
	second.Location = &Location{
		Path:              "a.go",
		Line:              &line,
		OriginalLine:      &originalLine,
		CommitID:          "3333333333333333333333333333333333333333",
		OriginalCommitID:  "4444444444444444444444444444444444444444",
		OriginalStartLine: &originalLine,
	}

	firstFP, err := Fingerprint(first)
	if err != nil {
		t.Fatalf("Fingerprint returned error: %v", err)
	}
	secondFP, err := Fingerprint(second)
	if err != nil {
		t.Fatalf("Fingerprint returned error: %v", err)
	}
	if firstFP != secondFP {
		t.Fatalf("fingerprint changed for commit metadata only: %q != %q", firstFP, secondFP)
	}

	review := Event{
		Key:      EventKey("review", 2),
		Type:     "review",
		ID:       2,
		Author:   "reviewer",
		State:    "COMMENTED",
		Body:     "review",
		CommitID: "1111111111111111111111111111111111111111",
	}
	otherReview := review
	otherReview.CommitID = "2222222222222222222222222222222222222222"

	firstFP, err = Fingerprint(review)
	if err != nil {
		t.Fatalf("Fingerprint returned error: %v", err)
	}
	secondFP, err = Fingerprint(otherReview)
	if err != nil {
		t.Fatalf("Fingerprint returned error: %v", err)
	}
	if firstFP != secondFP {
		t.Fatalf("review fingerprint changed for commit metadata only: %q != %q", firstFP, secondFP)
	}
}
