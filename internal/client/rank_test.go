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

// fakeJira emulates the endpoints RankIssuesRelative touches: issue lookup,
// Agile boards and their configuration, JQL search and the rank call itself.
type fakeJira struct {
	t *testing.T
	// statuses maps issue key → status ID.
	statuses map[string]string
	// project is the project key reported for every issue ("" to omit it).
	project string
	// boards is what GET /board returns for the project.
	boards []models.Board
	// columns is the board's column configuration.
	columns []models.BoardColumn
	// ranking is the board's reported rank field ("nil" → empty object, which
	// forces the /field fallback).
	ranking *models.BoardRanking
	// columnIssues is the column content in ascending rank order.
	columnIssues []string

	// Captured for assertions.
	lastJQL   string
	rankBody  models.RankIssuesRequest
	rankCalls int
	fieldGets int
}

func (f *fakeJira) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	w.Header().Set("Content-Type", "application/json")

	switch {
	case r.Method == http.MethodPut && strings.HasSuffix(path, "/rest/agile/1.0/issue/rank"):
		f.rankCalls++
		if err := json.NewDecoder(r.Body).Decode(&f.rankBody); err != nil {
			f.t.Errorf("decoding rank body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)

	case strings.HasSuffix(path, "/rest/agile/1.0/board"):
		if got := r.URL.Query().Get("projectKeyOrId"); got != f.project {
			f.t.Errorf("board query projectKeyOrId = %q, want %q", got, f.project)
		}
		_ = json.NewEncoder(w).Encode(models.BoardsResponse{
			IsLast: true, Total: len(f.boards), Values: f.boards,
		})

	case strings.Contains(path, "/rest/agile/1.0/board/") && strings.HasSuffix(path, "/configuration"):
		_ = json.NewEncoder(w).Encode(models.BoardConfiguration{
			ID:           f.boards[0].ID,
			Name:         f.boards[0].Name,
			Type:         f.boards[0].Type,
			Filter:       models.BoardFilter{ID: "12809"},
			ColumnConfig: models.BoardColumnConfig{Columns: f.columns},
			Ranking:      f.ranking,
		})

	case strings.HasSuffix(path, "/rest/api/2/search"):
		f.lastJQL = r.URL.Query().Get("jql")
		keys := append([]string{}, f.columnIssues...)
		if strings.Contains(f.lastJQL, "DESC") {
			for i, j := 0, len(keys)-1; i < j; i, j = i+1, j-1 {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
		issues := make([]models.Issue, 0, len(keys))
		for _, k := range keys {
			issues = append(issues, models.Issue{Key: k})
		}
		_ = json.NewEncoder(w).Encode(models.SearchResult{Total: len(issues), Issues: issues})

	case strings.HasSuffix(path, "/rest/api/2/field"):
		f.fieldGets++
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "customfield_10002", "name": "Rank (Obsolete)", "custom": true,
				"schema": map[string]any{"custom": "com.pyxis.greenhopper.jira:gh-global-rank"}},
			{"id": "customfield_10200", "name": "Rank", "custom": true,
				"schema": map[string]any{"custom": SchemaRank}},
		})

	case strings.Contains(path, "/rest/api/2/issue/"):
		key := path[strings.LastIndex(path, "/")+1:]
		status, ok := f.statuses[key]
		if !ok {
			http.NotFound(w, r)
			return
		}
		fields := map[string]any{"status": map[string]any{"id": status, "name": "Status " + status}}
		if f.project != "" {
			fields["project"] = map[string]any{"key": f.project}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"key": key, "fields": fields})

	default:
		f.t.Errorf("unexpected request: %s %s", r.Method, path)
		http.NotFound(w, r)
	}
}

