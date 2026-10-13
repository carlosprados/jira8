package board

import (
	"context"
	"fmt"
	"strings"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/client"
	"github.com/amplia/jira8/internal/models"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// BoardCmd is the parent command for Agile boards.
var BoardCmd = &cobra.Command{
	Use:   "board",
	Short: "Agile boards: list them and view a kanban board column by column",
}

var listCmd = &cobra.Command{
	Use:     "list",
	Short:   "List the boards of a project (all visible boards without a project)",
	Example: "  jira8 board list --project ABC",
	Args:    cobra.NoArgs,
	RunE:    runList,
}

var viewCmd = &cobra.Command{
	Use:   "view [BOARD]",
	Short: "Show a board's columns with their issues in board order and WIP limits",
	Long: `Shows the board as Jira draws it: its columns left to right, each with its
issues in rank order (top first), the issue count and the WIP limits, flagging
the columns over the maximum or under the minimum. It uses the board's own
filter and kanban sub-filter, so old Done issues stay hidden as on the board.

BOARD is a board id or name ('jira8 board list'); without it the project must
have a single board. --max limits the issues listed per column (0 = counts
only); the counts always cover the whole column.`,
	Example: `  jira8 board view "Team Kanban" --project ABC
  jira8 board view 110 --column "In Progress"
  jira8 board view 110 --assignee me --max 50 -o json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runView,
}

func init() {
	listCmd.Flags().String("project", "", "Project key (default from config)")
	viewCmd.Flags().String("project", "", "Project key, to find the board by name (default from config)")
	viewCmd.Flags().String("column", "", "Only this column")
	viewCmd.Flags().String("assignee", "", "Only issues of this user ('me' for you)")
	viewCmd.Flags().Int("max", 10, "Issues listed per column (0 = counts only)")
	BoardCmd.AddCommand(listCmd, viewCmd)
}

var (
	headerStyle = lipgloss.NewStyle().Bold(true)
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	warnStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1"))
)

func runList(cmd *cobra.Command, _ []string) error {
	project, _ := cmd.Flags().GetString("project")
	if project == "" {
		project = app.Get().Config.Project
	}
	boards, err := app.Get().Client.GetBoards(context.Background(), project)
	if err != nil {
		return err
	}
	if app.Get().Output == "json" {
		return app.OutputJSON(boards)
	}
	if len(boards) == 0 {
		fmt.Println("No boards.")
		return nil
	}
	for _, b := range boards {
		fmt.Printf("%-6d %-8s %s\n", b.ID, b.Type, b.Name)
	}
	return nil
}

// issueRow is an issue as board view reports it in JSON.
type issueRow struct {
	Key      string `json:"key"`
	Summary  string `json:"summary"`
	Type     string `json:"type,omitempty"`
	Status   string `json:"status,omitempty"`
	Priority string `json:"priority,omitempty"`
	Assignee string `json:"assignee,omitempty"`
	URL      string `json:"url"`
}

type columnOut struct {
	client.BoardColumn
	Issues []issueRow `json:"issues"`
}

func runView(cmd *cobra.Command, args []string) error {
	req := client.BoardViewRequest{}
	if len(args) == 1 {
		req.Board = args[0]
	}
	req.Project, _ = cmd.Flags().GetString("project")
	if req.Project == "" {
		req.Project = app.Get().Config.Project
	}
	req.Column, _ = cmd.Flags().GetString("column")
	req.Assignee, _ = cmd.Flags().GetString("assignee")
	req.Max, _ = cmd.Flags().GetInt("max")

	view, err := app.Get().Client.BoardView(context.Background(), req)
	if err != nil {
		return err
	}

	cols := make([]columnOut, len(view.Columns))
	for i, c := range view.Columns {
		cols[i] = columnOut{BoardColumn: c, Issues: make([]issueRow, len(c.Issues))}
		for j, is := range c.Issues {
			cols[i].Issues[j] = row(is)
		}
	}
	if app.Get().Output == "json" {
		return app.OutputJSON(map[string]any{"board": view.Board, "columns": cols})
	}

	fmt.Printf("%s %s\n", headerStyle.Render(view.Board.Name), dimStyle.Render(fmt.Sprintf("(%d, %s)", view.Board.ID, view.Board.Type)))
	if req.Assignee != "" {
		fmt.Println(dimStyle.Render("Only issues assigned to " + req.Assignee + ": WIP limits are not checked."))
	}
	for _, c := range cols {
		fmt.Printf("\n%s  %s\n", headerStyle.Render(c.Name), columnCount(c.BoardColumn))
		for _, r := range c.Issues {
			fmt.Printf("  %-11s %-9s %-20s %s\n", r.Key, trunc(r.Priority, 9), trunc(orDash(r.Assignee), 20), trunc(r.Summary, 70))
		}
		if more := c.Total - len(c.Issues); more > 0 && len(c.Issues) > 0 {
			fmt.Println(dimStyle.Render(fmt.Sprintf("  … %d more", more)))
		}
	}
	return nil
}

// columnCount renders "12 (min 3, max 10) OVER MAX".
func columnCount(c client.BoardColumn) string {
	s := fmt.Sprint(c.Total)
	var limits []string
	if c.Min > 0 {
		limits = append(limits, fmt.Sprintf("min %d", c.Min))
	}
	if c.Max > 0 {
		limits = append(limits, fmt.Sprintf("max %d", c.Max))
	}
	if len(limits) > 0 {
		if c.WIP != c.Total {
			limits = append([]string{fmt.Sprintf("%d without sub-tasks", c.WIP)}, limits...)
		}
		s += dimStyle.Render(" (" + strings.Join(limits, ", ") + ")")
	}
	if c.Over != "" {
		s += " " + warnStyle.Render("OVER "+strings.ToUpper(c.Over))
	}
	return s
}

func row(is models.Issue) issueRow {
	r := issueRow{Key: is.Key, Summary: is.Fields.Summary, URL: is.URL}
	if t := is.Fields.IssueType; t != nil {
		r.Type = t.Name
	}
	if s := is.Fields.Status; s != nil {
		r.Status = s.Name
	}
	if p := is.Fields.Priority; p != nil {
		r.Priority = p.Name
	}
	if u := is.Fields.Assignee; u != nil {
		r.Assignee = u.DisplayName
	}
	return r
}

func trunc(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
