package project

import (
	"context"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/spf13/cobra"
)

var typesCmd = &cobra.Command{
	Use:     "types",
	Short:   "List issue types available for creation",
	Example: "  jira8 project types --project ABC",
	RunE:    runTypes,
}

func init() {
	typesCmd.Flags().String("project", "", "Project key (default from config)")
}

func runTypes(cmd *cobra.Command, args []string) error {
	a := app.Get()

	project, err := app.ProjectKey(cmd)
	if err != nil {
		return err
	}

	meta, err := a.Client.GetCreateMeta(context.Background(), project)
	if err != nil {
		return err
	}

	if a.Output == "json" {
		return app.OutputJSON(meta.IssueTypes)
	}

	fmt.Printf("Issue types for %s:\n\n", project)
	for _, t := range meta.IssueTypes {
		kind := ""
		if t.Subtask {
			kind = "  sub-task: needs --parent"
		}
		fmt.Printf("  %-20s (id: %s)%s\n", t.Name, t.ID, kind)
	}
	return nil
}
