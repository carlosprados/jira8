package epic

import (
	"context"
	"fmt"
	"strings"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/client"
	"github.com/amplia/jira8/internal/models"
	"github.com/spf13/cobra"
)

var viewCmd = &cobra.Command{
	Use:     "view EPIC-KEY",
	Short:   "View Epic details and its linked children",
	Example: "  jira8 epic view ESA-42",
	Args:    cobra.ExactArgs(1),
	RunE:    runView,
}

func init() {
	viewCmd.Flags().Bool("no-children", false, "Skip fetching child issues")
	viewCmd.Flags().Int("max-children", 100, "Maximum children to fetch")
	viewCmd.Flags().Bool("markdown", false, "Convert description fields from Jira Wiki Markup to Markdown")
}

func runView(cmd *cobra.Command, args []string) error {
	a := app.Get()
	key := args[0]

	issue, err := a.Client.GetIssue(context.Background(), key)
	if err != nil {
		return err
	}

	epicNameID, epicLinkID, _ := a.EpicFieldIDs(context.Background())

	skipChildren, _ := cmd.Flags().GetBool("no-children")
	maxChildren, _ := cmd.Flags().GetInt("max-children")

	var children []models.Issue
	if !skipChildren {
		jql := client.BuildJQLWith(client.JQLFilters{Epic: key})
		children, err = a.Client.SearchAllIssues(context.Background(), jql, maxChildren)
		if err != nil {
			return fmt.Errorf("fetching children: %w", err)
		}
	}

	if md, _ := cmd.Flags().GetBool("markdown"); md {
		app.RenderIssueAsMarkdown(issue)
		app.RenderIssuesAsMarkdown(children)
	}

	if a.Output == "json" {
		out := struct {
			Epic     *models.Issue  `json:"epic"`
			Children []models.Issue `json:"children,omitempty"`
		}{Epic: issue, Children: children}
		return app.OutputJSON(out)
	}

	printEpicDetail(issue, epicNameID, epicLinkID)
	if !skipChildren {
		fmt.Println()
		fmt.Printf("Children (%d):\n", len(children))
		for _, c := range children {
			status := "-"
			if c.Fields.Status != nil {
				status = c.Fields.Status.Name
			}
			issueType := "-"
			if c.Fields.IssueType != nil {
				issueType = c.Fields.IssueType.Name
			}
			fmt.Printf("  %-12s  %-10s  %-15s  %s\n", c.Key, issueType, status, c.Fields.Summary)
		}
	}
	return nil
}

func printEpicDetail(issue *models.Issue, epicNameID, _ string) {
	f := issue.Fields
	fmt.Println()
	fmt.Printf("%s  %s\n", issue.Key, f.Summary)
	fmt.Println(strings.Repeat("─", 60))
	if epicNameID != "" {
		if name := f.CustomString(epicNameID); name != "" {
			fmt.Printf("  Epic Name:  %s\n", name)
		}
	}
	if f.Status != nil {
		fmt.Printf("  Status:     %s\n", f.Status.Name)
	}
	if f.Priority != nil {
		fmt.Printf("  Priority:   %s\n", f.Priority.Name)
	}
	if f.Assignee != nil {
		name := f.Assignee.DisplayName
		if name == "" {
			name = f.Assignee.Name
		}
		fmt.Printf("  Assignee:   %s\n", name)
	}
	if f.Description != "" {
		fmt.Println()
		fmt.Println("Description:")
		fmt.Println(f.Description)
	}
}
