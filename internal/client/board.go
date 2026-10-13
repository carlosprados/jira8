package client

import (
	"context"
	"fmt"
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
	Name   string         `json:"name"`
	Total  int            `json:"total"`
	Min    int            `json:"min,omitempty"`
	Max    int            `json:"max,omitempty"`
	WIP    int            `json:"wip"`
	Over   string         `json:"over,omitempty"`
	Issues []models.Issue `json:"-"`
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
