package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amplia/jira8/internal/client"
	"github.com/amplia/jira8/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
)

// TestEditIssueHandler_DueDateAndEstimate checks that due_date and
// original_estimate land on the PUT body as Jira expects: duedate as a plain
// date string (nil to clear), timetracking as a nested originalEstimate.
func TestEditIssueHandler_DueDateAndEstimate(t *testing.T) {
	var gotFields map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		var body struct {
			Fields map[string]any `json:"fields"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		gotFields = body.Fields
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	jc := client.New(&config.Config{URL: srv.URL, Token: "x"})
	handler := editIssueHandler(jc)

	req := mcp.CallToolRequest{}
	req.Params.Name = "jira_edit_issue"
	req.Params.Arguments = map[string]any{
		"key":               "TEST-1",
		"due_date":          "2026-09-24",
		"original_estimate": "5d",
	}

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if res.IsError {
		t.Fatalf("handler returned tool error: %+v", res.Content)
	}

	if gotFields["duedate"] != "2026-09-24" {
		t.Errorf("duedate = %v, want 2026-09-24", gotFields["duedate"])
	}

	tt, ok := gotFields["timetracking"].(map[string]any)
	if !ok {
		t.Fatalf("timetracking = %T %v, want map", gotFields["timetracking"], gotFields["timetracking"])
	}
	if tt["originalEstimate"] != "5d" {
		t.Errorf("timetracking.originalEstimate = %v, want 5d", tt["originalEstimate"])
	}
}

// TestEditIssueHandler_ClearDueDate checks the empty-string convention used
// elsewhere in this handler (epic_link, assignee): due_date: "" clears the
// field by sending fields.duedate = null.
func TestEditIssueHandler_ClearDueDate(t *testing.T) {
	var gotFields map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Fields map[string]any `json:"fields"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotFields = body.Fields
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	jc := client.New(&config.Config{URL: srv.URL, Token: "x"})
	handler := editIssueHandler(jc)

	req := mcp.CallToolRequest{}
	req.Params.Name = "jira_edit_issue"
	req.Params.Arguments = map[string]any{
		"key":      "TEST-1",
		"due_date": "",
	}

	if _, err := handler(context.Background(), req); err != nil {
		t.Fatalf("handler: %v", err)
	}

	if v, ok := gotFields["duedate"]; !ok || v != nil {
		t.Errorf("duedate = %v (present=%v), want nil", v, ok)
	}
}
