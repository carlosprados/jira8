package project

import (
	"context"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/spf13/cobra"
)

var fieldsCmd = &cobra.Command{
	Use:   "fields",
	Short: "List the fields on the create screen of an issue type, with allowed values",
	Example: `  jira8 project fields --project ABC --type Bug
  jira8 project fields --type Story -o json | jq '.[] | select(.required)'`,
	RunE: runFields,
}

func init() {
	fieldsCmd.Flags().String("project", "", "Project key (default from config)")
	fieldsCmd.Flags().String("type", "Task", "Issue type")
}

func runFields(cmd *cobra.Command, args []string) error {
	project, err := app.ProjectKey(cmd)
	if err != nil {
		return err
	}
	issueType, _ := cmd.Flags().GetString("type")
	screen, err := app.Get().Client.GetCreateFields(context.Background(), project, issueType)
	if err != nil {
		return err
	}
	rows := app.FieldRows(screen)
	if app.Get().Output == "json" {
		return app.OutputJSON(rows)
	}
	fmt.Printf("Fields to create a %s in %s:\n\n", issueType, project)
	app.PrintFieldRows(rows)
	return nil
}
