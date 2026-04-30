package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amplia/jira8/internal/client"
	"github.com/amplia/jira8/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
)

// stubJiraServer serves a fixed priority list and a fixed issue, used to exercise
// the resource handlers without touching a real Jira instance.
func stubJiraServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/rest/api/2/priority"):
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "1", "name": "Blocker"},
				{"id": "2", "name": "Critical"},
			})
		case strings.HasSuffix(r.URL.Path, "/rest/api/2/issue/TEST-1"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "1", "key": "TEST-1", "self": r.URL.String(),
				"fields": map[string]any{"summary": "stub"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestPrioritiesResourceHandler(t *testing.T) {
	srv := stubJiraServer(t)
	defer srv.Close()
	jc := client.New(&config.Config{URL: srv.URL, Token: "x"})

	handler := prioritiesResourceHandler(jc)
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "jira://priorities"

	contents, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(contents))
	}
	trc, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatalf("expected TextResourceContents, got %T", contents[0])
	}
	if trc.MIMEType != "application/json" {
		t.Errorf("mime=%q, want application/json", trc.MIMEType)
	}
	if !strings.Contains(trc.Text, "Blocker") {
		t.Errorf("payload missing Blocker: %q", trc.Text)
	}
}

// TestTemplateString_AcceptsUriTemplateSlice documents the gotcha: mcp-go stores
// the parsed URI template captures as []string (the V field of a
// yosida95/uritemplate Value), not a plain string. templateString must unwrap it.
func TestTemplateString_AcceptsUriTemplateSlice(t *testing.T) {
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "jira://issues/TEST-1"
	req.Params.Arguments = map[string]any{"key": []string{"TEST-1"}}

	got, err := templateString(req, "key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "TEST-1" {
		t.Errorf("got %q, want TEST-1", got)
	}
}

func TestTemplateString_PlainStringStillWorks(t *testing.T) {
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "jira://issues/TEST-1"
	req.Params.Arguments = map[string]any{"key": "TEST-1"}
	got, err := templateString(req, "key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "TEST-1" {
		t.Errorf("got %q, want TEST-1", got)
	}
}

func TestTemplateString_MissingOrEmpty(t *testing.T) {
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "jira://issues/"
	req.Params.Arguments = map[string]any{"key": []string{""}}
	if _, err := templateString(req, "key"); err == nil {
		t.Error("expected error on empty string, got nil")
	}
	req.Params.Arguments = map[string]any{}
	if _, err := templateString(req, "key"); err == nil {
		t.Error("expected error on missing key, got nil")
	}
}

func TestIssueResourceHandler(t *testing.T) {
	srv := stubJiraServer(t)
	defer srv.Close()
	jc := client.New(&config.Config{URL: srv.URL, Token: "x"})

	handler := issueResourceHandler(jc)
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "jira://issues/TEST-1"
	req.Params.Arguments = map[string]any{"key": []string{"TEST-1"}}

	contents, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	trc := contents[0].(mcp.TextResourceContents)
	var payload map[string]any
	if err := json.Unmarshal([]byte(trc.Text), &payload); err != nil {
		t.Fatalf("payload not JSON: %v", err)
	}
	if payload["key"] != "TEST-1" {
		t.Errorf("expected key=TEST-1, got %v", payload["key"])
	}
}
