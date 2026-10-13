package board

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/client"
	"github.com/spf13/cobra"
)

var flowCmd = &cobra.Command{
	Use:   "flow [BOARD]",
	Short: "Flow metrics of a kanban board: throughput, cycle time and time per column",
	Long: `Measures the issues that reached the board's done column (the last one) in
the period:

  throughput   how many, in total and per week (weeks start on Monday)
  cycle time   days from the start column to done: p50, p85 (85% took that
               long or less) and max
  per column   where that time went: days summed over the issues, share of
               the total and p50/p85 per issue

The cycle starts the first time an issue enters the start column or any column
after it; by default the start column is the first one holding an in-progress
status. It ends the last time the issue entered done. Days are calendar days.
Issues that went straight from to-do to done count in the throughput only.

It reads the board's saved filter without its kanban sub-filter (which tends
to hide older done issues) and each issue's history, so long periods on busy
boards take a while.`,
	Example: `  jira8 board flow 110
  jira8 board flow "Team Kanban" --project ABC --since 90d
  jira8 board flow 110 --start-column "In Progress" -o json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runFlow,
}

func init() {
	flowCmd.Flags().String("project", "", "Project key, to find the board by name (default from config)")
	flowCmd.Flags().String("since", "30d", "Period to measure, back from now (30d, 12w)")
	flowCmd.Flags().String("start-column", "", "Column where the cycle starts (default: first in-progress column)")
	flowCmd.Flags().Bool("issues", false, "Also list the finished issues with their cycle time")
}

func runFlow(cmd *cobra.Command, args []string) error {
	req := client.FlowRequest{}
	if len(args) == 1 {
		req.Board = args[0]
	}
	req.Project, _ = cmd.Flags().GetString("project")
	if req.Project == "" {
		req.Project = app.Get().Config.Project
	}
	req.StartColumn, _ = cmd.Flags().GetString("start-column")
	since, _ := cmd.Flags().GetString("since")
	d, err := periodDuration(since)
	if err != nil {
		return fmt.Errorf("--since %q: %w", since, err)
	}
	req.Since = d

	r, err := app.Get().Client.BoardFlow(context.Background(), req)
	if err != nil {
		return err
	}
	if app.Get().Output == "json" {
		return app.OutputJSON(r)
	}

	fmt.Printf("%s %s\n", headerStyle.Render(r.Board.Name), dimStyle.Render(fmt.Sprintf("(%d, %s)", r.Board.ID, r.Board.Type)))
	fmt.Printf("%s → %s, cycle %s → %s\n\n", r.From.Format("2006-01-02"), r.To.Format("2006-01-02"), r.StartColumn, r.DoneColumn)

	fmt.Printf("%s  %d done", headerStyle.Render("Throughput"), r.Throughput)
	if skipped := r.Throughput - r.CycleTime.Count; skipped > 0 {
		fmt.Print(dimStyle.Render(fmt.Sprintf(" (%d never went through %s or later)", skipped, r.StartColumn)))
	}
	fmt.Println()
	for _, w := range r.Weekly {
		bar := strings.Repeat("▇", w.Count)
		if w.Partial {
			bar += dimStyle.Render(" (partial week)")
		}
		fmt.Printf("  %s  %3d  %s\n", w.Week, w.Count, bar)
	}

	fmt.Printf("\n%s  ", headerStyle.Render("Cycle time"))
	if r.CycleTime.Count == 0 {
		fmt.Println("nothing measured")
		return nil
	}
	fmt.Printf("p50 %s  p85 %s  max %s  %s\n", dayStr(r.CycleTime.P50), dayStr(r.CycleTime.P85), dayStr(r.CycleTime.Max),
		dimStyle.Render(fmt.Sprintf("(%d issues)", r.CycleTime.Count)))

	fmt.Printf("\n%s\n", headerStyle.Render("Time per column"))
	width := 6
	for _, c := range r.Columns {
		width = max(width, len([]rune(c.Name)))
	}
	fmt.Println(dimStyle.Render(fmt.Sprintf("  %-*s %6s %8s %8s %8s", width, "", "share", "days", "p50", "p85")))
	for _, c := range r.Columns {
		fmt.Printf("  %-*s %5.0f%% %8.1f %8s %8s\n", width, c.Name, c.Share, c.Days, dayStr(c.Per.P50), dayStr(c.Per.P85))
	}

	if all, _ := cmd.Flags().GetBool("issues"); all {
		fmt.Printf("\n%s\n", headerStyle.Render("Issues"))
		for _, is := range r.Issues {
			cycle := "-"
			if is.CycleDays != nil {
				cycle = dayStr(*is.CycleDays)
			}
			fmt.Printf("  %-11s %s %7s  %s\n", is.Key, is.Done.Local().Format("2006-01-02"), cycle, trunc(is.Summary, 70))
		}
	}
	return nil
}

func dayStr(d float64) string {
	return strconv.FormatFloat(d, 'f', 1, 64) + "d"
}

// periodDuration turns "30d", "12w" or "36h" into a duration.
func periodDuration(s string) (time.Duration, error) {
	m := periodRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil || m[1] == "0" {
		return 0, fmt.Errorf("use a period like 30d, 12w or 36h")
	}
	n, _ := strconv.Atoi(m[1])
	unit := map[string]time.Duration{"m": time.Minute, "h": time.Hour, "d": 24 * time.Hour, "w": 7 * 24 * time.Hour}[m[2]]
	return time.Duration(n) * unit, nil
}
