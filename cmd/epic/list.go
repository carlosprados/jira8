package epic

import (
	"context"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/client"
	"github.com/amplia/jira8/internal/models"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Short:   "List Epics in a project",
	Example: "  jira8 epic list\n  jira8 epic list --project ESA --status \"In Progress\"",
	RunE:    runList,
}

func init() {
	listCmd.Flags().String("project", "", "Project key (default from config)")
	listCmd.Flags().String("status", "", "Filter by status")
	listCmd.Flags().Int("max", 50, "Maximum number of results")
	listCmd.Flags().Bool("markdown", false, "Convert description fields from Jira Wiki Markup to Markdown")
}

func runList(cmd *cobra.Command, args []string) error {
	a := app.Get()

	project, _ := cmd.Flags().GetString("project")
	if project == "" {
		project = a.Config.Project
	}
	status, _ := cmd.Flags().GetString("status")
	max, _ := cmd.Flags().GetInt("max")

	jql := client.BuildJQLWith(client.JQLFilters{
		Project: project,
		Status:  status,
		Type:    "Epic",
	})

	// Request the Epic Name field so it appears in the JSON output; the text
	// renderer will pick it up from Issue.Fields.Raw.
	epicNameID, _, _ := a.EpicFieldIDs(context.Background())
	var extra []string
	if epicNameID != "" {
		extra = append(extra, epicNameID)
	}

	issues, err := a.Client.SearchAllIssues(context.Background(), jql, max, extra...)
	if err != nil {
		return err
	}

	if md, _ := cmd.Flags().GetBool("markdown"); md {
		app.RenderIssuesAsMarkdown(issues)
	}

	if a.Output == "json" {
		return app.OutputJSON(issues)
	}

	printEpicTable(issues, epicNameID)
	return nil
}

// printEpicTable renders Epics with a dedicated Epic Name column.
func printEpicTable(issues []models.Issue, epicNameID string) {
	if len(issues) == 0 {
		fmt.Println("No epics found.")
		return
	}
	fmt.Printf("%-12s  %-25s  %-30s  %s\n", "KEY", "EPIC NAME", "STATUS", "SUMMARY")
	for _, i := range issues {
		name := ""
		if epicNameID != "" {
			name = i.Fields.CustomString(epicNameID)
		}
		status := "-"
		if i.Fields.Status != nil {
			status = i.Fields.Status.Name
		}
		fmt.Printf("%-12s  %-25s  %-30s  %s\n", i.Key, truncateRune(name, 25), truncateRune(status, 30), i.Fields.Summary)
	}
	fmt.Printf("\n%d epic(s)\n", len(issues))
}

func truncateRune(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}
