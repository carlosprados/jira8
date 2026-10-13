package client

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/amplia/jira8/internal/models"
)

// FlowRequest selects the board and the period BoardFlow measures.
type FlowRequest struct {
	Board       string
	Project     string
	Since       time.Duration // finished within this long before now
	StartColumn string        // column where the cycle starts; empty = first in-progress column
	Now         time.Time     // zero = time.Now()
}

// FlowReport is how work went through a board: what finished in the period,
// how long it took from the start column to the done column and where that
// time was spent. Durations are calendar days.
type FlowReport struct {
	Board       models.Board `json:"board"`
	From        time.Time    `json:"from"`
	To          time.Time    `json:"to"`
	StartColumn string       `json:"startColumn"`
	DoneColumn  string       `json:"doneColumn"`
	Throughput  int          `json:"throughput"`
	Weekly      []WeekCount  `json:"weekly"`
	CycleTime   Distribution `json:"cycleTime"`
	Columns     []ColumnTime `json:"columns"`
	Issues      []FlowIssue  `json:"issues"`
}

// WeekCount is the throughput of the week starting on Monday Week. Partial
// marks the first and last weeks when the period covers them only in part.
type WeekCount struct {
	Week    string `json:"week"`
	Count   int    `json:"count"`
	Partial bool   `json:"partial,omitempty"`
}

// Distribution summarises durations in days. Count is how many were measured.
type Distribution struct {
	Count int     `json:"count"`
	P50   float64 `json:"p50"`
	P85   float64 `json:"p85"`
	Max   float64 `json:"max"`
}

// ColumnTime is the time finished issues spent in a column during their
// cycle: Days is the sum over all of them and Share its part of the total.
// Columns before the start one appear only when cards went back to them.
type ColumnTime struct {
	Name  string       `json:"name"`
	Days  float64      `json:"days"`
	Share float64      `json:"share"`
	Per   Distribution `json:"perIssue"` // over the issues that went through it
}

// FlowIssue is one finished issue. Started is nil when it skipped every
// column from the start column on (straight from to-do to done).
type FlowIssue struct {
	Key       string     `json:"key"`
	Summary   string     `json:"summary"`
	Type      string     `json:"type,omitempty"`
	Started   *time.Time `json:"started,omitempty"`
	Done      time.Time  `json:"done"`
	CycleDays *float64   `json:"cycleDays,omitempty"`
	URL       string     `json:"url"`
}

const jiraTime = "2006-01-02T15:04:05.000-0700"

