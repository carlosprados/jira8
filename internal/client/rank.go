package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/amplia/jira8/internal/models"
)

// maxRankIssues is the number of issues Jira accepts in a single rank call.
const maxRankIssues = 50

// RankPosition is where the moved issues should end up.
type RankPosition string

const (
	// RankTop moves the issues to the first position of their own board column.
	RankTop RankPosition = "top"
	// RankBottom moves the issues to the last position of their own board column.
	RankBottom RankPosition = "bottom"
	// RankBefore moves the issues immediately above an explicit anchor issue.
	RankBefore RankPosition = "before"
	// RankAfter moves the issues immediately below an explicit anchor issue.
	RankAfter RankPosition = "after"
)

// RankRequest describes a ranking operation. Ranking changes only the vertical
// order of a board column; it never changes an issue's status (that is what a
// transition does).
type RankRequest struct {
	// Keys are the issues to move, at most maxRankIssues. They keep their
	// relative order.
	Keys []string
	// Position selects the ranking strategy.
	Position RankPosition
	// Anchor is the reference issue key, required by RankBefore / RankAfter and
	// ignored by RankTop / RankBottom (which resolve their own anchor).
	Anchor string
	// Board is an optional board ID or name, only consulted by RankTop /
	// RankBottom. When empty, the issue's project must have exactly one board.
	Board string
}

// RankResult reports what a ranking operation did, including the anchor it
// resolved, so callers can explain the move to the user.
type RankResult struct {
	Issues   []string      `json:"issues"`
	Position RankPosition  `json:"position"`
	Anchor   string        `json:"anchor,omitempty"`
	Board    *models.Board `json:"board,omitempty"`
	Column   string        `json:"column,omitempty"`
	// NoOp is true when the issues already occupied the requested edge of the
	// column and no request was sent.
	NoOp bool `json:"noop"`
}

// GetBoards returns the Agile boards visible to the caller, optionally scoped to
// a project key or ID. Results are paginated with the Agile API's isLast flag.
func (c *Client) GetBoards(ctx context.Context, projectKeyOrID string) ([]models.Board, error) {
	var all []models.Board
	startAt := 0

	for {
		path := fmt.Sprintf("/board?startAt=%d&maxResults=%d", startAt, pageSize)
		if projectKeyOrID != "" {
			path += "&projectKeyOrId=" + url.QueryEscape(projectKeyOrID)
		}

		data, err := c.doAgile(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}

		var resp models.BoardsResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, fmt.Errorf("parsing boards: %w", err)
		}

		all = append(all, resp.Values...)
		if resp.IsLast || len(resp.Values) == 0 {
			break
		}
		startAt += len(resp.Values)
	}

	return all, nil
}

// GetBoardConfiguration returns a board's saved filter, column layout and rank
// field. Needed to work out which column an issue currently sits in.
func (c *Client) GetBoardConfiguration(ctx context.Context, boardID int) (*models.BoardConfiguration, error) {
	path := fmt.Sprintf("/board/%d/configuration", boardID)
	data, err := c.doAgile(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var cfg models.BoardConfiguration
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing board configuration: %w", err)
	}
	return &cfg, nil
}

// RankIssues performs the raw rank call (PUT /rest/agile/1.0/issue/rank).
// Prefer RankIssuesRelative, which resolves anchors and validates the request.
func (c *Client) RankIssues(ctx context.Context, req *models.RankIssuesRequest) error {
	_, err := c.doAgile(ctx, http.MethodPut, "/issue/rank", req)
	return err
}

// RankIssuesRelative moves issues within the board's rank order. For RankTop and
// RankBottom it resolves the issue's own column on the board and picks the first
// (or last) issue of that column as the anchor, so "top" means "top of the
// column the issue is already in". Shared by the CLI `issue rank` command and
// the jira_rank_issue MCP tool so both resolve boards and columns identically.
func (c *Client) RankIssuesRelative(ctx context.Context, req RankRequest) (*RankResult, error) {
	if len(req.Keys) == 0 {
		return nil, fmt.Errorf("no issues to rank")
	}
	if len(req.Keys) > maxRankIssues {
		return nil, fmt.Errorf("cannot rank more than %d issues in one call (got %d)", maxRankIssues, len(req.Keys))
	}

	result := &RankResult{Issues: req.Keys, Position: req.Position}
	var rankFieldID int

	switch req.Position {
	case RankBefore, RankAfter:
		if req.Anchor == "" {
			return nil, fmt.Errorf("position %q requires an anchor issue", req.Position)
		}
		if containsFold(req.Keys, req.Anchor) {
			return nil, fmt.Errorf("cannot rank %s relative to itself", req.Anchor)
		}
		id, err := c.ResolveRankField(ctx)
		if err != nil {
			return nil, err
		}
		rankFieldID = id
		result.Anchor = req.Anchor

	case RankTop, RankBottom:
		id, err := c.resolveColumnAnchor(ctx, req, result)
		if err != nil {
			return nil, err
		}
		rankFieldID = id

	default:
		return nil, fmt.Errorf("unknown rank position %q", req.Position)
	}

	if result.NoOp {
		return result, nil
	}

	body := &models.RankIssuesRequest{Issues: req.Keys, RankCustomFieldID: rankFieldID}
	if req.Position == RankAfter || req.Position == RankBottom {
		body.RankAfterIssue = result.Anchor
	} else {
		body.RankBeforeIssue = result.Anchor
	}
	if err := c.RankIssues(ctx, body); err != nil {
		return nil, err
	}
	return result, nil
}

