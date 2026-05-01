package issue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/client"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Short:   "List issues",
	Example: "  jira8 issue list --status \"In Progress\" --assignee me",
	RunE:    runList,
}

func init() {
	listCmd.Flags().String("project", "", "Project key (default from config)")
	listCmd.Flags().String("status", "", "Filter by status")
	listCmd.Flags().String("assignee", "", "Filter by assignee (use 'me' for current user)")
	listCmd.Flags().String("type", "", "Filter by issue type (e.g. Epic, Story, Bug)")
	listCmd.Flags().String("epic", "", "Filter by parent Epic key (issues linked to this Epic)")
	listCmd.Flags().String("jql", "", "Raw JQL query (overrides other filters)")
	listCmd.Flags().Int("max", 50, "Maximum number of results")
	listCmd.Flags().Bool("markdown", false, "Convert description fields from Jira Wiki Markup to Markdown")
}

func runList(cmd *cobra.Command, args []string) error {
	jql, _ := cmd.Flags().GetString("jql")
	if jql == "" {
		project, _ := cmd.Flags().GetString("project")
		if project == "" {
			project = app.Get().Config.Project
		}
		status, _ := cmd.Flags().GetString("status")
		assignee, _ := cmd.Flags().GetString("assignee")
		issueType, _ := cmd.Flags().GetString("type")
		epic, _ := cmd.Flags().GetString("epic")
		jql = client.BuildJQLWith(client.JQLFilters{
			Project:  project,
			Status:   status,
			Assignee: assignee,
			Type:     issueType,
			Epic:     epic,
		})
	}

	max, _ := cmd.Flags().GetInt("max")
	issues, err := app.Get().Client.SearchAllIssues(context.Background(), jql, max)
	if err != nil {
		return err
	}

	if md, _ := cmd.Flags().GetBool("markdown"); md {
		app.RenderIssuesAsMarkdown(issues)
	}

	if app.Get().Output == "json" {
		data, err := json.MarshalIndent(issues, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	printIssueTable(issues)
	return nil
}
