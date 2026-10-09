package issue

import (
	"context"

	"github.com/amplia/jira8/cmd/app"
	"github.com/spf13/cobra"
)

var viewCmd = &cobra.Command{
	Use:     "view ISSUE-KEY",
	Short:   "View issue details",
	Example: "  jira8 issue view ESA-123\n  jira8 issue view ESA-123 --with-history",
	Args:    cobra.ExactArgs(1),
	RunE:    runView,
}

func init() {
	viewCmd.Flags().Bool("markdown", false, "Convert description and comment bodies from Jira Wiki Markup to Markdown")
	viewCmd.Flags().Bool("with-history", false, "Include the issue history: who changed which field, when, from and to (long values trimmed except with -o json)")
}

func runView(cmd *cobra.Command, args []string) error {
	a := app.Get()
	get := a.Client.GetIssue
	if wh, _ := cmd.Flags().GetBool("with-history"); wh {
		get = a.Client.GetIssueWithChangelog
	}
	issue, err := get(context.Background(), args[0])
	if err != nil {
		return err
	}

	if md, _ := cmd.Flags().GetBool("markdown"); md {
		app.RenderIssueAsMarkdown(issue)
	}

	if a.Output == "json" {
		return app.OutputJSON(issue)
	}

	// Resolve Epic custom field IDs best-effort so view can render Epic Name /
	// Epic Link. A failure here is non-fatal — we still render the rest.
	epicNameID, epicLinkID, _ := a.EpicFieldIDs(context.Background())
	printIssueDetailWithEpic(issue, epicNameID, epicLinkID)
	app.TrimHistory(issue, app.HistoryValueMax)
	printHistory(issue)
	return nil
}