// BoardFlow measures the issues of a board that reached its done column (the
// last one) within req.Since. It uses the board's saved filter but not its
// kanban sub-filter, which usually hides older done issues.
func (c *Client) BoardFlow(ctx context.Context, req FlowRequest) (*FlowReport, error) {
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
	if len(cols) < 2 {
		return nil, fmt.Errorf("board %q has fewer than two columns with statuses", board.Name)
	}
	categories, err := c.StatusCategories(ctx)
	if err != nil {
		return nil, err
	}
	start, err := startColumn(cols, categories, req.StartColumn)
	if err != nil {
		return nil, fmt.Errorf("board %q: %w", board.Name, err)
	}
	done := len(cols) - 1
	if start >= done {
		return nil, fmt.Errorf("board %q: the start column must come before %q", board.Name, cols[done].Name)
	}

	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}
	from := now.Add(-req.Since)
	// History clauses read a quoted id as a status name: ids go bare there.
	ids := columnStatusIDs(&cols[done])
	jql := fmt.Sprintf("filter = %s AND status IN (%s) AND status CHANGED TO (%s) AFTER -%dm ORDER BY key ASC",
		cfg.Filter.ID, quoteJoin(ids), strings.Join(ids, ", "), int(req.Since.Minutes())+1)
	issues, err := c.searchWithChangelog(ctx, jql)
	if err != nil {
		return nil, err
	}

	colOf := map[string]int{}
	for i := range cols {
		for _, st := range cols[i].Statuses {
			colOf[st.ID] = i
		}
	}
	report := &FlowReport{Board: *board, From: from, To: now, StartColumn: cols[start].Name, DoneColumn: cols[done].Name}
	spent := make([][]float64, len(cols))
	var cycles []float64
	weeks := map[string]int{}
	for _, is := range issues {
		f, ok := issueFlow(is, colOf, start, done)
		if !ok || f.done.Before(from) {
			continue
		}
		fi := FlowIssue{Key: is.Key, Summary: is.Fields.Summary, Done: f.done, URL: is.URL}
		if t := is.Fields.IssueType; t != nil {
			fi.Type = t.Name
		}
		if f.started != nil {
			d := days(f.done.Sub(*f.started))
			fi.Started, fi.CycleDays = f.started, &d
			cycles = append(cycles, d)
			for i, s := range f.inColumn {
				if s > 0 {
					spent[i] = append(spent[i], days(s))
				}
			}
		}
		report.Issues = append(report.Issues, fi)
		weeks[monday(f.done.In(now.Location()))]++
	}

	report.Throughput = len(report.Issues)
	for w := monday(from); w <= monday(now); w = nextWeek(w) {
		report.Weekly = append(report.Weekly, WeekCount{Week: w, Count: weeks[w]})
	}
	report.Weekly[0].Partial = from.Format("2006-01-02") != report.Weekly[0].Week || from.Hour()+from.Minute() > 0
	report.Weekly[len(report.Weekly)-1].Partial = true // the current week is not over
	report.CycleTime = distribution(cycles)
	var total float64
	for i := 0; i < done; i++ {
		// Before the start column only cards sent back after starting spend time.
		if i < start && len(spent[i]) == 0 {
			continue
		}
		ct := ColumnTime{Name: cols[i].Name, Per: distribution(spent[i])}
		for _, d := range spent[i] {
			ct.Days += d
		}
		total += ct.Days
		report.Columns = append(report.Columns, ct)
	}
	for i := range report.Columns {
		if total > 0 {
			report.Columns[i].Share = round1(100 * report.Columns[i].Days / total)
		}
		report.Columns[i].Days = round1(report.Columns[i].Days)
	}
	slices.SortFunc(report.Issues, func(a, b FlowIssue) int { return a.Done.Compare(b.Done) })
	return report, nil
}

// startColumn is the named column, or the first one holding an in-progress
// status.
func startColumn(cols []models.BoardColumn, categories map[string]string, name string) (int, error) {
	var names []string
	for i := range cols {
		names = append(names, cols[i].Name)
		if name == "" && inFlight(&cols[i], categories) || name != "" && strings.EqualFold(cols[i].Name, name) {
			return i, nil
		}
	}
	if name == "" {
		return 0, fmt.Errorf("no in-progress column: pass the start column; columns: %s", strings.Join(names, ", "))
	}
	return 0, fmt.Errorf("column %q not found; columns: %s", name, strings.Join(names, ", "))
}

type flow struct {
	started  *time.Time
	done     time.Time
	inColumn []time.Duration // time in each column between started and done
}

// issueFlow replays an issue's status changes over the board columns. The
// cycle starts the first time the issue reaches column start or later and
// ends the last time it enters column done. Time in statuses that are not on
// the board counts in no column. ok is false when the issue never entered
// the done column or its history cannot be read.
func issueFlow(is models.Issue, colOf map[string]int, start, done int) (f flow, ok bool) {
	timeline, ok := statusTimeline(is)
	if !ok {
		return f, false
	}

	maxCols := 0
	for _, i := range colOf {
		maxCols = max(maxCols, i+1)
	}
	f.inColumn = make([]time.Duration, maxCols)
	end := -1
	for i, ch := range timeline {
		if col, on := colOf[ch.To]; on && col == done {
			end = i
		}
	}
	if end < 0 {
		return f, false
	}
	f.done = timeline[end].At
	for i := 0; i < end; i++ {
		col, on := colOf[timeline[i].To]
		if f.started == nil && on && col >= start {
			t := timeline[i].At
			f.started = &t
		}
		if f.started != nil && on {
			f.inColumn[col] += timeline[i+1].At.Sub(timeline[i].At)
		}
	}
	return f, true
}

