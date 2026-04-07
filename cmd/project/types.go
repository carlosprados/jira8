package project

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/spf13/cobra"
)

var typesCmd = &cobra.Command{
	Use:     "types",
	Short:   "List issue types available for creation",
	Example: "  jira8 project types --project ESA",
	RunE:    runTypes,
}

func init() {
	typesCmd.Flags().String("project", "", "Project key (default from config)")
}

func runTypes(cmd *cobra.Command, args []string) error {
	a := app.Get()

	project, _ := cmd.Flags().GetString("project")
	if project == "" {
		project = a.Config.Project
	}

	meta, err := a.Client.GetCreateMeta(context.Background(), project)
	if err != nil {
		return err
	}

	if a.Output == "json" {
		data, err := json.MarshalIndent(meta.IssueTypes, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Issue types for %s:\n\n", project)
	for _, t := range meta.IssueTypes {
		fmt.Printf("  %-20s (id: %s)\n", t.Name, t.ID)
	}
	return nil
}
