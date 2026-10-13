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
