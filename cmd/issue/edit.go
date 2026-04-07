package issue

import (
	"context"
	"fmt"
	"strings"

	"github.com/amplia/jira-cli/cmd/app"
	"github.com/amplia/jira-cli/internal/models"
	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit ISSUE-KEY",
	Short: "Edit an issue",
	Args:  cobra.ExactArgs(1),
	RunE:  runEdit,
}

func init() {
	editCmd.Flags().String("summary", "", "New summary")
	editCmd.Flags().String("description", "", "New description")
	editCmd.Flags().String("assignee", "", "New assignee (use 'me' for current user, empty to unassign)")
	editCmd.Flags().String("priority", "", "New priority")
}

func runEdit(cmd *cobra.Command, args []string) error {
	a := app.Get()
	key := args[0]

	fields := make(map[string]any)

	if cmd.Flags().Changed("summary") {
		v, _ := cmd.Flags().GetString("summary")
		fields["summary"] = v
	}

	if cmd.Flags().Changed("description") {
		v, _ := cmd.Flags().GetString("description")
		fields["description"] = v
	}

	if cmd.Flags().Changed("assignee") {
		v, _ := cmd.Flags().GetString("assignee")
		if v == "" {
			fields["assignee"] = nil
		} else {
			username := v
			if strings.EqualFold(v, "me") {
				user, err := a.Client.GetMyself(context.Background())
				if err != nil {
					return fmt.Errorf("resolving current user: %w", err)
				}
				username = user.Name
			}
			fields["assignee"] = models.UserRef{Name: username}
		}
	}

	if cmd.Flags().Changed("priority") {
		v, _ := cmd.Flags().GetString("priority")
		fields["priority"] = models.PriorityRef{Name: v}
	}

	if len(fields) == 0 {
		return fmt.Errorf("no fields to update; use --summary, --description, --assignee, or --priority")
	}

	req := &models.EditIssueRequest{Fields: fields}
	if err := a.Client.EditIssue(context.Background(), key, req); err != nil {
		return err
	}

	fmt.Printf("Updated %s\n", key)
	return nil
}
