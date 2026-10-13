package board

import (
	"context"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/client"
	"github.com/spf13/cobra"
)

var agingCmd = &cobra.Command{
	Use:   "aging [BOARD]",
	Short: "How long each in-progress card has been in its column and since its work started",
	Long: `Lists the cards of the board's in-progress columns, oldest in its column
first, with two ages in calendar days:

  in column   since the card last entered its current column (status changes
              inside the column do not reset it)
  age         since its work started: the first time it reached the start
              column or a later one, as in 'board flow' (by default the first
              in-progress column; --start-column changes it)

Compare the age with the cycle time p85 of 'board flow': a card older than
that is already slower than 85% of the finished ones. It reads each card's
history, through the board's filter and kanban sub-filter.`,
	Example: `  jira8 board aging 110
  jira8 board aging 110 --column "In Progress" --older-than 14d
  jira8 board aging "Team Kanban" --project ABC --assignee me -o json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAging,
}

func init() {
	agingCmd.Flags().String("project", "", "Project key, to find the board by name (default from config)")
	agingCmd.Flags().String("column", "", "Only this in-progress column")
	agingCmd.Flags().String("assignee", "", "Only cards of this user ('me' for you)")
	agingCmd.Flags().String("older-than", "", "Only cards in their column for at least this long (14d, 2w)")
	agingCmd.Flags().String("start-column", "", "Column where work starts (default: first in-progress column)")
}

func runAging(cmd *cobra.Command, args []string) error {
	str := func(name string) string { v, _ := cmd.Flags().GetString(name); return v }
	req := client.AgingRequest{Project: str("project"), Column: str("column"), Assignee: str("assignee"), StartColumn: str("start-column")}
	if len(args) == 1 {
		req.Board = args[0]
	}
	if req.Project == "" {
		req.Project = app.Get().Config.Project
	}
	if v := str("older-than"); v != "" {
		d, err := periodDuration(v)
		if err != nil {
			return fmt.Errorf("--older-than %q: %w", v, err)
		}
		req.OlderThan = d
	}

	r, err := app.Get().Client.BoardAging(context.Background(), req)
	if err != nil {
		return err
	}
	if app.Get().Output == "json" {
		return app.OutputJSON(r)
	}

	fmt.Printf("%s %s\n", headerStyle.Render(r.Board.Name), dimStyle.Render(fmt.Sprintf("(%d, %s) · work starts in %s", r.Board.ID, r.Board.Type, r.StartColumn)))
	for _, col := range r.Columns {
		var n int
		for _, c := range r.Cards {
			if c.Column == col {
				n++
			}
		}
		fmt.Printf("\n%s  %d\n", headerStyle.Render(col), n)
		if n == 0 {
			continue
		}
		fmt.Println(dimStyle.Render(fmt.Sprintf("  %-11s %9s %7s  %-20s %s", "", "in column", "age", "assignee", "summary")))
		for _, c := range r.Cards {
			if c.Column != col {
				continue
			}
			age := "-"
			if c.AgeDays != nil {
				age = dayStr(*c.AgeDays)
			}
			fmt.Printf("  %-11s %9s %7s  %-20s %s\n", c.Key, dayStr(c.InColumnDays), age, trunc(orDash(c.AssigneeName), 20), trunc(c.Summary, 60))
		}
	}
	return nil
}
