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

func TestCreateBugReportPrompt_RequiredArgs(t *testing.T) {
	handler := createBugReportPromptHandler("ESA")
	req := mcp.GetPromptRequest{}
	req.Params.Name = "create_bug_report"
	req.Params.Arguments = map[string]string{"summary": "x"} // missing the rest

	if _, err := handler(context.Background(), req); err == nil {
		t.Fatal("expected error on missing required args, got nil")
	}
}

func TestCreateBugReportPrompt_ProducesWikiMarkup(t *testing.T) {
	handler := createBugReportPromptHandler("ESA")
	req := mcp.GetPromptRequest{}
	req.Params.Arguments = map[string]string{
		"summary":            "Login fails",
		"steps_to_reproduce": "1. Go to login\n2. Submit",
		"expected_behavior":  "User authenticates",
		"actual_behavior":    "500 error",
		"environment":        "Firefox 120, Linux",
	}
	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(res.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(res.Messages))
	}
	tc, ok := res.Messages[0].Content.(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Messages[0].Content)
	}
	for _, want := range []string{"h2. Steps to reproduce", "h2. Environment", "type=Bug", "ESA"} {
		if !strings.Contains(tc.Text, want) {
			t.Errorf("message missing %q in:\n%s", want, tc.Text)
		}
	}
}

func TestTriageIssuePrompt_EmbedsIssueData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/rest/api/2/issue/TEST-1") {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "1", "key": "TEST-1",
			"fields": map[string]any{
				"summary":  "Widget broken",
				"status":   map[string]any{"name": "To Do"},
				"priority": map[string]any{"name": "Major"},
			},
		})
	}))
	defer srv.Close()

	jc := client.New(&config.Config{URL: srv.URL, Token: "x"})
	handler := triageIssuePromptHandler(jc)
	req := mcp.GetPromptRequest{}
	req.Params.Arguments = map[string]string{"key": "TEST-1"}

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	tc := res.Messages[0].Content.(mcp.TextContent)
	for _, want := range []string{"Widget broken", "Major", "To Do", "Priority assessment"} {
		if !strings.Contains(tc.Text, want) {
			t.Errorf("triage prompt missing %q", want)
		}
	}
}

func TestTriageIssuePrompt_MissingKey(t *testing.T) {
	jc := client.New(&config.Config{URL: "http://unused", Token: "x"})
	handler := triageIssuePromptHandler(jc)
	req := mcp.GetPromptRequest{}
	req.Params.Arguments = map[string]string{}
	if _, err := handler(context.Background(), req); err == nil {
		t.Fatal("expected error on missing key, got nil")
	}
}

func TestSummariseCommentsPrompt_EmbedsComments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/rest/api/2/issue/TEST-1/comment"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"total": 2,
				"comments": []map[string]any{
					{"id": "1", "body": "we should ship friday", "created": "2026-04-29T10:00:00Z", "author": map[string]any{"displayName": "Alice"}},
					{"id": "2", "body": "agreed, blocked on infra", "created": "2026-04-29T11:00:00Z", "author": map[string]any{"displayName": "Bob"}},
				},
			})
		case strings.HasSuffix(r.URL.Path, "/rest/api/2/issue/TEST-1"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "1", "key": "TEST-1",
				"fields": map[string]any{"summary": "Release prep"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	jc := client.New(&config.Config{URL: srv.URL, Token: "x"})
	handler := summariseCommentsPromptHandler(jc)
	req := mcp.GetPromptRequest{}
	req.Params.Arguments = map[string]string{"key": "TEST-1"}

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	tc := res.Messages[0].Content.(mcp.TextContent)
	for _, want := range []string{"Release prep", "Alice", "Bob", "ship friday", "Decisions taken", "Open questions"} {
		if !strings.Contains(tc.Text, want) {
			t.Errorf("summarise_comments prompt missing %q in:\n%s", want, tc.Text)
		}
	}
}

func TestSummariseCommentsPrompt_MissingKey(t *testing.T) {
	jc := client.New(&config.Config{URL: "http://unused", Token: "x"})
	handler := summariseCommentsPromptHandler(jc)
	req := mcp.GetPromptRequest{}
	req.Params.Arguments = map[string]string{}
	if _, err := handler(context.Background(), req); err == nil {
		t.Fatal("expected error on missing key, got nil")
	}
}
