package board

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/client"
)

// assigneeWIP is one person's share of the in-progress columns.
type assigneeWIP struct {
	Assignee string         `json:"assignee"` // username; empty = unassigned
	Name     string         `json:"name"`
	Total    int            `json:"total"`
	Stale    int            `json:"stale,omitempty"`
	Columns  map[string]int `json:"columns"`
	Keys     []string       `json:"keys"`
}

func printByAssignee(view *client.BoardView) error {
	byUser := map[string]*assigneeWIP{}
	var names []string
	stale := false
	for _, c := range view.Columns {
		names = append(names, c.Name)
		stale = stale || c.Stale != nil
		for _, is := range c.Issues {
			user, name := "", "(unassigned)"
			if u := is.Fields.Assignee; u != nil {
				user, name = u.Name, u.DisplayName
			}
			w := byUser[user]
			if w == nil {
				w = &assigneeWIP{Assignee: user, Name: name, Columns: map[string]int{}}
				byUser[user] = w
			}
			w.Total++
			w.Columns[c.Name]++
			w.Keys = append(w.Keys, is.Key)
			if c.StaleKeys[is.Key] {
				w.Stale++
			}
		}
	}
	rows := make([]*assigneeWIP, 0, len(byUser))
	for _, w := range byUser {
		rows = append(rows, w)
	}
	slices.SortFunc(rows, func(a, b *assigneeWIP) int {
		return cmp.Or(cmp.Compare(b.Total, a.Total), cmp.Compare(a.Name, b.Name))
	})

	if app.Get().Output == "json" {
		return app.OutputJSON(map[string]any{"board": view.Board, "columns": names, "assignees": rows})
	}
	fmt.Printf("%s %s\n", headerStyle.Render(view.Board.Name), dimStyle.Render(fmt.Sprintf("(%d, %s)", view.Board.ID, view.Board.Type)))
	if len(rows) == 0 {
		fmt.Println("No work in progress.")
		return nil
	}
	width := 6
	for _, n := range names {
		width = max(width, len([]rune(n)))
	}
	header := fmt.Sprintf("%-22s %5s", "ASSIGNEE", "TOTAL")
	if stale {
		header += fmt.Sprintf(" %5s", "STALE")
	}
	for _, n := range names {
		header += fmt.Sprintf("  %*s", width, n)
	}
	fmt.Println(headerStyle.Render(header))
	for _, w := range rows {
		line := fmt.Sprintf("%-22s %5d", trunc(w.Name, 22), w.Total)
		if stale {
			s := fmt.Sprintf(" %5d", w.Stale)
			if w.Stale > 0 {
				s = warnStyle.Render(s)
			} else {
				s = dimStyle.Render(s)
			}
			line += s
		}
		for _, n := range names {
			line += fmt.Sprintf("  %*s", width, countOrDot(w.Columns[n]))
		}
		fmt.Println(strings.TrimRight(line, " "))
	}
	return nil
}

func countOrDot(n int) string {
	if n == 0 {
		return "-"
	}
	return fmt.Sprint(n)
}
