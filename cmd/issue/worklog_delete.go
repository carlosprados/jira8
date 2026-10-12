package issue

import (
	"context"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/spf13/cobra"
)

var worklogDeleteCmd = &cobra.Command{
	Use:     "worklog-delete ISSUE-KEY",
	Aliases: []string{"wld"},
	Short:   "Delete a worklog entry from an issue",
	Example: `  jira8 issue worklog-delete ABC-123 --id 27705
  jira8 issue worklog-delete ABC-123 --id 27705 --yes`,
	Args: cobra.ExactArgs(1),
	RunE: runWorklogDelete,
}

func init() {
	worklogDeleteCmd.Flags().String("id", "", "Worklog ID (required)")
	worklogDeleteCmd.Flags().Bool("yes", false, "Skip confirmation prompt")
	_ = worklogDeleteCmd.MarkFlagRequired("id")
}

func runWorklogDelete(cmd *cobra.Command, args []string) error {
	a := app.Get()
	key := args[0]

	worklogID, _ := cmd.Flags().GetString("id")
	yes, _ := cmd.Flags().GetBool("yes")

	if ok, err := app.Confirm(fmt.Sprintf("Delete worklog %s from %s?", worklogID, key), yes); !ok {
		return err
	}

	if err := a.Client.DeleteWorklog(context.Background(), key, worklogID); err != nil {
		return err
	}

	return app.Done(map[string]any{"key": key, "id": worklogID, "deleted": true},
		"Worklog %s deleted from %s", worklogID, key)
}