// newFake returns a fake Jira with one kanban board whose "In Progress" column
// holds PHO-1, PHO-2 and PHO-3 in that rank order.
func newFake(t *testing.T) (*fakeJira, *Client, func()) {
	f := &fakeJira{
		t:        t,
		project:  "PHO",
		statuses: map[string]string{"PHO-1": "3", "PHO-2": "3", "PHO-3": "3", "PHO-9": "99"},
		boards:   []models.Board{{ID: 110, Name: "Phoenix Kanban", Type: "kanban"}},
		columns: []models.BoardColumn{
			{Name: "Backlog", Statuses: []models.BoardColumnStatus{{ID: "10006"}}},
			{Name: "In Progress", Statuses: []models.BoardColumnStatus{{ID: "3"}, {ID: "10003"}}},
		},
		ranking:      &models.BoardRanking{RankCustomFieldID: 10200},
		columnIssues: []string{"PHO-1", "PHO-2", "PHO-3"},
	}
	srv := httptest.NewServer(f)
	return f, New(&config.Config{URL: srv.URL, Token: "x"}), srv.Close
}

func TestRankTop(t *testing.T) {
	f, c, done := newFake(t)
	defer done()

	got, err := c.RankIssuesRelative(context.Background(), RankRequest{
		Keys: []string{"PHO-3"}, Position: RankTop,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Anchor != "PHO-1" {
		t.Errorf("anchor = %q, want PHO-1 (first issue of the column)", got.Anchor)
	}
	if got.Column != "In Progress" {
		t.Errorf("column = %q, want In Progress", got.Column)
	}
	if got.Board == nil || got.Board.Name != "Phoenix Kanban" {
		t.Errorf("board = %+v, want Phoenix Kanban", got.Board)
	}
	if got.NoOp {
		t.Error("NoOp = true, want false")
	}

	// The column's statuses must scope the query, ordered by the board's rank field.
	for _, want := range []string{"filter = 12809", `status IN ("3", "10003")`, "ORDER BY cf[10200] ASC"} {
		if !strings.Contains(f.lastJQL, want) {
			t.Errorf("JQL %q does not contain %q", f.lastJQL, want)
		}
	}
	if f.rankBody.RankBeforeIssue != "PHO-1" {
		t.Errorf("rankBeforeIssue = %q, want PHO-1", f.rankBody.RankBeforeIssue)
	}
	if f.rankBody.RankAfterIssue != "" {
		t.Errorf("rankAfterIssue = %q, want empty", f.rankBody.RankAfterIssue)
	}
	if f.rankBody.RankCustomFieldID != 10200 {
		t.Errorf("rankCustomFieldId = %d, want 10200", f.rankBody.RankCustomFieldID)
	}
	if f.fieldGets != 0 {
		t.Errorf("GET /field calls = %d, want 0 (board reported the rank field)", f.fieldGets)
	}
}

// The issue already at the top must anchor on the next issue, which leaves it in
// place instead of ranking it relative to itself.
func TestRankTop_AlreadyFirstIsIdempotent(t *testing.T) {
	f, c, done := newFake(t)
	defer done()

	got, err := c.RankIssuesRelative(context.Background(), RankRequest{
		Keys: []string{"PHO-1"}, Position: RankTop,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Anchor != "PHO-2" {
		t.Errorf("anchor = %q, want PHO-2", got.Anchor)
	}
	if f.rankBody.RankBeforeIssue != "PHO-2" {
		t.Errorf("rankBeforeIssue = %q, want PHO-2", f.rankBody.RankBeforeIssue)
	}
}

func TestRankBottom(t *testing.T) {
	f, c, done := newFake(t)
	defer done()

	got, err := c.RankIssuesRelative(context.Background(), RankRequest{
		Keys: []string{"PHO-1"}, Position: RankBottom,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Anchor != "PHO-3" {
		t.Errorf("anchor = %q, want PHO-3 (last issue of the column)", got.Anchor)
	}
	if !strings.Contains(f.lastJQL, "ORDER BY cf[10200] DESC") {
		t.Errorf("JQL %q must order descending for a bottom move", f.lastJQL)
	}
	if f.rankBody.RankAfterIssue != "PHO-3" {
		t.Errorf("rankAfterIssue = %q, want PHO-3", f.rankBody.RankAfterIssue)
	}
	if f.rankBody.RankBeforeIssue != "" {
		t.Errorf("rankBeforeIssue = %q, want empty", f.rankBody.RankBeforeIssue)
	}
}

// Ranking a block keeps the whole set out of the anchor search: the anchor must
// be the first issue that is not being moved.
func TestRankTop_BlockSkipsMovedIssues(t *testing.T) {
	f, c, done := newFake(t)
	defer done()

	got, err := c.RankIssuesRelative(context.Background(), RankRequest{
		Keys: []string{"PHO-1", "PHO-2"}, Position: RankTop,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Anchor != "PHO-3" {
		t.Errorf("anchor = %q, want PHO-3", got.Anchor)
	}
	if len(f.rankBody.Issues) != 2 || f.rankBody.Issues[0] != "PHO-1" {
		t.Errorf("issues = %v, want [PHO-1 PHO-2] in order", f.rankBody.Issues)
	}
}

// When every issue in the column is being moved there is no anchor to rank
// against, so the operation must be reported as a no-op without calling Jira.
func TestRankTop_NoOpWhenColumnFullyMoved(t *testing.T) {
	f, c, done := newFake(t)
	defer done()
	f.columnIssues = []string{"PHO-1"}

	got, err := c.RankIssuesRelative(context.Background(), RankRequest{
		Keys: []string{"PHO-1"}, Position: RankTop,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.NoOp {
		t.Error("NoOp = false, want true")
	}
	if got.Anchor != "" {
		t.Errorf("anchor = %q, want empty", got.Anchor)
	}
	if f.rankCalls != 0 {
		t.Errorf("rank calls = %d, want 0", f.rankCalls)
	}
}

// Boards that report an empty "ranking" object must fall back to resolving the
// LexoRank field from /field — and must not pick the obsolete global rank.
func TestRankTop_RankFieldFallback(t *testing.T) {
	f, c, done := newFake(t)
	defer done()
	f.ranking = nil

	if _, err := c.RankIssuesRelative(context.Background(), RankRequest{
		Keys: []string{"PHO-3"}, Position: RankTop,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.fieldGets != 1 {
		t.Errorf("GET /field calls = %d, want 1", f.fieldGets)
	}
	if f.rankBody.RankCustomFieldID != 10200 {
		t.Errorf("rankCustomFieldId = %d, want 10200", f.rankBody.RankCustomFieldID)
	}
	if !strings.Contains(f.lastJQL, "cf[10200]") {
		t.Errorf("JQL %q must order by the resolved rank field", f.lastJQL)
	}
}

// An issue whose status is not mapped to any column is not on the board, so
// "top of its column" is meaningless and must fail loudly.
func TestRankTop_StatusNotOnBoard(t *testing.T) {
	f, c, done := newFake(t)
	defer done()

	_, err := c.RankIssuesRelative(context.Background(), RankRequest{
		Keys: []string{"PHO-9"}, Position: RankTop,
	})
	if err == nil {
		t.Fatal("expected an error for a status outside the board columns")
	}
	if !strings.Contains(err.Error(), "not mapped to any column") {
		t.Errorf("error = %v, want it to explain the unmapped status", err)
	}
	if f.rankCalls != 0 {
		t.Errorf("rank calls = %d, want 0", f.rankCalls)
	}
}

func TestRankTop_AmbiguousBoard(t *testing.T) {
	f, c, done := newFake(t)
	defer done()
	f.boards = append(f.boards, models.Board{ID: 119, Name: "Phoenix Scrum", Type: "scrum"})

	_, err := c.RankIssuesRelative(context.Background(), RankRequest{
		Keys: []string{"PHO-1"}, Position: RankTop,
	})
	if err == nil {
		t.Fatal("expected an error when the project has several boards")
	}
	// The message must name the candidates so the user can pick one.
	for _, want := range []string{"Phoenix Kanban", "Phoenix Scrum", "110", "119"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %v does not mention %q", err, want)
		}
	}
}

func TestRankTop_BoardByName(t *testing.T) {
	f, c, done := newFake(t)
	defer done()
	f.boards = append(f.boards, models.Board{ID: 119, Name: "Phoenix Scrum", Type: "scrum"})

	got, err := c.RankIssuesRelative(context.Background(), RankRequest{
		Keys: []string{"PHO-3"}, Position: RankTop, Board: "phoenix kanban",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Board == nil || got.Board.ID != 110 {
		t.Errorf("board = %+v, want ID 110 matched case-insensitively by name", got.Board)
	}
}

func TestRankTop_BoardNotFound(t *testing.T) {
	_, c, done := newFake(t)
	defer done()

	_, err := c.RankIssuesRelative(context.Background(), RankRequest{
		Keys: []string{"PHO-1"}, Position: RankTop, Board: "Nonexistent",
	})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %v, want a not-found error listing the boards", err)
	}
}

// before/after need no board lookup: the anchor is explicit.
func TestRankBefore(t *testing.T) {
	f, c, done := newFake(t)
	defer done()

	got, err := c.RankIssuesRelative(context.Background(), RankRequest{
		Keys: []string{"PHO-3"}, Position: RankBefore, Anchor: "PHO-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Anchor != "PHO-1" || got.Board != nil || got.Column != "" {
		t.Errorf("result = %+v, want anchor PHO-1 and no board/column lookup", got)
	}
	if f.rankBody.RankBeforeIssue != "PHO-1" {
		t.Errorf("rankBeforeIssue = %q, want PHO-1", f.rankBody.RankBeforeIssue)
	}
	if f.rankBody.RankCustomFieldID != 10200 {
		t.Errorf("rankCustomFieldId = %d, want 10200", f.rankBody.RankCustomFieldID)
	}
	if f.lastJQL != "" {
		t.Errorf("unexpected search for an explicit anchor: %q", f.lastJQL)
	}
}

func TestRankAfter(t *testing.T) {
	f, c, done := newFake(t)
	defer done()

	if _, err := c.RankIssuesRelative(context.Background(), RankRequest{
		Keys: []string{"PHO-1"}, Position: RankAfter, Anchor: "PHO-3",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.rankBody.RankAfterIssue != "PHO-3" {
		t.Errorf("rankAfterIssue = %q, want PHO-3", f.rankBody.RankAfterIssue)
	}
}

func TestRankIssuesRelative_Validation(t *testing.T) {
	tests := []struct {
		name string
		req  RankRequest
		want string
	}{
		{
			name: "no keys",
			req:  RankRequest{Position: RankTop},
			want: "no issues to rank",
		},
		{
			name: "anchor is the moved issue",
			req:  RankRequest{Keys: []string{"PHO-1"}, Position: RankBefore, Anchor: "pho-1"},
			want: "relative to itself",
		},
		{
			name: "missing anchor",
			req:  RankRequest{Keys: []string{"PHO-1"}, Position: RankAfter},
			want: "requires an anchor",
		},
		{
			name: "unknown position",
			req:  RankRequest{Keys: []string{"PHO-1"}, Position: RankPosition("sideways")},
			want: "unknown rank position",
		},
		{
			name: "too many issues",
			req:  RankRequest{Keys: make([]string, maxRankIssues+1), Position: RankTop},
			want: "cannot rank more than 50",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, c, done := newFake(t)
			defer done()

			_, err := c.RankIssuesRelative(context.Background(), tc.req)
			if err == nil {
				t.Fatalf("expected an error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to contain %q", err, tc.want)
			}
			if f.rankCalls != 0 {
				t.Errorf("rank calls = %d, want 0", f.rankCalls)
			}
		})
	}
}
