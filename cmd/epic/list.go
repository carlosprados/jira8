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
	Use:   "list",
	Short: "List Epics in a project",
	Long: `Lists the Epics of a project. --progress adds, per Epic, how many children
(issues linked to it, not their sub-tasks) are to do, in progress and done by
status category, and the sum of their estimates, remaining (children not done)
and time spent, in seconds. It costs one extra search per 100 Epics.`,
	Example: "  jira8 epic list\n  jira8 epic list --project ABC --status \"In Progress\"\n  jira8 epic list --project ABC --progress --max 0 -o json",
	RunE:    runList,
}

func init() {
	listCmd.Flags().String("project", "", "Project key (default from config)")
	listCmd.Flags().String("status", "", "Filter by status")
	listCmd.Flags().Int("max", 50, "Maximum number of results (0 = all)")
	listCmd.Flags().Bool("progress", false, "Count each Epic's children by status category and sum their estimates")
	listCmd.Flags().Bool("markdown", false, "Convert description fields from Jira Wiki Markup to Markdown")
}

func runList(cmd *cobra.Command, args []string) error {
	a := app.Get()

	project, err := app.ProjectKey(cmd)
	if err != nil {
		return err
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

	issues, _, err := a.Client.SearchAllIssues(context.Background(), jql, max, extra...)
	if err != nil {
		return err
	}

	if md, _ := cmd.Flags().GetBool("markdown"); md {
		app.RenderIssuesAsMarkdown(issues)
	}

	if progress, _ := cmd.Flags().GetBool("progress"); progress {
		return listWithProgress(issues, epicNameID)
	}

	if a.Output == "json" {
		return app.OutputJSON(issues)
	}

	printEpicTable(issues, epicNameID)
	return nil
}

// epicWithProgress is an Epic of `epic list --progress`: the issue plus progress.
type epicWithProgress struct {
	models.Issue
	Progress *client.EpicProgress `json:"progress"`
}

func listWithProgress(issues []models.Issue, epicNameID string) error {
	a := app.Get()
	ctx := context.Background()
	_, epicLinkID, err := a.EpicFieldIDs(ctx)
	if err != nil {
		return err
	}
	keys := make([]string, len(issues))
	for i, is := range issues {
		keys[i] = is.Key
	}
	progress, err := a.Client.EpicsProgress(ctx, epicLinkID, keys)
	if err != nil {
		return err
	}

	if a.Output == "json" {
		out := make([]epicWithProgress, len(issues))
		for i, is := range issues {
			out[i] = epicWithProgress{Issue: is, Progress: progress[is.Key]}
		}
		return app.OutputJSON(out)
	}

	if len(issues) == 0 {
		fmt.Println("No epics found.")
		return nil
	}
	fmt.Printf("%-12s  %-25s  %5s  %5s  %5s  %5s  %8s  %s\n", "KEY", "EPIC NAME", "TODO", "DOING", "DONE", "DONE%", "REMAIN", "SUMMARY")
	for _, is := range issues {
		p := progress[is.Key]
		pct := "-"
		if p.Children > 0 {
			pct = fmt.Sprintf("%d%%", p.Done*100/p.Children)
		}
		remain := "-"
		if p.RemainingEstimate > 0 {
			remain = fmt.Sprintf("%.1fh", float64(p.RemainingEstimate)/3600)
		}
		name := ""
		if epicNameID != "" {
			name = is.Fields.CustomString(epicNameID)
		}
		fmt.Printf("%-12s  %-25s  %5d  %5d  %5d  %5s  %8s  %s\n", is.Key, truncateRune(name, 25),
			p.Todo, p.Doing, p.Done, pct, remain, is.Fields.Summary)
	}
	fmt.Printf("\n%d epic(s)\n", len(issues))
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
