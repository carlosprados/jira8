package issue

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/client"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List issues",
	Long: `List issues matching the filters, combined with AND. --jql replaces all of them
(but not --sort, --max, --all or --fields). When more issues match than are
shown, a note on stderr says how many.`,
	Example: `  jira8 issue list --status "In Progress" --assignee me
  jira8 issue list --label backend --label urgent --updated-after -7d
  jira8 issue list --text "timeout" --sort created --all -o json
  jira8 issue list --fields labels,components -o json`,
	RunE: runList,
}

// maxAllWarning is where --all starts warning about the size of the answer.
const maxAllWarning = 1000

var jqlDate = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}( \d{2}:\d{2})?|-\d+[mhdw])$`)

func init() {
	listCmd.Flags().String("project", "", "Project key (default from config)")
	listCmd.Flags().String("status", "", "Filter by status")
	listCmd.Flags().String("assignee", "", "Filter by assignee (use 'me' for current user)")
	listCmd.Flags().String("reporter", "", "Filter by reporter (use 'me' for current user)")
	listCmd.Flags().String("type", "", "Filter by issue type (e.g. Epic, Story, Bug)")
	listCmd.Flags().String("priority", "", "Filter by priority")
	listCmd.Flags().String("component", "", "Filter by component")
	listCmd.Flags().StringArray("label", nil, "Filter by label (repeat: issues with all of them)")
	listCmd.Flags().String("epic", "", "Filter by parent Epic key (issues linked to this Epic)")
	listCmd.Flags().String("text", "", "Full-text search in summary, description and comments")
	listCmd.Flags().String("created-after", "", "Created on or after: YYYY-MM-DD or relative (-7d, -2w)")
	listCmd.Flags().String("updated-after", "", "Updated on or after: YYYY-MM-DD or relative (-7d, -2w)")
	listCmd.Flags().String("jql", "", "Raw JQL query (overrides other filters)")
	listCmd.Flags().String("sort", "updated", "Sort by created, updated, priority, key or rank (newest/highest first)")
	listCmd.Flags().Bool("asc", false, "Ascending order (oldest first; board order with --sort rank)")
	listCmd.Flags().Int("max", 50, "Maximum number of results")
	listCmd.Flags().Bool("all", false, "Fetch every matching issue (ignores --max)")
	listCmd.Flags().StringSlice("fields", nil, "Extra field ids to fetch for -o json (e.g. labels,components,customfield_10011)")
	listCmd.Flags().Bool("markdown", false, "Convert description fields from Jira Wiki Markup to Markdown")
}

func runList(cmd *cobra.Command, args []string) error {
	str := func(name string) string { v, _ := cmd.Flags().GetString(name); return v }

	sort := strings.ToLower(str("sort"))
	if _, ok := client.SortFields[sort]; !ok {
		return fmt.Errorf("--sort %q: use one of created, updated, priority, key, rank", sort)
	}
	asc, _ := cmd.Flags().GetBool("asc")

	jql := str("jql")
	if jql != "" {
		if cmd.Flags().Changed("sort") || cmd.Flags().Changed("asc") {
			jql = withOrder(jql, client.SortFields[sort], asc)
		}
	} else {
		for _, f := range []string{"created-after", "updated-after"} {
			if v := str(f); v != "" && !jqlDate.MatchString(v) {
				return fmt.Errorf("--%s %q: use YYYY-MM-DD or a relative period like -7d, -2w, -12h", f, v)
			}
		}
		project := str("project")
		if project == "" {
			project = app.Get().Config.Project
		}
		labels, _ := cmd.Flags().GetStringArray("label")
		jql = client.BuildJQLWith(client.JQLFilters{
			Project:      project,
			Status:       str("status"),
			Assignee:     str("assignee"),
			Reporter:     str("reporter"),
			Type:         str("type"),
			Priority:     str("priority"),
			Component:    str("component"),
			Labels:       labels,
			Epic:         str("epic"),
			Text:         str("text"),
			CreatedAfter: str("created-after"),
			UpdatedAfter: str("updated-after"),
			Sort:         sort,
			Asc:          asc,
		})
	}

	max, _ := cmd.Flags().GetInt("max")
	if all, _ := cmd.Flags().GetBool("all"); all {
		max = 0
	}
	fields, _ := cmd.Flags().GetStringSlice("fields")
	issues, total, err := app.Get().Client.SearchAllIssues(context.Background(), jql, max, fields...)
	if err != nil {
		return err
	}
	switch {
	case len(issues) < total:
		fmt.Fprintf(os.Stderr, "Showing %d of %d issues: raise --max or pass --all.\n", len(issues), total)
	case max == 0 && total > maxAllWarning:
		fmt.Fprintf(os.Stderr, "Fetched all %d issues: narrow the filters if that was not intended.\n", total)
	}

	if md, _ := cmd.Flags().GetBool("markdown"); md {
		app.RenderIssuesAsMarkdown(issues)
	}

	if app.Get().Output == "json" {
		return app.OutputJSON(issues)
	}

	printIssueTable(issues)
	return nil
}

// withOrder replaces the ORDER BY of a raw JQL query, or appends one.
func withOrder(jql, field string, asc bool) string {
	dir := "DESC"
	if asc {
		dir = "ASC"
	}
	if i := strings.LastIndex(strings.ToUpper(jql), "ORDER BY"); i >= 0 {
		jql = strings.TrimSpace(jql[:i])
	}
	return strings.TrimSpace(strings.Join(slices.DeleteFunc([]string{jql, "ORDER BY " + field + " " + dir}, func(s string) bool { return s == "" }), " "))
}
