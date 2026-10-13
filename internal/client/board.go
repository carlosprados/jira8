package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/amplia/jira8/internal/models"
)

// BoardViewRequest selects what BoardView shows.
type BoardViewRequest struct {
	Board    string // id or name; empty = the project's only board
	Project  string // needed to resolve a board by name or by default
	Column   string // only this column (case-insensitive); empty = all
	Assignee string // only issues of this user ("me" = current user)
	Max      int    // issues listed per column; the count covers them all
	Stale    string // JQL period (-7d): flag in-flight cards whose status is older
}

// BoardView is a board's columns with their issues in board order.
type BoardView struct {
	Board   models.Board  `json:"board"`
	Columns []BoardColumn `json:"columns"`
}

// BoardColumn is one column of a BoardView. WIP is the count the board's limits
// apply to (sub-tasks excluded when the board says so); Over is "max" or "min"
// when that count is outside the limits.
type BoardColumn struct {
	Name  string `json:"name"`
	Total int    `json:"total"`
	Min   int    `json:"min,omitempty"`
	Max   int    `json:"max,omitempty"`
	WIP   int    `json:"wip"`
	Over  string `json:"over,omitempty"`
	// Stale counts the cards whose status has not changed for BoardViewRequest.Stale;
	// nil when not asked, or in to-do and done columns, where waiting is normal.
	Stale     *int            `json:"stale,omitempty"`
	StaleKeys map[string]bool `json:"-"`
	Issues    []models.Issue  `json:"-"`
}

// BoardView reads a board as Jira draws it: its saved filter plus the kanban
// sub-filter, each column's statuses, and the rank order.
func (c *Client) BoardView(ctx context.Context, req BoardViewRequest) (*BoardView, error) {
	board, err := c.resolveBoard(ctx, req.Project, req.Board)
	if err != nil {
		return nil, err
	}
	cfg, err := c.GetBoardConfiguration(ctx, board.ID)
	if err != nil {
		return nil, err
	}
	board.Name, board.Type = cfg.Name, cfg.Type
	if cfg.Filter.ID == "" {
		return nil, fmt.Errorf("board %q does not expose its saved filter", board.Name)
	}
	rankField := 0
	if cfg.Ranking != nil {
		rankField = cfg.Ranking.RankCustomFieldID
	}
	if rankField == 0 {
		if rankField, err = c.ResolveRankField(ctx); err != nil {
			return nil, err
		}
	}

	scope := "filter = " + cfg.Filter.ID
	if cfg.SubQuery != nil && strings.TrimSpace(cfg.SubQuery.Query) != "" {
		scope += " AND (" + cfg.SubQuery.Query + ")"
	}
	if req.Assignee != "" {
		if strings.EqualFold(req.Assignee, "me") {
			scope += " AND assignee = currentUser()"
		} else {
			scope += " AND assignee = " + jqlQuote(req.Assignee)
		}
	}

	var categories map[string]string
	if req.Stale != "" {
		if categories, err = c.StatusCategories(ctx); err != nil {
			return nil, err
		}
	}

	view := &BoardView{Board: *board}
	var names []string
	for _, col := range cfg.ColumnConfig.Columns {
		names = append(names, col.Name)
		if req.Column != "" && !strings.EqualFold(col.Name, req.Column) {
			continue
		}
		bc := BoardColumn{Name: col.Name, Min: col.Min, Max: col.Max}
		if len(col.Statuses) > 0 {
			in := scope + " AND status IN (" + quoteJoin(columnStatusIDs(&col)) + ")"
			res, err := c.SearchIssues(ctx, fmt.Sprintf("%s ORDER BY cf[%d] ASC", in, rankField), 0, max(req.Max, 0))
			if err != nil {
				return nil, fmt.Errorf("column %q: %w", col.Name, err)
			}
			bc.Total, bc.Issues, bc.WIP = res.Total, res.Issues, res.Total
			if (col.Min > 0 || col.Max > 0) && cfg.ColumnConfig.ConstraintType == "issueCountExclSubs" {
				res, err := c.SearchIssues(ctx, in+" AND issuetype NOT IN subTaskIssueTypes()", 0, 0)
				if err != nil {
					return nil, fmt.Errorf("column %q: %w", col.Name, err)
				}
				bc.WIP = res.Total
			}
			if req.Stale != "" && inFlight(&col, categories) {
				if err := c.markStale(ctx, &bc, in, req.Stale); err != nil {
					return nil, fmt.Errorf("column %q: %w", col.Name, err)
				}
			}
		}
		// Limits apply to the whole column, not to one assignee's share of it.
		if req.Assignee == "" && cfg.ColumnConfig.ConstraintType != "none" {
			switch {
			case col.Max > 0 && bc.WIP > col.Max:
				bc.Over = "max"
			case col.Min > 0 && bc.WIP < col.Min:
				bc.Over = "min"
			}
		}
		view.Columns = append(view.Columns, bc)
	}
	if req.Column != "" && len(view.Columns) == 0 {
		return nil, fmt.Errorf("column %q not on board %q; columns: %s", req.Column, board.Name, strings.Join(names, ", "))
	}
	return view, nil
}