// resolveColumnAnchor fills result.Board, result.Column and result.Anchor for a
// top/bottom move, and returns the rank custom field ID to use. It sets
// result.NoOp when the moved issues already sit at the requested edge.
func (c *Client) resolveColumnAnchor(ctx context.Context, req RankRequest, result *RankResult) (int, error) {
	// The first key decides the project and the column; ranking a block of
	// issues that live in different columns has no single meaning.
	issue, err := c.GetIssue(ctx, req.Keys[0])
	if err != nil {
		return 0, err
	}
	if issue.Fields.Status == nil {
		return 0, fmt.Errorf("cannot determine the status of %s", req.Keys[0])
	}
	statusID, statusName := issue.Fields.Status.ID, issue.Fields.Status.Name

	projectKey := ""
	if issue.Fields.Project != nil {
		projectKey = issue.Fields.Project.Key
	}

	board, err := c.resolveBoard(ctx, projectKey, req.Board)
	if err != nil {
		return 0, err
	}

	cfg, err := c.GetBoardConfiguration(ctx, board.ID)
	if err != nil {
		return 0, err
	}
	if cfg.Name != "" {
		board.Name = cfg.Name
	}
	if cfg.Type != "" {
		board.Type = cfg.Type
	}
	result.Board = board

	column := findColumnForStatus(cfg.ColumnConfig.Columns, statusID)
	if column == nil {
		return 0, fmt.Errorf("status %q of %s is not mapped to any column of board %q; the issue is not shown on that board",
			statusName, req.Keys[0], board.Name)
	}
	result.Column = column.Name

	rankFieldID := 0
	if cfg.Ranking != nil {
		rankFieldID = cfg.Ranking.RankCustomFieldID
	}
	if rankFieldID == 0 {
		// Some boards report an empty "ranking" object; fall back to the
		// instance-wide LexoRank field.
		if rankFieldID, err = c.ResolveRankField(ctx); err != nil {
			return 0, err
		}
	}

	if cfg.Filter.ID == "" {
		return 0, fmt.Errorf("board %q does not expose its saved filter, cannot determine column order", board.Name)
	}

	order := "ASC"
	if req.Position == RankBottom {
		order = "DESC"
	}
	jql := fmt.Sprintf("filter = %s AND status IN (%s) ORDER BY cf[%d] %s",
		cfg.Filter.ID, quoteJoin(columnStatusIDs(column)), rankFieldID, order)

	// Fetching len(Keys)+1 issues guarantees at least one candidate that is not
	// itself being moved, if such an issue exists at all.
	search, err := c.SearchIssues(ctx, jql, 0, len(req.Keys)+1)
	if err != nil {
		return 0, fmt.Errorf("resolving the %s of column %q: %w", req.Position, column.Name, err)
	}

	for _, candidate := range search.Issues {
		if !containsFold(req.Keys, candidate.Key) {
			result.Anchor = candidate.Key
			return rankFieldID, nil
		}
	}

	// Every issue in the column is part of the move: nothing to rank against.
	result.NoOp = true
	return rankFieldID, nil
}

// resolveBoard picks the board for a top/bottom move. A numeric ref is taken as
// a board ID; a non-numeric ref is matched by name against the project's boards.
// With no ref, the project must have exactly one board — otherwise the error
// lists the candidates so the user can pass --board.
func (c *Client) resolveBoard(ctx context.Context, projectKey, ref string) (*models.Board, error) {
	if id, err := strconv.Atoi(strings.TrimSpace(ref)); err == nil && id > 0 {
		return &models.Board{ID: id}, nil
	}

	if projectKey == "" {
		return nil, fmt.Errorf("cannot determine the project of the issue; pass a board ID explicitly")
	}

	boards, err := c.GetBoards(ctx, projectKey)
	if err != nil {
		return nil, err
	}
	if len(boards) == 0 {
		return nil, fmt.Errorf("project %s has no Agile boards", projectKey)
	}

	if ref != "" {
		for i, b := range boards {
			if strings.EqualFold(b.Name, ref) {
				return &boards[i], nil
			}
		}
		return nil, fmt.Errorf("board %q not found in project %s; available: %s", ref, projectKey, describeBoards(boards))
	}

	if len(boards) > 1 {
		return nil, fmt.Errorf("project %s has %d boards, specify one: %s", projectKey, len(boards), describeBoards(boards))
	}
	return &boards[0], nil
}

// findColumnForStatus returns the column whose status set contains statusID.
func findColumnForStatus(columns []models.BoardColumn, statusID string) *models.BoardColumn {
	for i, col := range columns {
		for _, st := range col.Statuses {
			if st.ID == statusID {
				return &columns[i]
			}
		}
	}
	return nil
}

// columnStatusIDs returns the status IDs mapped to a column.
func columnStatusIDs(col *models.BoardColumn) []string {
	out := make([]string, 0, len(col.Statuses))
	for _, st := range col.Statuses {
		out = append(out, st.ID)
	}
	return out
}

// describeBoards renders boards as `name (id, type)` for error messages.
func describeBoards(boards []models.Board) string {
	out := make([]string, 0, len(boards))
	for _, b := range boards {
		out = append(out, fmt.Sprintf("%q (%d, %s)", b.Name, b.ID, b.Type))
	}
	return strings.Join(out, ", ")
}

// quoteJoin renders values as a quoted, comma-separated JQL list.
func quoteJoin(values []string) string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, `"`+v+`"`)
	}
	return strings.Join(out, ", ")
}

// containsFold reports whether values contains target, ignoring case (Jira issue
// keys are case-insensitive).
func containsFold(values []string, target string) bool {
	for _, v := range values {
		if strings.EqualFold(v, target) {
			return true
		}
	}
	return false
}
