package issue

import (
	"context"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/markup"
	"github.com/amplia/jira8/internal/models"
	"github.com/spf13/cobra"
)

var worklogAddCmd = &cobra.Command{
	Use:     "worklog-add ISSUE-KEY",
	Aliases: []string{"wla"},
	Short:   "Add a worklog entry to an issue",
	Example: `  jira8 issue worklog-add ESA-123 --time 2h --comment "Sprint review"
  jira8 issue worklog-add ESA-123 --time 30m --date "2026-04-07T09:00:00.000+0200"`,
	Args: cobra.ExactArgs(1),
	RunE: runWorklogAdd,
}

func init() {
	worklogAddCmd.Flags().String("time", "", "Time spent (e.g., 2h, 30m, 1d) (required)")
	worklogAddCmd.Flags().String("date", "", "Start date/time in ISO 8601 (optional, defaults to now)")
	worklogAddCmd.Flags().String("comment", "", "Worklog comment (optional)")
	worklogAddCmd.Flags().String("comment-file", "", "Read worklog comment from file (use - for stdin)")
	worklogAddCmd.Flags().Bool("markdown", false, "Treat --comment as Markdown and convert to Jira Wiki Markup before sending")
	_ = worklogAddCmd.MarkFlagRequired("time")
}

func runWorklogAdd(cmd *cobra.Command, args []string) error {
	a := app.Get()
	key := args[0]

	timeSpent, _ := cmd.Flags().GetString("time")
	started, _ := cmd.Flags().GetString("date")
	comment, _, err := app.ReadTextInput(cmd, "comment", "comment-file")
	if err != nil {
		return err
	}
	if md, _ := cmd.Flags().GetBool("markdown"); md && comment != "" {
		comment = markup.MarkdownToWiki(comment)
	}

	req := &models.AddWorklogRequest{
		TimeSpent: timeSpent,
	}
	if started != "" {
		req.Started = started
	}
	if comment != "" {
		req.Comment = comment
	}

	wl, err := a.Client.AddWorklog(context.Background(), key, req)
	if err != nil {
		return err
	}

	if a.Output == "json" {
		return app.OutputJSON(wl)
	}

	fmt.Printf("Worklog added to %s: %s (ID: %s)\n", key, wl.TimeSpent, wl.ID)
	return nil
}