// inFlight reports whether a column holds work in progress: one of its
// statuses is in Jira's "indeterminate" category (neither to-do nor done).
func inFlight(col *models.BoardColumn, categories map[string]string) bool {
	for _, st := range col.Statuses {
		if categories[st.ID] == "indeterminate" {
			return true
		}
	}
	return false
}

// markStale counts the column's cards whose status has not changed within
// period, and flags those among the listed ones. Cards created within the
// period never had the chance to move, so they do not count.
func (c *Client) markStale(ctx context.Context, bc *BoardColumn, in, period string) error {
	stale := in + " AND NOT status CHANGED AFTER " + period + " AND created < " + period
	res, err := c.SearchIssues(ctx, stale, 0, 0)
	if err != nil {
		return err
	}
	n := res.Total
	bc.Stale = &n
	if n == 0 || len(bc.Issues) == 0 {
		return nil
	}
	keys := make([]string, len(bc.Issues))
	for i, is := range bc.Issues {
		keys[i] = is.Key
	}
	res, err = c.SearchIssues(ctx, stale+" AND key IN ("+strings.Join(keys, ", ")+")", 0, len(keys))
	if err != nil {
		return err
	}
	bc.StaleKeys = map[string]bool{}
	for _, is := range res.Issues {
		bc.StaleKeys[is.Key] = true
	}
	return nil
}

// ResolveColumn finds a column of a board by name (case-insensitive). Board
// is an id or name, or empty when the project has a single board.
func (c *Client) ResolveColumn(ctx context.Context, project, boardRef, column string) (*models.Board, *models.BoardColumn, error) {
	board, err := c.resolveBoard(ctx, project, boardRef)
	if err != nil {
		return nil, nil, err
	}
	cfg, err := c.GetBoardConfiguration(ctx, board.ID)
	if err != nil {
		return nil, nil, err
	}
	board.Name, board.Type = cfg.Name, cfg.Type
	// A kanban board can show an empty Backlog column next to the real one:
	// prefer the column that has statuses.
	var names []string
	var match *models.BoardColumn
	for i, col := range cfg.ColumnConfig.Columns {
		names = append(names, col.Name)
		if strings.EqualFold(col.Name, column) && (match == nil || len(match.Statuses) == 0) {
			match = &cfg.ColumnConfig.Columns[i]
		}
	}
	if match != nil {
		return board, match, nil
	}
	return nil, nil, fmt.Errorf("column %q not on board %q; columns: %s", column, board.Name, strings.Join(names, ", "))
}

// StatusCategories maps every status id to its category key: "new" (to do),
// "indeterminate" (in progress) or "done".
func (c *Client) StatusCategories(ctx context.Context) (map[string]string, error) {
	data, err := c.do(ctx, http.MethodGet, "/status", nil)
	if err != nil {
		return nil, err
	}
	var statuses []struct {
		ID       string `json:"id"`
		Category struct {
			Key string `json:"key"`
		} `json:"statusCategory"`
	}
	if err := json.Unmarshal(data, &statuses); err != nil {
		return nil, fmt.Errorf("parsing statuses: %w", err)
	}
	out := make(map[string]string, len(statuses))
	for _, st := range statuses {
		out[st.ID] = st.Category.Key
	}
	return out, nil
}
