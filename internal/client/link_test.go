package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amplia/jira8/internal/config"
	"github.com/amplia/jira8/internal/models"
)

// TestGetIssueLinkTypes verifies the GET /issueLinkType response is parsed into
// the typed slice.
func TestGetIssueLinkTypes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/rest/api/2/issueLinkType") {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issueLinkTypes": []map[string]any{
				{"id": "10000", "name": "Blocks", "inward": "is blocked by", "outward": "blocks"},
				{"id": "10001", "name": "Relates", "inward": "relates to", "outward": "relates to"},
			},
		})
	}))
	defer srv.Close()

	c := New(&config.Config{URL: srv.URL, Token: "secret"})
	types, err := c.GetIssueLinkTypes(context.Background())
	if err != nil {
		t.Fatalf("GetIssueLinkTypes: %v", err)
	}
	if len(types) != 2 {
		t.Fatalf("len(types) = %d, want 2", len(types))
	}
	if types[1].Name != "Relates" || types[1].Outward != "relates to" {
		t.Errorf("unexpected type[1]: %+v", types[1])
	}
}

// TestLinkIssues_PostsBody verifies LinkIssues POSTs the correct body to
// /issueLink with the type, both issue keys and the optional comment.
func TestLinkIssues_PostsBody(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody models.IssueLinkRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &gotBody)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := New(&config.Config{URL: srv.URL, Token: "secret"})
	req := &models.IssueLinkRequest{
		Type:         models.IssueLinkTypeRef{Name: "Relates"},
		OutwardIssue: models.LinkedIssueRef{Key: "ESA-207"},
		InwardIssue:  models.LinkedIssueRef{Key: "ESA-214"},
		Comment:      &models.IssueLinkComment{Body: "same work"},
	}
	if err := c.LinkIssues(context.Background(), req); err != nil {
		t.Fatalf("LinkIssues: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if !strings.HasSuffix(gotPath, "/rest/api/2/issueLink") {
		t.Errorf("path = %s, want suffix /rest/api/2/issueLink", gotPath)
	}
	if gotBody.Type.Name != "Relates" {
		t.Errorf("type = %q, want Relates", gotBody.Type.Name)
	}
	if gotBody.OutwardIssue.Key != "ESA-207" || gotBody.InwardIssue.Key != "ESA-214" {
		t.Errorf("issues = %s / %s, want ESA-207 / ESA-214", gotBody.OutwardIssue.Key, gotBody.InwardIssue.Key)
	}
	if gotBody.Comment == nil || gotBody.Comment.Body != "same work" {
		t.Errorf("comment = %+v, want body 'same work'", gotBody.Comment)
	}
}

// TestLinkIssues_APIError verifies a Jira 4xx is surfaced as a Go error.
func TestLinkIssues_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errorMessages": []string{"No issue link type with name 'Nope' found"},
		})
	}))
	defer srv.Close()

	c := New(&config.Config{URL: srv.URL, Token: "secret"})
	err := c.LinkIssues(context.Background(), &models.IssueLinkRequest{
		Type:         models.IssueLinkTypeRef{Name: "Nope"},
		OutwardIssue: models.LinkedIssueRef{Key: "ESA-1"},
		InwardIssue:  models.LinkedIssueRef{Key: "ESA-2"},
	})
	if err == nil {
		t.Fatal("LinkIssues: expected error on HTTP 400, got nil")
	}
}
