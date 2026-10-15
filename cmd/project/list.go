package project

import (
	"context"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List every project you can browse, with its lead",
	Long: `Lists all the projects visible to you, active or idle, with their lead
(username, display name and email), category and, where Jira supports
archiving (Data Center), archived ones flagged.`,
	Example: "  jira8 project list\n  jira8 project list -o json | jq -r '.[] | [.key, .leadEmail] | @tsv'",
	Args:    cobra.NoArgs,
	RunE:    runList,
}

func runList(cmd *cobra.Command, _ []string) error {
	a := app.Get()
	projects, err := a.Client.ListProjects(context.Background())
	if err != nil {
		return err
	}
	if a.Output == "json" {
		return app.OutputJSON(projects)
	}
	fmt.Printf("%-12s  %-35s  %-25s  %s\n", "KEY", "NAME", "LEAD", "CATEGORY")
	for _, p := range projects {
		name := p.Name
		if p.Archived != nil && *p.Archived {
			name += " (archived)"
		}
		fmt.Printf("%-12s  %-35s  %-25s  %s\n", p.Key, truncate(name, 35), truncate(p.LeadName, 25), p.Category)
	}
	fmt.Printf("\n%d project(s)\n", len(projects))
	return nil
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}