// statusChange is an issue entering status To at At.
type statusChange struct {
	At   time.Time
	To   string
	from string
}

// statusTimeline is an issue's statuses in order, from its creation: the
// first entry is the status it was created in. ok is false when a date
// cannot be read.
func statusTimeline(is models.Issue) ([]statusChange, bool) {
	created, err := time.Parse(jiraTime, is.Fields.Created)
	if err != nil {
		return nil, false
	}
	var changes []statusChange
	if is.Changelog != nil {
		for _, h := range is.Changelog.Histories {
			for _, it := range h.Items {
				if it.Field != "status" {
					continue
				}
				at, err := time.Parse(jiraTime, h.Created)
				if err != nil {
					return nil, false
				}
				changes = append(changes, statusChange{At: at, To: it.To, from: it.From})
			}
		}
	}
	slices.SortStableFunc(changes, func(a, b statusChange) int { return a.At.Compare(b.At) })
	initial := ""
	if len(changes) > 0 {
		initial = changes[0].from
	} else if is.Fields.Status != nil {
		initial = is.Fields.Status.ID
	}
	return append([]statusChange{{At: created, To: initial}}, changes...), true
}

// searchWithChangelog runs a search with each issue's history.
func (c *Client) searchWithChangelog(ctx context.Context, jql string) ([]models.Issue, error) {
	var all []models.Issue
	for startAt := 0; ; {
		path := fmt.Sprintf("/search?jql=%s&startAt=%d&maxResults=%d&fields=summary,status,issuetype,created,assignee,priority&expand=changelog",
			url.QueryEscape(jql), startAt, pageSize)
		data, err := c.do(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}
		var res models.SearchResult
		if err := json.Unmarshal(data, &res); err != nil {
			return nil, fmt.Errorf("parsing search results: %w", err)
		}
		for i := range res.Issues {
			res.Issues[i].URL = c.BrowseURL(res.Issues[i].Key)
			if cl := res.Issues[i].Changelog; cl != nil && len(cl.Histories) < cl.Total {
				return nil, fmt.Errorf("%s: search returned %d of %d history entries", res.Issues[i].Key, len(cl.Histories), cl.Total)
			}
		}
		all = append(all, res.Issues...)
		startAt += len(res.Issues)
		if len(res.Issues) == 0 || startAt >= res.Total {
			return all, nil
		}
	}
}

func distribution(ds []float64) Distribution {
	if len(ds) == 0 {
		return Distribution{}
	}
	s := slices.Sorted(slices.Values(ds))
	return Distribution{Count: len(s), P50: round1(percentile(s, 50)), P85: round1(percentile(s, 85)), Max: round1(s[len(s)-1])}
}

// percentile of sorted values, nearest-rank: p85 = 85% of them take that long or less.
func percentile(sorted []float64, p float64) float64 {
	rank := int(math.Ceil(p / 100 * float64(len(sorted))))
	return sorted[max(rank, 1)-1]
}

func days(d time.Duration) float64 { return d.Hours() / 24 }

func round1(f float64) float64 { return math.Round(f*10) / 10 }

// monday is the date of the Monday starting t's week, in t's location.
func monday(t time.Time) string {
	wd := (int(t.Weekday()) + 6) % 7
	return t.AddDate(0, 0, -wd).Format("2006-01-02")
}

func nextWeek(w string) string {
	t, _ := time.Parse("2006-01-02", w)
	return t.AddDate(0, 0, 7).Format("2006-01-02")
}
