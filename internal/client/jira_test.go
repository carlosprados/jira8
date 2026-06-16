package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amplia/jira8/internal/config"
)

func TestBuildJQLWith(t *testing.T) {
	tests := []struct {
		name    string
		filters JQLFilters
		want    string
	}{
		{
			name:    "empty",
			filters: JQLFilters{},
			want:    "ORDER BY updated DESC",
		},
		{
			name:    "project only",
			filters: JQLFilters{Project: "ESA"},
			want:    "project = ESA ORDER BY updated DESC",
		},
		{
			name:    "assignee me",
			filters: JQLFilters{Assignee: "me"},
			want:    "assignee = currentUser() ORDER BY updated DESC",
		},
		{
			name:    "epic filter",
			filters: JQLFilters{Epic: "ESA-42"},
			want:    `"Epic Link" = ESA-42 ORDER BY updated DESC`,
		},
		{
			name:    "type filter",
			filters: JQLFilters{Type: "Epic"},
			want:    `issuetype = "Epic" ORDER BY updated DESC`,
		},
		{
			name:    "combined filters",
			filters: JQLFilters{Project: "ESA", Status: "In Progress", Type: "Story", Epic: "ESA-42"},
			want:    `project = ESA AND status = "In Progress" AND issuetype = "Story" AND "Epic Link" = ESA-42 ORDER BY updated DESC`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildJQLWith(tc.filters)
			if got != tc.want {
				t.Errorf("BuildJQLWith() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveEpicFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/rest/api/2/field") {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "summary", "name": "Summary", "custom": false},
			{"id": "customfield_10011", "name": "Epic Name", "custom": true, "schema": map[string]any{"custom": SchemaEpicName}},
			{"id": "customfield_10014", "name": "Epic Link", "custom": true, "schema": map[string]any{"custom": SchemaEpicLink}},
			{"id": "customfield_10020", "name": "Sprint", "custom": true, "schema": map[string]any{"custom": "com.pyxis.greenhopper.jira:gh-sprint"}},
		})
	}))
	defer srv.Close()

	c := New(&config.Config{URL: srv.URL, Token: "x"})
	nameID, linkID, err := c.ResolveEpicFields(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nameID != "customfield_10011" {
		t.Errorf("epic name ID = %q, want customfield_10011", nameID)
	}
	if linkID != "customfield_10014" {
		t.Errorf("epic link ID = %q, want customfield_10014", linkID)
	}
}

func TestResolveEpicFields_Missing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Instance without the Agile plugin: no Epic custom fields.
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "summary", "name": "Summary", "custom": false},
		})
	}))
	defer srv.Close()

	c := New(&config.Config{URL: srv.URL, Token: "x"})
	_, _, err := c.ResolveEpicFields(context.Background())
	if err == nil {
		t.Fatal("expected error when epic fields are missing, got nil")
	}
}
