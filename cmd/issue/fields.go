package issue

import (
	"context"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/spf13/cobra"
)

var fieldsCmd = &cobra.Command{
	Use:   "fields ISSUE-KEY",
	Short: "List the fields you can edit on an issue, with allowed values",
	Example: `  jira8 issue fields ABC-123
  jira8 issue fields ABC-123 -o json | jq '.[] | {id, name, type}'`,
	Args: cobra.ExactArgs(1),
	RunE: runFields,
}

func runFields(cmd *cobra.Command, args []string) error {
	screen, err := app.Get().Client.GetEditFields(context.Background(), args[0])
	if err != nil {
		return err
	}
	rows := app.FieldRows(screen)
	if app.Get().Output == "json" {
		return app.OutputJSON(rows)
	}
	fmt.Printf("Editable fields of %s:\n\n", args[0])
	app.PrintFieldRows(rows)
	return nil
}
