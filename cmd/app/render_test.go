package app

import (
	"strings"
	"testing"

	"github.com/amplia/jira8/internal/models"
)

func TestTrimHistory(t *testing.T) {
	long := strings.Repeat("é", 250)
	issue := &models.Issue{Changelog: &models.Changelog{Histories: []models.History{{
		Items: []models.HistoryItem{
			{Field: "status", FromString: "Open", ToString: "In Progress"},
			{Field: "description", FromString: "h1. Title\nbody", ToString: long},
			{Field: "summary", FromString: "", ToString: "line\r\nmore"},
		},
	}}}}

	TrimHistory(issue, 200)
	items := issue.Changelog.Histories[0].Items

	if items[0].FromString != "Open" || items[0].ToString != "In Progress" {
		t.Errorf("short values must be untouched: %+v", items[0])
	}
	if got, want := items[1].FromString, "h1. Title… (14 chars)"; got != want {
		t.Errorf("multi-line: got %q, want %q", got, want)
	}
	if got, want := items[1].ToString, strings.Repeat("é", 200)+"… (250 chars)"; got != want {
		t.Errorf("long: got %q, want %q", got, want)
	}
	if got, want := items[2].ToString, "line… (10 chars)"; got != want {
		t.Errorf("CRLF: got %q, want %q", got, want)
	}
	if items[2].FromString != "" {
		t.Errorf("empty must stay empty, got %q", items[2].FromString)
	}

	TrimHistory(nil, 200)
	TrimHistory(&models.Issue{}, 200)
}
