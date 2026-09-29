package fmedit

import (
	"strings"
	"testing"
)

const frontmatterWithComments = `# a leading comment about the whole file
title: Original Title   # the author's title, with its comment
tags: [rust, notes]     # order matters to the author
custom_field: keep me exactly
date: 2026-09-01
`

// TestEditRecognisedFieldPreservesTheRest verifies editing one field leaves
// unrecognised fields, comments, and key order intact (task 8.5).
func TestEditRecognisedFieldPreservesTheRest(t *testing.T) {
	out, err := Update([]byte(frontmatterWithComments), "title", "New Title")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	got := string(out)

	if !strings.Contains(got, "title: New Title") {
		t.Errorf("title not updated:\n%s", got)
	}
	for _, want := range []string{
		"# a leading comment about the whole file",
		"# the author's title, with its comment",
		"custom_field: keep me exactly",
		"tags: [rust, notes]",
		"date: 2026-09-01",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("edit lost %q:\n%s", want, got)
		}
	}
	// Order preserved: title before tags before custom_field before date.
	idx := func(sub string) int { return strings.Index(got, sub) }
	if !(idx("title:") < idx("tags:") && idx("tags:") < idx("custom_field:") && idx("custom_field:") < idx("date:")) {
		t.Errorf("key order disturbed:\n%s", got)
	}
}

// TestEditAbsentFieldAppendsAtEnd verifies a new field is appended rather
// than the document reordered.
func TestEditAbsentFieldAppendsAtEnd(t *testing.T) {
	out, err := Update([]byte(frontmatterWithComments), "description", "Added later")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "description: Added later") {
		t.Errorf("field not added:\n%s", got)
	}
	if strings.Index(got, "description:") < strings.Index(got, "date:") {
		t.Errorf("new field should append at end:\n%s", got)
	}
}

// TestChangedDetectsOnlyDifferences verifies the write-only-what-changed
// gate (task 8.7).
func TestChangedDetectsOnlyDifferences(t *testing.T) {
	changed := Changed([]byte(frontmatterWithComments), map[string]any{
		"title":       "Original Title", // same value: not changed
		"description": "new",            // absent: changed
	})
	if _, ok := changed["title"]; ok {
		t.Errorf("unchanged field reported changed: %+v", changed)
	}
	if _, ok := changed["description"]; !ok {
		t.Errorf("new field not reported changed: %+v", changed)
	}
}
