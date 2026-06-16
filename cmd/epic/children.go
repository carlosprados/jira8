package epic

import (
	"context"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/client"
	"github.com/spf13/cobra"
)

var childrenCmd = &cobra.Command{
	Use:     "children EPIC-KEY",
	Short:   "List issues linked to an Epic",
	Example: "  jira8 epic children ESA-42",
	Args:    cobra.ExactArgs(1),
	RunE:    runChildren,
}

func init() {
	childrenCmd.Flags().Int("max", 100, "Maximum number of results")
	childrenCmd.Flags().Bool("markdown", false, "Convert description fields from Jira Wiki Markup to Markdown")
}

func runChildren(cmd *cobra.Command, args []string) error {
	a := app.Get()
	key := args[0]
	max, _ := cmd.Flags().GetInt("max")

	jql := client.BuildJQLWith(client.JQLFilters{Epic: key})
	issues, err := a.Client.SearchAllIssues(context.Background(), jql, max)
	if err != nil {
		return err
	}

	if md, _ := cmd.Flags().GetBool("markdown"); md {
		app.RenderIssuesAsMarkdown(issues)
	}

	if a.Output == "json" {
		return app.OutputJSON(issues)
	}

	if len(issues) == 0 {
		fmt.Printf("No children linked to %s.\n", key)
		return nil
	}
	fmt.Printf("Children of %s (%d):\n", key, len(issues))
	for _, i := range issues {
		status := "-"
		if i.Fields.Status != nil {
			status = i.Fields.Status.Name
		}
		issueType := "-"
		if i.Fields.IssueType != nil {
			issueType = i.Fields.IssueType.Name
		}
		fmt.Printf("  %-12s  %-10s  %-15s  %s\n", i.Key, issueType, status, i.Fields.Summary)
	}
	return nil
}
