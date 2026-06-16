package epic

import (
	"context"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/markup"
	"github.com/amplia/jira8/internal/models"
	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:     "edit EPIC-KEY",
	Short:   "Edit an Epic",
	Example: "  jira8 epic edit ESA-42 --name \"Renamed epic\" --summary \"New summary\"",
	Args:    cobra.ExactArgs(1),
	RunE:    runEdit,
}

func init() {
	editCmd.Flags().String("name", "", "New Epic Name")
	editCmd.Flags().String("summary", "", "New summary")
	editCmd.Flags().String("description", "", "New description")
	editCmd.Flags().String("description-file", "", "Read new description from file (use - for stdin)")
	editCmd.Flags().String("assignee", "", "New assignee (use 'me' for current user, empty to unassign)")
	editCmd.Flags().String("priority", "", "New priority")
	editCmd.Flags().Bool("markdown", false, "Treat --description as Markdown and convert to Jira Wiki Markup before sending")
}

func runEdit(cmd *cobra.Command, args []string) error {
	a := app.Get()
	key := args[0]

	fields := make(map[string]any)

	if cmd.Flags().Changed("name") {
		epicNameID, _, err := a.EpicFieldIDs(context.Background())
		if err != nil {
			return err
		}
		v, _ := cmd.Flags().GetString("name")
		fields[epicNameID] = v
	}

	if cmd.Flags().Changed("summary") {
		v, _ := cmd.Flags().GetString("summary")
		fields["summary"] = v
	}
	if v, set, err := app.ReadTextInput(cmd, "description", "description-file"); err != nil {
		return err
	} else if set {
		if md, _ := cmd.Flags().GetBool("markdown"); md {
			v = markup.MarkdownToWiki(v)
		}
		fields["description"] = v
	}
	if cmd.Flags().Changed("assignee") {
		v, _ := cmd.Flags().GetString("assignee")
		if v == "" {
			fields["assignee"] = nil
		} else {
			username, err := a.Client.ResolveAssignee(context.Background(), v)
			if err != nil {
				return err
			}
			fields["assignee"] = models.UserRef{Name: username}
		}
	}
	if cmd.Flags().Changed("priority") {
		v, _ := cmd.Flags().GetString("priority")
		fields["priority"] = models.PriorityRef{Name: v}
	}

	if len(fields) == 0 {
		return fmt.Errorf("no fields to update; use --name, --summary, --description, --assignee or --priority")
	}

	req := &models.EditIssueRequest{Fields: fields}
	if err := a.Client.EditIssue(context.Background(), key, req); err != nil {
		return err
	}
	fmt.Printf("Updated Epic %s\n", key)
	return nil
}
