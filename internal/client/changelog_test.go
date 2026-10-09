package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amplia/jira8/internal/config"
)

func TestGetIssueWithChangelog(t *testing.T) {
	var expand string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expand = r.URL.Query().Get("expand")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"key":    "OUW-1",
			"fields": map[string]any{"summary": "x"},
			"changelog": map[string]any{"total": 1, "histories": []map[string]any{{
				"id": "1", "author": map[string]any{"displayName": "Ana"}, "created": "2026-10-09T08:40:01.000+0200",
				"items": []map[string]any{{"field": "status", "fromString": "Open", "toString": "In Progress"}},
			}}},
		})
	}))
	defer srv.Close()

	c := New(&config.Config{URL: srv.URL, Token: "secret"})
	issue, err := c.GetIssueWithChangelog(context.Background(), "OUW-1")
	if err != nil {
		t.Fatalf("GetIssueWithChangelog: %v", err)
	}
	if expand != "changelog" {
		t.Errorf("expand = %q, want changelog", expand)
	}
	h := issue.Changelog.Histories[0]
	if h.Author.DisplayName != "Ana" || h.Items[0].ToString != "In Progress" {
		t.Errorf("unexpected history: %+v", h)
	}

	if _, err := c.GetIssue(context.Background(), "OUW-1"); err != nil || expand != "" {
		t.Errorf("GetIssue must not expand: expand=%q err=%v", expand, err)
	}
}
