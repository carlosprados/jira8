package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amplia/jira8/internal/config"
	"github.com/amplia/jira8/internal/models"
)

// TestBoardView checks that each column is read with the board's filter and
// sub-filter in rank order, that WIP excludes sub-tasks when the board says so,
// and that only the whole board (no --assignee) is checked against limits.
func TestBoardView(t *testing.T) {
	var jqls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/configuration"):
			_ = json.NewEncoder(w).Encode(models.BoardConfiguration{
				Name: "Team", Type: "kanban", Filter: models.BoardFilter{ID: "77"},
				SubQuery: &models.BoardSubQuery{Query: "resolution is EMPTY"},
				ColumnConfig: models.BoardColumnConfig{ConstraintType: "issueCountExclSubs", Columns: []models.BoardColumn{
					{Name: "Backlog"},
					{Name: "Doing", Statuses: []models.BoardColumnStatus{{ID: "3"}, {ID: "4"}}, Max: 4},
				}},
				Ranking: &models.BoardRanking{RankCustomFieldID: 10200},
			})
		case strings.HasSuffix(r.URL.Path, "/search"):
			jql := r.URL.Query().Get("jql")
			jqls = append(jqls, jql)
			total := 6 // with sub-tasks
			if strings.Contains(jql, "subTaskIssueTypes") {
				total = 5
			}
			_ = json.NewEncoder(w).Encode(models.SearchResult{Total: total, Issues: []models.Issue{{Key: "A-1"}}})
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	c := New(&config.Config{URL: srv.URL, Token: "x"})

	view, err := c.BoardView(context.Background(), BoardViewRequest{Board: "9", Max: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Columns) != 2 || view.Columns[0].Total != 0 {
		t.Fatalf("columns = %+v", view.Columns)
	}
	doing := view.Columns[1]
	if doing.Total != 6 || doing.WIP != 5 || doing.Over != "max" {
		t.Errorf("Doing = total %d, wip %d, over %q; want 6, 5, max", doing.Total, doing.WIP, doing.Over)
	}
	want := `filter = 77 AND (resolution is EMPTY) AND status IN ("3", "4") ORDER BY cf[10200] ASC`
	if len(jqls) != 2 || jqls[0] != want {
		t.Errorf("jql = %q, want %q first", jqls, want)
	}

	jqls = nil
	view, err = c.BoardView(context.Background(), BoardViewRequest{Board: "9", Column: "doing", Assignee: "me"})
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Columns) != 1 || view.Columns[0].Over != "" {
		t.Errorf("with --assignee: %+v, want one column and no over", view.Columns)
	}
	if !strings.Contains(jqls[0], "assignee = currentUser()") {
		t.Errorf("jql = %q, want assignee clause", jqls[0])
	}
}

// TestBoardViewStale checks that only in-progress columns are checked, with a
// query that skips cards created within the period, and that the listed stale
// cards are flagged.
func TestBoardViewStale(t *testing.T) {
	var staleJQL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/configuration"):
			_ = json.NewEncoder(w).Encode(models.BoardConfiguration{
				Name: "Team", Filter: models.BoardFilter{ID: "77"},
				ColumnConfig: models.BoardColumnConfig{Columns: []models.BoardColumn{
					{Name: "To Do", Statuses: []models.BoardColumnStatus{{ID: "1"}}},
					{Name: "Doing", Statuses: []models.BoardColumnStatus{{ID: "3"}}},
				}},
				Ranking: &models.BoardRanking{RankCustomFieldID: 10200},
			})
		case strings.HasSuffix(r.URL.Path, "/status"):
			_, _ = w.Write([]byte(`[{"id":"1","statusCategory":{"key":"new"}},{"id":"3","statusCategory":{"key":"indeterminate"}}]`))
		case strings.HasSuffix(r.URL.Path, "/search"):
			jql := r.URL.Query().Get("jql")
			issues := []models.Issue{{Key: "A-1"}, {Key: "A-2"}}
			if strings.Contains(jql, "NOT status CHANGED") {
				staleJQL = jql
				issues = issues[1:]
			}
			_ = json.NewEncoder(w).Encode(models.SearchResult{Total: len(issues), Issues: issues})
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	c := New(&config.Config{URL: srv.URL, Token: "x"})

	view, err := c.BoardView(context.Background(), BoardViewRequest{Board: "9", Max: 10, Stale: "-7d"})
	if err != nil {
		t.Fatal(err)
	}
	todo, doing := view.Columns[0], view.Columns[1]
	if todo.Stale != nil {
		t.Errorf("To Do checked for stale cards: %d", *todo.Stale)
	}
	if doing.Stale == nil || *doing.Stale != 1 || !doing.StaleKeys["A-2"] || doing.StaleKeys["A-1"] {
		t.Errorf("Doing stale = %v, keys %v; want 1, A-2 only", doing.Stale, doing.StaleKeys)
	}
	if !strings.Contains(staleJQL, "NOT status CHANGED AFTER -7d AND created < -7d") {
		t.Errorf("stale jql = %q", staleJQL)
	}
}

// TestResolveColumnPrefersStatuses covers a kanban board showing an empty
// Backlog column next to the real one.
func TestResolveColumnPrefersStatuses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(models.BoardConfiguration{
			Name: "Team", ColumnConfig: models.BoardColumnConfig{Columns: []models.BoardColumn{
				{Name: "Backlog"},
				{Name: "Backlog", Statuses: []models.BoardColumnStatus{{ID: "10006"}}},
			}},
		})
	}))
	defer srv.Close()
	c := New(&config.Config{URL: srv.URL, Token: "x"})

	_, col, err := c.ResolveColumn(context.Background(), "", "9", "backlog")
	if err != nil {
		t.Fatal(err)
	}
	if len(col.Statuses) != 1 {
		t.Errorf("picked the empty Backlog column")
	}
	if _, _, err := c.ResolveColumn(context.Background(), "", "9", "Nope"); err == nil || !strings.Contains(err.Error(), "columns: Backlog, Backlog") {
		t.Errorf("err = %v, want the column list", err)
	}
}
