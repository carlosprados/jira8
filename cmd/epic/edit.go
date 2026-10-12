package epic

import (
	"context"
	"fmt"

	"github.com/amplia/jira8/cmd/app"
	"github.com/amplia/jira8/internal/models"
	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit EPIC-KEY",
	Short: "Edit an Epic",
	Example: "  jira8 epic edit ABC-42 --name \"Renamed epic\" --summary \"New summary\"\n" +
		"  jira8 epic edit ABC-42 --add-label q3 --field \"Fix Version/s=2.5\"",
	Args: cobra.ExactArgs(1),
	RunE: runEdit,
}

func init() {
	editCmd.Flags().String("name", "", "New Epic Name")
	editCmd.Flags().String("summary", "", "New summary")
	editCmd.Flags().String("description", "", "New description")
	editCmd.Flags().String("description-file", "", "Read new description from file (use - for stdin)")
	editCmd.Flags().String("assignee", "", "New assignee (use 'me' for current user, empty to unassign)")
	editCmd.Flags().String("priority", "", "New priority")
	app.AddFieldFlags(editCmd)
	app.AddLabelEditFlags(editCmd)
	editCmd.Flags().Bool("markdown", false, "Treat --description as Markdown and convert it to Jira Wiki Markup; local images (![alt](path)) are attached to the issue")
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
	body, err := app.ReadBody(cmd, "description", "description-file")
	if err != nil {
		return err
	}
	if body.Set {
		fields["description"] = body.Text
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

	update, err := app.EditFields(cmd, key, fields)
	if err != nil {
		return err
	}
	if len(fields) == 0 && len(update) == 0 {
		return fmt.Errorf("no fields to update; use --name, --summary, --description, --assignee, --priority, --labels, --add-label, --remove-label or --field")
	}

	uploaded, err := app.UploadImages(context.Background(), key, body.Images)
	if err != nil {
		return err
	}
	req := &models.EditIssueRequest{Fields: fields, Update: update}
	if err := a.Client.EditIssue(context.Background(), key, req); err != nil {
		return err
	}
	out := map[string]any{"key": key, "url": a.Client.BrowseURL(key), "updated": app.UpdatedFields(fields, update)}
	if len(uploaded) > 0 {
		out["attachments"] = uploaded
	}
	if err := app.Done(out, "Updated Epic %s", key); err != nil {
		return err
	}
	if a.Output != "json" {
		app.PrintUploadedImages(uploaded)
	}
	return nil
}
