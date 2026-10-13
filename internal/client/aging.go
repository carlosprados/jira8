package client

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/amplia/jira8/internal/models"
)

// AgingRequest selects the cards BoardAging reports.
type AgingRequest struct {
	Board       string
	Project     string
	Column      string        // only this in-progress column; empty = all of them
	Assignee    string        // only this user's cards ("me" = current user)
	OlderThan   time.Duration // only cards this long in their column; 0 = all
	StartColumn string        // where work starts; empty = first in-progress column
	Now         time.Time     // zero = time.Now()
}

// AgingReport is the work in progress of a board with its age.
type AgingReport struct {
	Board       models.Board `json:"board"`
	At          time.Time    `json:"at"`
	StartColumn string       `json:"startColumn"`
	Columns     []string     `json:"columns"`
	Cards       []AgingCard  `json:"cards"`
}

// AgingCard is one in-progress card. InColumnDays counts from when it last
// entered its column (status changes inside the column do not reset it);
// AgeDays from when it first reached the start column or a later one, as
// cycle time does in BoardFlow (nil when it never did, e.g. a column before
// the start one).
type AgingCard struct {
	Key           string     `json:"key"`
	Summary       string     `json:"summary"`
	Column        string     `json:"column"`
	Status        string     `json:"status"`
	Type          string     `json:"type,omitempty"`
	Priority      string     `json:"priority,omitempty"`
	Assignee      string     `json:"assignee,omitempty"`     // username
	AssigneeName  string     `json:"assigneeName,omitempty"` // display name
	InColumnSince time.Time  `json:"inColumnSince"`
	InColumnDays  float64    `json:"inColumnDays"`
	Started       *time.Time `json:"started,omitempty"`
	AgeDays       *float64   `json:"ageDays,omitempty"`
	URL           string     `json:"url"`
}

// BoardAging reads the cards of a board's in-progress columns with their
// history, as the board shows them (filter and kanban sub-filter), and says
// how long each has been in its column and since its work started. Cards
// come oldest in their column first.
func (c *Client) BoardAging(ctx context.Context, req AgingRequest) (*AgingReport, error) {
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
	var cols []models.BoardColumn
	for _, col := range cfg.ColumnConfig.Columns {
		if len(col.Statuses) > 0 {
			cols = append(cols, col)
		}
	}
	categories, err := c.StatusCategories(ctx)
	if err != nil {
		return nil, err
	}
	start, err := startColumn(cols, categories, req.StartColumn)
	if err != nil {
		return nil, fmt.Errorf("board %q: %w", board.Name, err)
	}
	colOf := map[string]int{}
	for i := range cols {
		for _, st := range cols[i].Statuses {
			colOf[st.ID] = i
		}
	}

	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}
	report := &AgingReport{Board: *board, At: now, StartColumn: cols[start].Name}
	scope := boardScope(cfg, req.Assignee)
	var names []string
	for i := range cols {
		col := &cols[i]
		if !inFlight(col, categories) {
			continue
		}
		names = append(names, col.Name)
		if req.Column != "" && !strings.EqualFold(col.Name, req.Column) {
			continue
		}
		report.Columns = append(report.Columns, col.Name)
		issues, err := c.searchWithChangelog(ctx, scope+" AND status IN ("+quoteJoin(columnStatusIDs(col))+") ORDER BY key ASC")
		if err != nil {
			return nil, fmt.Errorf("column %q: %w", col.Name, err)
		}
		for _, is := range issues {
			card, ok := agingCard(is, colOf, i, start, now)
			if !ok || card.InColumnDays*24 < req.OlderThan.Hours() {
				continue
			}
			card.Column = col.Name
			report.Cards = append(report.Cards, card)
		}
	}
	if req.Column != "" && len(report.Columns) == 0 {
		return nil, fmt.Errorf("column %q is not an in-progress column of board %q; those are: %s", req.Column, board.Name, strings.Join(names, ", "))
	}
	order := map[string]int{}
	for i, n := range report.Columns {
		order[n] = i
	}
	slices.SortStableFunc(report.Cards, func(a, b AgingCard) int {
		if d := order[a.Column] - order[b.Column]; d != 0 {
			return d
		}
		return a.InColumnSince.Compare(b.InColumnSince)
	})
	return report, nil
}

// agingCard dates an issue now in column col: when it entered that column
// for the last time, and when it first reached column start or a later one.
func agingCard(is models.Issue, colOf map[string]int, col, start int, now time.Time) (AgingCard, bool) {
	timeline, ok := statusTimeline(is)
	if !ok {
		return AgingCard{}, false
	}
	card := AgingCard{Key: is.Key, Summary: is.Fields.Summary, URL: is.URL}
	if s := is.Fields.Status; s != nil {
		card.Status = s.Name
	}
	if t := is.Fields.IssueType; t != nil {
		card.Type = t.Name
	}
	if p := is.Fields.Priority; p != nil {
		card.Priority = p.Name
	}
	if u := is.Fields.Assignee; u != nil {
		card.Assignee, card.AssigneeName = u.Name, u.DisplayName
	}
	prev := -1
	for _, ch := range timeline {
		c, on := colOf[ch.To]
		if !on {
			c = -1
		}
		if c == col && prev != col {
			card.InColumnSince = ch.At
		}
		if card.Started == nil && on && c >= start {
			t := ch.At
			card.Started = &t
		}
		prev = c
	}
	if card.InColumnSince.IsZero() {
		// The history does not show it entering the column (status changed
		// by an admin, or a workflow migration): count from its last change.
		card.InColumnSince = timeline[len(timeline)-1].At
	}
	card.InColumnDays = round1(days(now.Sub(card.InColumnSince)))
	if card.Started != nil {
		age := round1(days(now.Sub(*card.Started)))
		card.AgeDays = &age
	}
	return card, true
}
